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

package types

import (
	"time"

	"github.com/google/uuid"
)

type BalancePredictionConfidence string

const (
	BalancePredictionConfidenceHigh BalancePredictionConfidence = "high"
	BalancePredictionConfidenceLow  BalancePredictionConfidence = "low"
)

type BalanceAlertChannel string

const (
	BalanceAlertChannelEmail BalanceAlertChannel = "email"
	BalanceAlertChannelSMS   BalanceAlertChannel = "sms"
	BalanceAlertChannelVoice BalanceAlertChannel = "voice"
)

type BalanceAlertDeliveryStatus string

const (
	BalanceAlertDeliveryPending    BalanceAlertDeliveryStatus = "pending"
	BalanceAlertDeliveryProcessing BalanceAlertDeliveryStatus = "processing"
	BalanceAlertDeliveryRetry      BalanceAlertDeliveryStatus = "retry"
	BalanceAlertDeliverySent       BalanceAlertDeliveryStatus = "sent"
)

// BalanceUsageEvent is the immutable, globally deduplicated billing fact.
type BalanceUsageEvent struct {
	ID        string    `gorm:"column:id;type:text;primary_key"`
	SourceID  string    `gorm:"column:source_id;type:text;not null"`
	UserUID   uuid.UUID `gorm:"column:user_uid;type:uuid;not null;index:idx_balance_usage_event_user_hour,priority:1"`
	RegionUID uuid.UUID `gorm:"column:region_uid;type:uuid;not null;index"`
	UsageHour time.Time `gorm:"column:usage_hour;type:timestamp(3) with time zone;not null;index:idx_balance_usage_event_user_hour,priority:2"`
	Amount    int64     `gorm:"column:amount;type:bigint;not null"`
	Workspace string    `gorm:"column:workspace;type:text"`
	AppType   uint8     `gorm:"column:app_type;type:smallint"`
	AppName   string    `gorm:"column:app_name;type:text"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime;default:current_timestamp"`
}

func (BalanceUsageEvent) TableName() string { return "BalanceUsageEvent" }

// BalanceUsageHour bounds a prediction read to one row per user and hour.
type BalanceUsageHour struct {
	UserUID   uuid.UUID `gorm:"column:user_uid;type:uuid;not null;primaryKey"`
	UsageHour time.Time `gorm:"column:usage_hour;type:timestamp(3) with time zone;not null;primaryKey"`
	Amount    int64     `gorm:"column:amount;type:bigint;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime;default:current_timestamp"`
}

func (BalanceUsageHour) TableName() string { return "BalanceUsageHour" }

// BalanceBillingWatermark records the last complete billing hour in a region.
type BalanceBillingWatermark struct {
	RegionUID        uuid.UUID `gorm:"column:region_uid;type:uuid;primaryKey"`
	DataThrough      time.Time `gorm:"column:data_through;type:timestamp(3) with time zone;not null"`
	ConsecutiveHours int       `gorm:"column:consecutive_hours;not null;default:1"`
	UpdatedAt        time.Time `gorm:"column:updated_at;autoUpdateTime;default:current_timestamp"`
}

func (BalanceBillingWatermark) TableName() string { return "BalanceBillingWatermark" }

// BalanceAlertState stores debounce counters between prediction runs.
type BalanceAlertState struct {
	UserUID         uuid.UUID                   `gorm:"column:user_uid;type:uuid;primaryKey"`
	ActiveEpisodeID *uuid.UUID                  `gorm:"column:active_episode_id;type:uuid"`
	RiskCount       int                         `gorm:"column:risk_count;not null;default:0"`
	RecoveryCount   int                         `gorm:"column:recovery_count;not null;default:0"`
	LastRiskLevel   DebtStatusType              `gorm:"column:last_risk_level;type:text"`
	LastConfidence  BalancePredictionConfidence `gorm:"column:last_confidence;type:text"`
	LastETASeconds  *int64                      `gorm:"column:last_eta_seconds;type:bigint"`
	LastEvaluatedAt time.Time                   `gorm:"column:last_evaluated_at;type:timestamp(3) with time zone;not null"`
	UpdatedAt       time.Time                   `gorm:"column:updated_at;autoUpdateTime;default:current_timestamp"`
}

func (BalanceAlertState) TableName() string { return "BalanceAlertState" }

