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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labring/sealos/controllers/pkg/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type balanceAlertPayload struct {
	UserUID          uuid.UUID                         `json:"user_uid"`
	UserID           string                            `json:"user_id"`
	UserName         string                            `json:"user_name"`
	AlertLevel       types.DebtStatusType              `json:"alert_level"`
	AvailableBalance int64                             `json:"available_balance"`
	ETASeconds       *int64                            `json:"eta_seconds,omitempty"`
	ExhaustedAt      *time.Time                        `json:"exhausted_at,omitempty"`
	LongHourlyRate   float64                           `json:"long_hourly_rate"`
	ShortHourlyRate  float64                           `json:"short_hourly_rate"`
	ForecastRate     float64                           `json:"forecast_hourly_rate"`
	TopWorkspace     string                            `json:"top_workspace,omitempty"`
	TopApplication   string                            `json:"top_application,omitempty"`
	Confidence       types.BalancePredictionConfidence `json:"confidence"`
	ConfidenceReason string                            `json:"confidence_reason,omitempty"`
	DataThrough      time.Time                         `json:"data_through,omitempty"`
	Domain           string                            `json:"domain"`
	Language         string                            `json:"language"`
}

type balanceAlertDeliverySpec struct {
	Channel   types.BalanceAlertChannel
	Recipient string
}

func (r *DebtReconciler) loadBalancePrediction(
	userUID uuid.UUID,
	account *types.UsableBalanceWithCredits,
	now time.Time,
) (balancePrediction, error) {
	prediction := balancePrediction{
		AvailableBalance: account.Balance - account.DeductionBalance + account.UsableCredits,
		Confidence:       types.BalancePredictionConfidenceLow,
	}
	regions, err := r.AccountV2.GetRegions()
	if err != nil {
		return prediction, fmt.Errorf("get prediction regions: %w", err)
	}
	if len(regions) == 0 {
		prediction.ConfidenceReason = "no billing regions are registered"
		return prediction, nil
	}

	var watermarks []types.BalanceBillingWatermark
	if err := r.AccountV2.GetGlobalDB().Find(&watermarks).Error; err != nil {
		return prediction, fmt.Errorf("get billing watermarks: %w", err)
	}
	prediction.DataThrough, prediction.ConfidenceReason = resolveBalanceDataThrough(
		regions,
		watermarks,
		now,
	)
	if prediction.ConfidenceReason != "" {
		return prediction, nil
	}

	windowStart := prediction.DataThrough.Add(-balanceForecastLongWindowHours * time.Hour)
	var usageRows []types.BalanceUsageHour
	if err := r.AccountV2.GetGlobalDB().
		Where("user_uid = ? AND usage_hour > ? AND usage_hour <= ?", userUID, windowStart, prediction.DataThrough).
		Order("usage_hour ASC").
		Find(&usageRows).Error; err != nil {
		return prediction, fmt.Errorf("get hourly balance usage: %w", err)
	}
	usage := make([]hourlyBalanceUsage, len(usageRows))
	for i := range usageRows {
		usage[i] = hourlyBalanceUsage{Hour: usageRows[i].UsageHour, Amount: usageRows[i].Amount}
	}
	prediction.LongRate, prediction.ShortRate, prediction.ForecastRate =
		calculateForecastRates(prediction.DataThrough, usage)

	credits, err := r.AccountV2.GetAvailableCredits(&types.UserQueryOpts{UID: userUID})
	if err != nil {
		return prediction, fmt.Errorf("get expiring credits: %w", err)
	}
	expiringCredits := make([]expiringCredit, 0, len(credits))
	var detailedCredits int64
	for i := range credits {
		remaining := credits[i].Amount - credits[i].UsedAmount
		if remaining <= 0 || !credits[i].ExpireAt.After(now) {
			continue
		}
		detailedCredits += remaining
		expiringCredits = append(expiringCredits, expiringCredit{
			Amount: remaining, ExpireAt: credits[i].ExpireAt,
		})
	}
	if detailedCredits != account.UsableCredits {
		prediction.ConfidenceReason = "the account and credits snapshots are inconsistent"
		return prediction, nil
	}

	prediction.Confidence = types.BalancePredictionConfidenceHigh
	exhaustedAt := projectBalanceExhaustion(
		now,
		account.Balance-account.DeductionBalance,
		expiringCredits,
		prediction.ForecastRate,
	)
	if exhaustedAt != nil {
		eta := exhaustedAt.Sub(now)
		if eta < 0 {
			eta = 0
		}
		prediction.ETA = &eta
		prediction.ExhaustedAt = exhaustedAt
	}

	var top struct {
		Workspace string
		AppName   string
	}
	if err := r.AccountV2.GetGlobalDB().Model(&types.BalanceUsageEvent{}).
		Select("workspace, app_name, SUM(amount) AS total_amount").
		Where("user_uid = ? AND usage_hour > ? AND usage_hour <= ?", userUID, windowStart, prediction.DataThrough).
		Group("workspace, app_name").
		Order("total_amount DESC").
		Limit(1).
		Scan(&top).Error; err != nil {
		return prediction, fmt.Errorf("get top balance consumer: %w", err)
	}
	prediction.TopWorkspace = top.Workspace
	prediction.TopApplication = top.AppName
	return prediction, nil
}

