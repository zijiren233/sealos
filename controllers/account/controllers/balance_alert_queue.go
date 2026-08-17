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
	"time"

	"github.com/google/uuid"
	"github.com/labring/sealos/controllers/pkg/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	balanceAlertQueuePollInterval = 5 * time.Second
	balanceAlertQueueLease        = 2 * time.Minute
	balanceAlertQueueBatchSize    = 500
	balanceAlertQueueConcurrency  = 100
	balanceAlertQueueRetryDelay   = time.Minute
)

func (r *DebtReconciler) EnqueueBalanceAlertUsers(users []uuid.UUID, reason string) error {
	if err := enqueueBalanceAlertUsers(
		r.AccountV2.GetGlobalDB(), users, reason, time.Now().UTC(),
	); err != nil {
		return fmt.Errorf("enqueue balance alert users: %w", err)
	}
	return nil
}

func (r *DebtReconciler) processBalanceAlertQueueLoop(ctx context.Context) {
	run := func() {
		if err := r.processBalanceAlertQueue(ctx); err != nil {
			r.Error(err, "failed to process balance alert queue")
		}
	}
	run()
	ticker := time.NewTicker(balanceAlertQueuePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func (r *DebtReconciler) processBalanceAlertQueue(ctx context.Context) error {
	items, err := r.claimBalanceAlertQueue(ctx, balanceAlertQueueBatchSize)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	workers := make(chan struct{}, balanceAlertQueueConcurrency)
	errCh := make(chan error, len(items))
	var wg sync.WaitGroup
	for i := range items {
		item := items[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			workers <- struct{}{}
			defer func() { <-workers }()
			refreshErr := r.RefreshDebtStatus(item.UserUID)
			if err := r.finishBalanceAlertQueueItem(ctx, item, refreshErr); err != nil {
				errCh <- err
				return
			}
			if refreshErr != nil {
				r.Error(
					refreshErr,
					"balance alert user will be retried",
					"userUID", item.UserUID,
					"reason", item.Reason,
				)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for processErr := range errCh {
		if processErr != nil {
			return processErr
		}
	}
	return nil
}

func (r *DebtReconciler) claimBalanceAlertQueue(
	ctx context.Context,
	limit int,
) ([]types.BalanceAlertQueue, error) {
	now := time.Now().UTC()
	leaseUntil := now.Add(balanceAlertQueueLease)
	leaseToken := uuid.NewString()
	var items []types.BalanceAlertQueue
	err := r.AccountV2.GlobalTransactionHandler(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("available_at <= ? AND (lease_until IS NULL OR lease_until <= ?)", now, now).
			Order("available_at ASC, updated_at ASC").
			Limit(limit).
			Find(&items).Error; err != nil {
			return fmt.Errorf("claim balance alert queue rows: %w", err)
		}
		if len(items) == 0 {
			return nil
		}
		users := make([]uuid.UUID, len(items))
		for i := range items {
			users[i] = items[i].UserUID
		}
		if err := tx.Model(&types.BalanceAlertQueue{}).
			Where("user_uid IN ?", users).
			Updates(map[string]any{
				"lease_owner": r.processID,
				"lease_token": leaseToken,
				"lease_until": leaseUntil,
				"updated_at":  now,
			}).Error; err != nil {
			return fmt.Errorf("lease balance alert queue rows: %w", err)
		}
		for i := range items {
			items[i].LeaseOwner = r.processID
			items[i].LeaseToken = leaseToken
			items[i].LeaseUntil = &leaseUntil
		}
		return nil
	})
	return items, err
}

func (r *DebtReconciler) finishBalanceAlertQueueItem(
	ctx context.Context,
	item types.BalanceAlertQueue,
	refreshErr error,
) error {
	db := r.AccountV2.GetGlobalDB().WithContext(ctx)
	if refreshErr == nil {
		result := db.Where(
			"user_uid = ? AND lease_owner = ? AND lease_token = ?",
			item.UserUID,
			r.processID,
			item.LeaseToken,
		).Delete(&types.BalanceAlertQueue{})
		if result.Error != nil {
			return fmt.Errorf("complete balance alert queue item %s: %w", item.UserUID, result.Error)
		}
		if result.RowsAffected == 0 {
			if err := db.Model(&types.BalanceAlertQueue{}).
				Where("user_uid = ? AND lease_owner = ?", item.UserUID, r.processID).
				Updates(map[string]any{
					"lease_owner": "",
					"lease_token": "",
					"lease_until": nil,
					"updated_at":  time.Now().UTC(),
				}).Error; err != nil {
				return fmt.Errorf("release updated balance alert queue item %s: %w", item.UserUID, err)
			}
		}
		return nil
	}
	result := db.Model(&types.BalanceAlertQueue{}).
		Where(
			"user_uid = ? AND lease_owner = ? AND lease_token = ?",
			item.UserUID,
			r.processID,
			item.LeaseToken,
		).
		Updates(map[string]any{
			"available_at": time.Now().UTC().Add(balanceAlertQueueRetryDelay),
			"lease_owner":  "",
			"lease_token":  "",
			"lease_until":  nil,
			"updated_at":   time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("retry balance alert queue item %s: %w", item.UserUID, result.Error)
	}
	if result.RowsAffected == 0 {
		if err := db.Model(&types.BalanceAlertQueue{}).
			Where("user_uid = ? AND lease_owner = ?", item.UserUID, r.processID).
			Updates(map[string]any{
				"lease_owner": "",
				"lease_until": nil,
				"updated_at":  time.Now().UTC(),
			}).Error; err != nil {
			return fmt.Errorf("release retriggered balance alert queue item %s: %w", item.UserUID, err)
		}
	}
	return nil
}

func (r *DebtReconciler) enqueueBalanceAlertCompensation() error {
	now := time.Now().UTC()
	var users []uuid.UUID
	if err := r.AccountV2.GetGlobalDB().Raw(`
		SELECT DISTINCT user_uid
		FROM (
			SELECT user_uid
			FROM "BalanceUsageHour"
			WHERE usage_hour > ?
			UNION
			SELECT user_uid
			FROM "Credits"
			WHERE status = ? AND expire_at > ? AND expire_at <= ?
			UNION
			SELECT user_uid
			FROM "BalanceAlertEpisode"
			WHERE active_key IS NOT NULL
		) AS candidates`,
		now.Add(-balanceForecastLongWindowHours*time.Hour),
		types.CreditsStatusActive,
		now.Add(-2*time.Hour),
		now.Add(time.Hour),
	).Scan(&users).Error; err != nil {
		return fmt.Errorf("query balance alert compensation users: %w", err)
	}
	return r.EnqueueBalanceAlertUsers(users, "hourly-compensation")
}

func (r *DebtReconciler) enqueueBalanceAlertCompensationLoop(ctx context.Context) {
	run := func() {
		if err := r.enqueueBalanceAlertCompensation(); err != nil {
			r.Error(err, "failed to enqueue balance alert compensation")
		}
	}
	run()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