// BalanceAlertEpisode represents one continuous balance-risk period. ActiveKey
// is cleared on close, allowing the unique index to enforce one active episode.
type BalanceAlertEpisode struct {
	ID           uuid.UUID      `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	UserUID      uuid.UUID      `gorm:"column:user_uid;type:uuid;not null;index"`
	ActiveKey    *uuid.UUID     `gorm:"column:active_key;type:uuid;uniqueIndex"`
	HighestLevel DebtStatusType `gorm:"column:highest_level;type:text;not null"`
	StartedAt    time.Time      `gorm:"column:started_at;type:timestamp(3) with time zone;not null"`
	ClosedAt     *time.Time     `gorm:"column:closed_at;type:timestamp(3) with time zone"`
	UpdatedAt    time.Time      `gorm:"column:updated_at;autoUpdateTime;default:current_timestamp"`
}

func (BalanceAlertEpisode) TableName() string { return "BalanceAlertEpisode" }

type BalanceAlertEvent struct {
	ID         uuid.UUID      `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	UserUID    uuid.UUID      `gorm:"column:user_uid;type:uuid;not null;uniqueIndex:idx_balance_alert_event,priority:1"`
	EpisodeID  uuid.UUID      `gorm:"column:episode_id;type:uuid;not null;uniqueIndex:idx_balance_alert_event,priority:2"`
	AlertLevel DebtStatusType `gorm:"column:alert_level;type:text;not null;uniqueIndex:idx_balance_alert_event,priority:3"`
	Payload    string         `gorm:"column:payload;type:text;not null"`
	CreatedAt  time.Time      `gorm:"column:created_at;autoCreateTime;default:current_timestamp"`
}

func (BalanceAlertEvent) TableName() string { return "BalanceAlertEvent" }

type BalanceAlertDelivery struct {
	ID           uuid.UUID                  `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	AlertEventID uuid.UUID                  `gorm:"column:alert_event_id;type:uuid;not null;uniqueIndex:idx_balance_alert_delivery,priority:1"`
	Channel      BalanceAlertChannel        `gorm:"column:channel;type:text;not null;uniqueIndex:idx_balance_alert_delivery,priority:2"`
	Recipient    string                     `gorm:"column:recipient;type:text;not null;uniqueIndex:idx_balance_alert_delivery,priority:3"`
	Payload      string                     `gorm:"column:payload;type:text;not null"`
	Status       BalanceAlertDeliveryStatus `gorm:"column:status;type:text;not null;index:idx_balance_alert_delivery_due,priority:1"`
	Attempts     int                        `gorm:"column:attempts;not null;default:0"`
	AvailableAt  time.Time                  `gorm:"column:available_at;type:timestamp(3) with time zone;not null;index:idx_balance_alert_delivery_due,priority:2"`
	LeaseOwner   string                     `gorm:"column:lease_owner;type:text"`
	LeaseUntil   *time.Time                 `gorm:"column:lease_until;type:timestamp(3) with time zone"`
	LastError    string                     `gorm:"column:last_error;type:text"`
	SentAt       *time.Time                 `gorm:"column:sent_at;type:timestamp(3) with time zone"`
	CreatedAt    time.Time                  `gorm:"column:created_at;autoCreateTime;default:current_timestamp"`
	UpdatedAt    time.Time                  `gorm:"column:updated_at;autoUpdateTime;default:current_timestamp"`
}

func (BalanceAlertDelivery) TableName() string { return "BalanceAlertDelivery" }

// BalanceAlertQueue is a coalescing, per-user prediction queue.
type BalanceAlertQueue struct {
	UserUID     uuid.UUID  `gorm:"column:user_uid;type:uuid;primaryKey"`
	Reason      string     `gorm:"column:reason;type:text;not null"`
	AvailableAt time.Time  `gorm:"column:available_at;type:timestamp(3) with time zone;not null;index"`
	LeaseOwner  string     `gorm:"column:lease_owner;type:text"`
	LeaseToken  string     `gorm:"column:lease_token;type:text"`
	LeaseUntil  *time.Time `gorm:"column:lease_until;type:timestamp(3) with time zone"`
	CreatedAt   time.Time  `gorm:"column:created_at;autoCreateTime;default:current_timestamp"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;autoUpdateTime;default:current_timestamp"`
}

func (BalanceAlertQueue) TableName() string { return "BalanceAlertQueue" }