func resolveBalanceDataThrough(
	regions []types.Region,
	watermarks []types.BalanceBillingWatermark,
	now time.Time,
) (time.Time, string) {
	if len(regions) == 0 {
		return time.Time{}, "no billing regions are registered"
	}
	watermarkByRegion := make(map[uuid.UUID]types.BalanceBillingWatermark, len(watermarks))
	var dataThrough time.Time
	for i := range watermarks {
		watermarks[i].DataThrough = watermarks[i].DataThrough.UTC().Truncate(time.Hour)
		watermarkByRegion[watermarks[i].RegionUID] = watermarks[i]
	}
	for i := range regions {
		watermark, exists := watermarkByRegion[regions[i].UID]
		if !exists || watermark.DataThrough.IsZero() {
			return time.Time{}, "one or more billing regions have no watermark"
		}
		if watermark.ConsecutiveHours < balanceForecastLongWindowHours {
			return time.Time{}, "one or more billing regions have less than 48 complete hours"
		}
		if dataThrough.IsZero() || watermark.DataThrough.Before(dataThrough) {
			dataThrough = watermark.DataThrough
		}
	}
	lag := now.UTC().Sub(dataThrough)
	if lag < 0 || lag > balanceWatermarkMaxLag {
		return time.Time{}, "the global billing watermark is stale"
	}
	return dataThrough, ""
}

func (r *DebtReconciler) effectiveBalanceStatus(
	userUID uuid.UUID,
	lastStatus types.DebtStatusType,
	assessment balanceRiskAssessment,
) (types.DebtStatusType, error) {
	lastIsBalanceRisk := lastStatus == types.LowBalancePeriod ||
		lastStatus == types.CriticalBalancePeriod
	if assessment.Status == types.LowBalancePeriod && lastStatus != types.NormalPeriod {
		if lastIsBalanceRisk &&
			types.StatusMap[lastStatus] >= types.StatusMap[assessment.Status] {
			return lastStatus, nil
		}
		return assessment.Status, nil
	}
	needsState := assessment.Status == types.LowBalancePeriod ||
		(assessment.Status == types.NormalPeriod && lastIsBalanceRisk)
	if !needsState {
		return assessment.Status, nil
	}
	var state types.BalanceAlertState
	err := r.AccountV2.GetGlobalDB().Where("user_uid = ?", userUID).First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if assessment.Status == types.LowBalancePeriod {
			return lastStatus, nil
		}
		return assessment.Status, nil
	}
	if err != nil {
		return "", fmt.Errorf("get balance alert debounce state: %w", err)
	}
	if assessment.Status == types.LowBalancePeriod {
		if state.ActiveEpisodeID == nil && state.RiskCount < 1 {
			return lastStatus, nil
		}
		return assessment.Status, nil
	}
	if state.ActiveEpisodeID != nil &&
		(!assessment.Recovered || state.RecoveryCount < 1) {
		return lastStatus, nil
	}
	return types.NormalPeriod, nil
}

func (r *DebtReconciler) balanceAlertAudience(
	userUID uuid.UUID,
	status types.DebtStatusType,
	skipLowBalance bool,
) (string, string, []balanceAlertDeliverySpec, error) {
	user, err := r.AccountV2.GetUser(&types.UserQueryOpts{UID: userUID})
	if err != nil {
		return "", "", nil, fmt.Errorf("get balance alert user: %w", err)
	}
	if user.Status != types.UserStatusNormal || skipLowBalance {
		return user.ID, user.Name, nil, nil
	}
	providers, err := r.AccountV2.GetUserOauthProvider(&types.UserQueryOpts{
		UID: user.UID, ID: user.ID,
	})
	if err != nil {
		return "", "", nil, fmt.Errorf("get balance alert recipients: %w", err)
	}
	specs := make([]balanceAlertDeliverySpec, 0, len(providers)*2)
	for i := range providers {
		switch providers[i].ProviderType {
		case types.OauthProviderTypeEmail:
			if r.smtpConfig != nil {
				specs = append(specs, balanceAlertDeliverySpec{
					Channel: types.BalanceAlertChannelEmail, Recipient: providers[i].ProviderID,
				})
			}
		case types.OauthProviderTypePhone:
			if r.SmsConfig != nil && r.SmsConfig.SmsCode[string(status)] != "" {
				specs = append(specs, balanceAlertDeliverySpec{
					Channel: types.BalanceAlertChannelSMS, Recipient: providers[i].ProviderID,
				})
			}
			if r.VmsConfig != nil && types.ContainDebtStatus(types.DebtStates, status) &&
				r.VmsConfig.TemplateCode[string(status)] != "" {
				specs = append(specs, balanceAlertDeliverySpec{
					Channel: types.BalanceAlertChannelVoice, Recipient: providers[i].ProviderID,
				})
			}
		}
	}
	return user.ID, user.Name, specs, nil
}

