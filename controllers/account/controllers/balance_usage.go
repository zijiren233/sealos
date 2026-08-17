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
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labring/sealos/controllers/pkg/database"
	"github.com/labring/sealos/controllers/pkg/resources"
	"github.com/labring/sealos/controllers/pkg/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func balanceUsageHour(at time.Time) time.Time {
	at = at.UTC()
	hour := at.Truncate(time.Hour)
	if at.Equal(hour) {
		return hour
	}
	return hour.Add(time.Hour)
}

func balanceUsageEventID(regionUID uuid.UUID, sourceID string) string {
	return regionUID.String() + "/" + sourceID
}

func persistBalanceUsageBillings(
	account database.AccountV2,
	userUID uuid.UUID,
	billings []*resources.Billing,
) error {
	if userUID == uuid.Nil {
		return errors.New("balance usage user UID is empty")
	}
	regionUID := account.GetLocalRegion().UID
	if regionUID == uuid.Nil {
		return errors.New("balance usage region UID is empty")
	}
	return account.GlobalTransactionHandler(func(tx *gorm.DB) error {
		for i := range billings {
			billing := billings[i]
			if billing == nil || billing.Amount <= 0 || billing.Status == resources.Subscription {
				continue
			}
			if billing.OrderID == "" {
				return errors.New("balance usage billing order ID is empty")
			}
			event := types.BalanceUsageEvent{
				ID:        balanceUsageEventID(regionUID, billing.OrderID),
				SourceID:  billing.OrderID,
				UserUID:   userUID,
				RegionUID: regionUID,
				UsageHour: balanceUsageHour(billing.Time),
				Amount:    billing.Amount,
				Workspace: billing.Namespace,
				AppType:   billing.AppType,
				AppName:   billing.AppName,
			}
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&event)
			if result.Error != nil {
				return fmt.Errorf("create balance usage event %s: %w", event.ID, result.Error)
			}
			if result.RowsAffected == 0 {
				continue
			}
			hour := types.BalanceUsageHour{
				UserUID: userUID, UsageHour: event.UsageHour, Amount: event.Amount,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "user_uid"}, {Name: "usage_hour"}},
				DoUpdates: clause.Assignments(map[string]any{
					"amount": gorm.Expr(
						`"BalanceUsageHour"."amount" + EXCLUDED."amount"`,
					),
					"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
				}),
			}).Create(&hour).Error; err != nil {
				return fmt.Errorf("upsert balance usage hour for event %s: %w", event.ID, err)
			}
		}
		return nil
	})
}

func enqueueBalanceAlertUsers(
	db *gorm.DB,
	users []uuid.UUID,
	reason string,
	availableAt time.Time,
) error {
	if len(users) == 0 {
		return nil
	}
	items := make([]types.BalanceAlertQueue, 0, len(users))
	seen := make(map[uuid.UUID]struct{}, len(users))
	for _, userUID := range users {
		if userUID == uuid.Nil {
			continue
		}
		if _, exists := seen[userUID]; exists {
			continue
		}
		seen[userUID] = struct{}{}
		items = append(items, types.BalanceAlertQueue{
			UserUID: userUID, Reason: reason, AvailableAt: availableAt,
		})
	}
	if len(items) == 0 {
		return nil
	}
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_uid"}},
		DoUpdates: clause.Assignments(map[string]any{
			"reason":       gorm.Expr("EXCLUDED.reason"),
			"available_at": gorm.Expr("EXCLUDED.available_at"),
			"lease_token":  gorm.Expr("EXCLUDED.lease_token"),
			"updated_at":   gorm.Expr("CURRENT_TIMESTAMP"),
		}),
	}).Create(&items).Error
}

func completeBalanceBillingHour(account database.AccountV2, dataThrough time.Time) error {
	dataThrough = dataThrough.UTC().Truncate(time.Hour)
	regionUID := account.GetLocalRegion().UID
	if regionUID == uuid.Nil {
		return errors.New("billing watermark region UID is empty")
	}
	return account.GlobalTransactionHandler(func(tx *gorm.DB) error {
		watermark := types.BalanceBillingWatermark{
			RegionUID: regionUID, DataThrough: dataThrough, ConsecutiveHours: 1,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "region_uid"}},
			DoUpdates: clause.Assignments(map[string]any{
				"consecutive_hours": gorm.Expr(`CASE
					WHEN EXCLUDED."data_through" <= "BalanceBillingWatermark"."data_through"
						THEN "BalanceBillingWatermark"."consecutive_hours"
					WHEN EXCLUDED."data_through" = "BalanceBillingWatermark"."data_through" + INTERVAL '1 hour'
						THEN LEAST("BalanceBillingWatermark"."consecutive_hours" + 1, ?)
					ELSE 1
				END`, balanceForecastLongWindowHours),
				"data_through": gorm.Expr(
					`GREATEST("BalanceBillingWatermark"."data_through", EXCLUDED."data_through")`,
				),
				"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
			}),
		}).Create(&watermark).Error; err != nil {
			return fmt.Errorf("upsert region billing watermark: %w", err)
		}

		var users []uuid.UUID
		if err := tx.Model(&types.BalanceUsageHour{}).
			Where("usage_hour = ?", dataThrough).
			Distinct("user_uid").
			Pluck("user_uid", &users).Error; err != nil {
			return fmt.Errorf("query billed users for completed hour: %w", err)
		}
		if err := enqueueBalanceAlertUsers(tx, users, "billing-hour", time.Now().UTC()); err != nil {
			return fmt.Errorf("enqueue billed users: %w", err)
		}
		return nil
	})
}
