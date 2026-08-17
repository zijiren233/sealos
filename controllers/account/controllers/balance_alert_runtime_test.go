// Copyright © 2026 sealos.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controllers

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labring/sealos/controllers/pkg/database"
	"github.com/labring/sealos/controllers/pkg/resources"
	"github.com/labring/sealos/controllers/pkg/types"
	"github.com/testcontainers/testcontainers-go"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type balanceAlertRuntimeAccount struct {
	database.AccountV2
	db     *gorm.DB
	region types.Region
}

func (a *balanceAlertRuntimeAccount) GetGlobalDB() *gorm.DB {
	return a.db
}

func (a *balanceAlertRuntimeAccount) GetLocalRegion() types.Region {
	return a.region
}

func (a *balanceAlertRuntimeAccount) GlobalTransactionHandler(
	funcs ...func(tx *gorm.DB) error,
) error {
	return a.db.Transaction(func(tx *gorm.DB) error {
		for _, fn := range funcs {
			if err := fn(tx); err != nil {
				return err
			}
		}
		return nil
	})
}

func TestBalanceAlertPersistenceWithPostgresRuntime(t *testing.T) {
	testcontainers.SkipIfProviderIsNotHealthy(t)
	ctx := context.Background()
	container, err := postgrescontainer.Run(ctx, "postgres:16-alpine",
		postgrescontainer.WithDatabase("account"),
		postgrescontainer.WithUsername("account"),
		postgrescontainer.WithPassword("account"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Errorf("terminate PostgreSQL: %v", err)
		}
	})
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&types.BalanceUsageEvent{},
		&types.BalanceUsageHour{},
		&types.BalanceBillingWatermark{},
		&types.BalanceAlertState{},
		&types.BalanceAlertEpisode{},
		&types.BalanceAlertEvent{},
		&types.BalanceAlertDelivery{},
		&types.BalanceAlertQueue{},
	); err != nil {
		t.Fatal(err)
	}

	t.Run("usage events are globally idempotent and atomic", func(t *testing.T) {
		testBalanceUsagePersistenceRuntime(t, db)
	})
	t.Run("billing watermark is monotonic and enqueues users", func(t *testing.T) {
		testBalanceBillingWatermarkRuntime(t, db)
	})
	t.Run("schema enforces alert uniqueness", func(t *testing.T) {
		testBalanceAlertUniqueConstraintsRuntime(t, db)
	})
	t.Run("queue preserves retriggers during a lease", func(t *testing.T) {
		testBalanceAlertQueueRetriggerRuntime(t, db)
	})
	t.Run("episode transaction debounces and escalates", func(t *testing.T) {
		testBalanceAlertEpisodeRuntime(t, db)
	})
}

func testBalanceUsagePersistenceRuntime(t *testing.T, db *gorm.DB) {
	t.Helper()
	userUID := uuid.New()
	regionA := types.Region{UID: uuid.New(), Domain: "region-a.example.com"}
	regionB := types.Region{UID: uuid.New(), Domain: "region-b.example.com"}
	accountA := &balanceAlertRuntimeAccount{db: db, region: regionA}
	accountB := &balanceAlertRuntimeAccount{db: db, region: regionB}
	usageTime := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Hour).Add(10 * time.Minute)
	billing := &resources.Billing{
		Time: usageTime, OrderID: "shared-order", Namespace: "workspace-a",
		AppName: "database", Amount: 100,
	}

	errCh := make(chan error, 8)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- persistBalanceUsageBillings(accountA, userUID, []*resources.Billing{billing})
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	regionBBilling := *billing
	regionBBilling.Amount = 250
	if err := persistBalanceUsageBillings(
		accountB,
		userUID,
		[]*resources.Billing{&regionBBilling},
	); err != nil {
		t.Fatal(err)
	}

	var eventCount int64
	if err := db.Model(&types.BalanceUsageEvent{}).
		Where("user_uid = ? AND source_id = ?", userUID, billing.OrderID).
		Count(&eventCount).Error; err != nil {
		t.Fatal(err)
	}
	if eventCount != 2 {
		t.Fatalf("usage event count = %d, want 2", eventCount)
	}
	var hour types.BalanceUsageHour
	if err := db.Where(
		"user_uid = ? AND usage_hour = ?",
		userUID,
		balanceUsageHour(usageTime),
	).First(&hour).Error; err != nil {
		t.Fatal(err)
	}
	if hour.Amount != 350 {
		t.Fatalf("aggregated amount = %d, want 350", hour.Amount)
	}

	rollbackUserUID := uuid.New()
	err := persistBalanceUsageBillings(accountA, rollbackUserUID, []*resources.Billing{
		{Time: usageTime, OrderID: "valid-order", Amount: 10},
		{Time: usageTime, Amount: 20},
	})
	if err == nil {
		t.Fatal("persisting an event without an order ID succeeded")
	}
	if err := db.Model(&types.BalanceUsageEvent{}).
		Where("user_uid = ?", rollbackUserUID).
		Count(&eventCount).Error; err != nil {
		t.Fatal(err)
	}
	if eventCount != 0 {
		t.Fatalf("rolled-back usage event count = %d, want 0", eventCount)
	}
}