func (r *DebtReconciler) applyBalanceAlertEpisode(
	tx *gorm.DB,
	userUID uuid.UUID,
	assessment balanceRiskAssessment,
	prediction balancePrediction,
	payload balanceAlertPayload,
	deliverySpecs []balanceAlertDeliverySpec,
	now time.Time,
) error {
	initialState := types.BalanceAlertState{
		UserUID: userUID, LastEvaluatedAt: now,
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&initialState).Error; err != nil {
		return fmt.Errorf("initialize balance alert state: %w", err)
	}
	var state types.BalanceAlertState
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_uid = ?", userUID).First(&state).Error; err != nil {
		return fmt.Errorf("lock balance alert state: %w", err)
	}

	var episode types.BalanceAlertEpisode
	active := state.ActiveEpisodeID != nil
	if active {
		err := tx.Where("id = ?", *state.ActiveEpisodeID).First(&episode).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			active = false
			state.ActiveEpisodeID = nil
		} else if err != nil {
			return fmt.Errorf("get active balance alert episode: %w", err)
		}
	}
	transition := nextEpisodeTransition(episodeTransitionInput{
		Active: active, RiskCount: state.RiskCount, RecoveryCount: state.RecoveryCount,
		HighestLevel: episode.HighestLevel, Assessment: assessment,
	})
	state.RiskCount = transition.RiskCount
	state.RecoveryCount = transition.RecoveryCount

	if transition.CreateEpisode {
		activeKey := userUID
		episode = types.BalanceAlertEpisode{
			ID: uuid.New(), UserUID: userUID, ActiveKey: &activeKey,
			HighestLevel: transition.HighestLevel, StartedAt: now,
		}
		if err := tx.Create(&episode).Error; err != nil {
			return fmt.Errorf("create balance alert episode: %w", err)
		}
		state.ActiveEpisodeID = &episode.ID
		active = true
	}
	if transition.CloseEpisode && active {
		closedAt := now
		if err := tx.Model(&types.BalanceAlertEpisode{}).
			Where("id = ?", episode.ID).
			Updates(map[string]any{
				"active_key": nil, "closed_at": closedAt, "updated_at": now,
			}).Error; err != nil {
			return fmt.Errorf("close balance alert episode: %w", err)
		}
		state.ActiveEpisodeID = nil
		active = false
	}
	if transition.EmitAlertEvent && active {
		payload.AlertLevel = assessment.Status
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal balance alert payload: %w", err)
		}
		event := types.BalanceAlertEvent{
			ID: uuid.New(), UserUID: userUID, EpisodeID: episode.ID,
			AlertLevel: assessment.Status, Payload: string(payloadJSON), CreatedAt: now,
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&event)
		if result.Error != nil {
			return fmt.Errorf("create balance alert event: %w", result.Error)
		}
		if result.RowsAffected > 0 {
			for i := range deliverySpecs {
				availableAt := now
				if deliverySpecs[i].Channel == types.BalanceAlertChannelVoice {
					availableAt = GetSendVmsTimeInUTCPlus8(now)
				}
				delivery := types.BalanceAlertDelivery{
					ID: uuid.New(), AlertEventID: event.ID,
					Channel: deliverySpecs[i].Channel, Recipient: deliverySpecs[i].Recipient,
					Payload: string(payloadJSON), Status: types.BalanceAlertDeliveryPending,
					AvailableAt: availableAt, CreatedAt: now,
				}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&delivery).Error; err != nil {
					return fmt.Errorf("create balance alert delivery: %w", err)
				}
			}
		}
		if episode.HighestLevel != transition.HighestLevel {
			if err := tx.Model(&types.BalanceAlertEpisode{}).
				Where("id = ?", episode.ID).
				Update("highest_level", transition.HighestLevel).Error; err != nil {
				return fmt.Errorf("update balance alert episode level: %w", err)
			}
		}
	}

	state.LastRiskLevel = assessment.Status
	state.LastConfidence = prediction.Confidence
	state.LastEvaluatedAt = now
	state.LastETASeconds = nil
	if prediction.ETA != nil {
		etaSeconds := int64(prediction.ETA.Seconds())
		state.LastETASeconds = &etaSeconds
	}
	if err := tx.Save(&state).Error; err != nil {
		return fmt.Errorf("save balance alert state: %w", err)
	}
	return nil
}