func testBalanceBillingWatermarkRuntime(t *testing.T, db *gorm.DB) {
	t.Helper()
	userUID := uuid.New()
	region := types.Region{UID: uuid.New(), Domain: "watermark.example.com"}
	account := &balanceAlertRuntimeAccount{db: db, region: region}
	completedHour := time.Now().UTC().Add(-6 * time.Hour).Truncate(time.Hour)
	if err := db.Create(&types.BalanceUsageHour{
		UserUID: userUID, UsageHour: completedHour, Amount: 100,
	}).Error; err != nil {
		t.Fatal(err)
	}

	assertWatermark := func(wantHour time.Time, wantConsecutive int) {
		t.Helper()
		var watermark types.BalanceBillingWatermark
		if err := db.First(&watermark, "region_uid = ?", region.UID).Error; err != nil {
			t.Fatal(err)
		}
		if !watermark.DataThrough.Equal(wantHour) ||
			watermark.ConsecutiveHours != wantConsecutive {
			t.Fatalf(
				"watermark = (%s, %d), want (%s, %d)",
				watermark.DataThrough,
				watermark.ConsecutiveHours,
				wantHour,
				wantConsecutive,
			)
		}
	}

	if err := completeBalanceBillingHour(account, completedHour); err != nil {
		t.Fatal(err)
	}
	assertWatermark(completedHour, 1)
	if err := completeBalanceBillingHour(account, completedHour); err != nil {
		t.Fatal(err)
	}
	assertWatermark(completedHour, 1)

	nextHour := completedHour.Add(time.Hour)
	if err := completeBalanceBillingHour(account, nextHour); err != nil {
		t.Fatal(err)
	}
	assertWatermark(nextHour, 2)

	jumpedHour := nextHour.Add(2 * time.Hour)
	if err := completeBalanceBillingHour(account, jumpedHour); err != nil {
		t.Fatal(err)
	}
	assertWatermark(jumpedHour, 1)
	if err := completeBalanceBillingHour(account, completedHour); err != nil {
		t.Fatal(err)
	}
	assertWatermark(jumpedHour, 1)

	var queue types.BalanceAlertQueue
	if err := db.First(&queue, "user_uid = ?", userUID).Error; err != nil {
		t.Fatal(err)
	}
	if queue.Reason != "billing-hour" {
		t.Fatalf("queue reason = %q, want billing-hour", queue.Reason)
	}
}

func testBalanceAlertUniqueConstraintsRuntime(t *testing.T, db *gorm.DB) {
	t.Helper()
	userUID := uuid.New()
	activeKey := userUID
	now := time.Now().UTC()
	episode := types.BalanceAlertEpisode{
		ID: uuid.New(), UserUID: userUID, ActiveKey: &activeKey,
		HighestLevel: types.LowBalancePeriod, StartedAt: now,
	}
	if err := db.Create(&episode).Error; err != nil {
		t.Fatal(err)
	}
	duplicateEpisode := episode
	duplicateEpisode.ID = uuid.New()
	if err := db.Create(&duplicateEpisode).Error; err == nil {
		t.Fatal("creating a second active episode for one user succeeded")
	}

	event := types.BalanceAlertEvent{
		ID: uuid.New(), UserUID: userUID, EpisodeID: episode.ID,
		AlertLevel: types.LowBalancePeriod, Payload: `{}`,
	}
	if err := db.Create(&event).Error; err != nil {
		t.Fatal(err)
	}
	duplicateEvent := event
	duplicateEvent.ID = uuid.New()
	if err := db.Create(&duplicateEvent).Error; err == nil {
		t.Fatal("creating a duplicate episode alert level succeeded")
	}

	delivery := types.BalanceAlertDelivery{
		ID: uuid.New(), AlertEventID: event.ID, Channel: types.BalanceAlertChannelEmail,
		Recipient: "user@example.com", Payload: `{}`,
		Status: types.BalanceAlertDeliveryPending, AvailableAt: now,
	}
	if err := db.Create(&delivery).Error; err != nil {
		t.Fatal(err)
	}
	duplicateDelivery := delivery
	duplicateDelivery.ID = uuid.New()
	if err := db.Create(&duplicateDelivery).Error; err == nil {
		t.Fatal("creating a duplicate alert delivery succeeded")
	}
}

func testBalanceAlertQueueRetriggerRuntime(t *testing.T, db *gorm.DB) {
	t.Helper()
	userUID := uuid.New()
	now := time.Now().UTC().Add(-time.Second)
	if err := enqueueBalanceAlertUsers(db, []uuid.UUID{userUID}, "billing-hour", now); err != nil {
		t.Fatal(err)
	}
	reconciler := &DebtReconciler{
		AccountV2: &balanceAlertRuntimeAccount{db: db},
		processID: "worker-a",
	}
	items, err := reconciler.claimBalanceAlertQueue(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].LeaseToken == "" {
		t.Fatalf("claimed queue items = %#v", items)
	}

	if err := enqueueBalanceAlertUsers(db, []uuid.UUID{userUID}, "payment", now); err != nil {
		t.Fatal(err)
	}
	var retriggered types.BalanceAlertQueue
	if err := db.First(&retriggered, "user_uid = ?", userUID).Error; err != nil {
		t.Fatal(err)
	}
	if retriggered.Reason != "payment" || retriggered.LeaseOwner != reconciler.processID ||
		retriggered.LeaseToken != "" || retriggered.LeaseUntil == nil {
		t.Fatalf("retriggered queue row = %#v", retriggered)
	}

	if err := reconciler.finishBalanceAlertQueueItem(context.Background(), items[0], nil); err != nil {
		t.Fatal(err)
	}
	var released types.BalanceAlertQueue
	if err := db.First(&released, "user_uid = ?", userUID).Error; err != nil {
		t.Fatal(err)
	}
	if released.LeaseOwner != "" || released.LeaseToken != "" || released.LeaseUntil != nil {
		t.Fatalf("released queue row = %#v", released)
	}

	items, err = reconciler.claimBalanceAlertQueue(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("reclaimed queue item count = %d, want 1", len(items))
	}
	if err := reconciler.finishBalanceAlertQueueItem(context.Background(), items[0], nil); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&types.BalanceAlertQueue{}).
		Where("user_uid = ?", userUID).
		Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("completed queue item count = %d, want 0", count)
	}
}

func testBalanceAlertEpisodeRuntime(t *testing.T, db *gorm.DB) {
	t.Helper()
	userUID := uuid.New()
	reconciler := &DebtReconciler{}
	prediction := balancePrediction{Confidence: types.BalancePredictionConfidenceHigh}
	payload := balanceAlertPayload{UserUID: userUID, UserID: "user-a", UserName: "User A"}
	deliverySpecs := []balanceAlertDeliverySpec{
		{Channel: types.BalanceAlertChannelEmail, Recipient: "user@example.com"},
		{Channel: types.BalanceAlertChannelSMS, Recipient: "+8613800000000"},
	}
	account := &balanceAlertRuntimeAccount{db: db}
	now := time.Now().UTC()
	apply := func(assessment balanceRiskAssessment, at time.Time) error {
		return account.GlobalTransactionHandler(func(tx *gorm.DB) error {
			return reconciler.applyBalanceAlertEpisode(
				tx,
				userUID,
				assessment,
				prediction,
				payload,
				deliverySpecs,
				at,
			)
		})
	}

	low := balanceRiskAssessment{Status: types.LowBalancePeriod, Risk: true}
	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			errCh <- apply(low, now.Add(time.Duration(offset)*time.Second))
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	assertAlertCounts(t, db, userUID, 1, 1, 2)
	if err := apply(low, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	assertAlertCounts(t, db, userUID, 1, 1, 2)

	critical := balanceRiskAssessment{
		Status: types.CriticalBalancePeriod, Risk: true, Immediate: true,
	}
	if err := apply(critical, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	assertAlertCounts(t, db, userUID, 1, 2, 4)

	recovered := balanceRiskAssessment{Status: types.NormalPeriod, Recovered: true}
	if err := apply(recovered, now.Add(4*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := apply(recovered, now.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	var state types.BalanceAlertState
	if err := db.First(&state, "user_uid = ?", userUID).Error; err != nil {
		t.Fatal(err)
	}
	if state.ActiveEpisodeID != nil || state.RecoveryCount != 0 {
		t.Fatalf("recovered alert state = %#v", state)
	}
	var episode types.BalanceAlertEpisode
	if err := db.First(&episode, "user_uid = ?", userUID).Error; err != nil {
		t.Fatal(err)
	}
	if episode.ActiveKey != nil || episode.ClosedAt == nil ||
		episode.HighestLevel != types.CriticalBalancePeriod {
		t.Fatalf("closed alert episode = %#v", episode)
	}
}

func assertAlertCounts(
	t *testing.T,
	db *gorm.DB,
	userUID uuid.UUID,
	wantEpisodes, wantEvents, wantDeliveries int64,
) {
	t.Helper()
	models := []struct {
		name  string
		model any
		want  int64
	}{
		{name: "episodes", model: &types.BalanceAlertEpisode{}, want: wantEpisodes},
		{name: "events", model: &types.BalanceAlertEvent{}, want: wantEvents},
	}
	for _, item := range models {
		var count int64
		if err := db.Model(item.model).Where("user_uid = ?", userUID).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != item.want {
			t.Fatalf("%s count = %d, want %d", item.name, count, item.want)
		}
	}

	var deliveryCount int64
	err := db.Model(&types.BalanceAlertDelivery{}).
		Joins(`JOIN "BalanceAlertEvent" ON "BalanceAlertEvent".id = "BalanceAlertDelivery".alert_event_id`).
		Where(`"BalanceAlertEvent".user_uid = ?`, userUID).
		Count(&deliveryCount).Error
	if err != nil {
		t.Fatal(fmt.Errorf("count alert deliveries: %w", err))
	}
	if deliveryCount != wantDeliveries {
		t.Fatalf("deliveries count = %d, want %d", deliveryCount, wantDeliveries)
	}
}
