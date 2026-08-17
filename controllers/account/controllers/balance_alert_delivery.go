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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"math"
	"strings"
	"time"

	v1 "github.com/labring/sealos/controllers/account/api/v1"
	utils2 "github.com/labring/sealos/controllers/account/controllers/utils"
	"github.com/labring/sealos/controllers/pkg/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	balanceAlertDeliveryPollInterval = 10 * time.Second
	balanceAlertDeliveryLease        = 2 * time.Minute
	balanceAlertDeliveryBatchSize    = 100
)

func (r *DebtReconciler) dispatchBalanceAlertDeliveriesLoop(ctx context.Context) {
	run := func() {
		if err := r.dispatchBalanceAlertDeliveries(ctx); err != nil {
			r.Error(err, "failed to dispatch balance alert deliveries")
		}
	}
	run()
	ticker := time.NewTicker(balanceAlertDeliveryPollInterval)
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

func (r *DebtReconciler) dispatchBalanceAlertDeliveries(ctx context.Context) error {
	deliveries, err := r.claimBalanceAlertDeliveries(ctx, balanceAlertDeliveryBatchSize)
	if err != nil {
		return err
	}
	var deliveryErrors []error
	for i := range deliveries {
		deliveryErr := r.sendBalanceAlertDelivery(deliveries[i])
		if err := r.finishBalanceAlertDelivery(ctx, deliveries[i], deliveryErr); err != nil {
			deliveryErrors = append(deliveryErrors, err)
			continue
		}
		if deliveryErr != nil {
			r.Error(
				deliveryErr,
				"balance alert delivery will be retried",
				"deliveryID", deliveries[i].ID,
				"channel", deliveries[i].Channel,
			)
		}
	}
	return errors.Join(deliveryErrors...)
}

func (r *DebtReconciler) claimBalanceAlertDeliveries(
	ctx context.Context,
	limit int,
) ([]types.BalanceAlertDelivery, error) {
	now := time.Now().UTC()
	leaseUntil := now.Add(balanceAlertDeliveryLease)
	var deliveries []types.BalanceAlertDelivery
	err := r.AccountV2.GlobalTransactionHandler(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where(`(
				status IN ? AND available_at <= ? AND (lease_until IS NULL OR lease_until <= ?)
			) OR (status = ? AND lease_until <= ?)`,
				[]types.BalanceAlertDeliveryStatus{
					types.BalanceAlertDeliveryPending,
					types.BalanceAlertDeliveryRetry,
				},
				now,
				now,
				types.BalanceAlertDeliveryProcessing,
				now,
			).
			Order("available_at ASC, created_at ASC").
			Limit(limit).
			Find(&deliveries).Error; err != nil {
			return fmt.Errorf("claim balance alert delivery rows: %w", err)
		}
		if len(deliveries) == 0 {
			return nil
		}
		ids := make([]any, len(deliveries))
		for i := range deliveries {
			ids[i] = deliveries[i].ID
		}
		if err := tx.Model(&types.BalanceAlertDelivery{}).
			Where("id IN ?", ids).
			Updates(map[string]any{
				"status":      types.BalanceAlertDeliveryProcessing,
				"lease_owner": r.processID,
				"lease_until": leaseUntil,
				"updated_at":  now,
			}).Error; err != nil {
			return fmt.Errorf("lease balance alert deliveries: %w", err)
		}
		for i := range deliveries {
			deliveries[i].Status = types.BalanceAlertDeliveryProcessing
			deliveries[i].LeaseOwner = r.processID
			deliveries[i].LeaseUntil = &leaseUntil
		}
		return nil
	})
	return deliveries, err
}

func (r *DebtReconciler) sendBalanceAlertDelivery(
	delivery types.BalanceAlertDelivery,
) error {
	var payload balanceAlertPayload
	if err := json.Unmarshal([]byte(delivery.Payload), &payload); err != nil {
		return fmt.Errorf("decode balance alert delivery payload: %w", err)
	}
	switch delivery.Channel {
	case types.BalanceAlertChannelEmail:
		if r.smtpConfig == nil {
			return errors.New("SMTP is unavailable")
		}
		subject, body, err := renderBalanceAlertEmail(payload)
		if err != nil {
			return err
		}
		if override := r.SendDebtStatusEmailBody[v1.DebtStatusType(payload.AlertLevel)]; override != "" {
			body = override
		}
		if err := r.smtpConfig.SendEmailWithTitle(subject, body, delivery.Recipient); err != nil {
			return fmt.Errorf("send balance alert email: %w", err)
		}
		return nil
	case types.BalanceAlertChannelSMS:
		if r.SmsConfig == nil {
			return errors.New("SMS is unavailable")
		}
		templateCode := r.SmsConfig.SmsCode[string(payload.AlertLevel)]
		if templateCode == "" {
			return fmt.Errorf("SMS template is unavailable for %s", payload.AlertLevel)
		}
		params, err := balanceAlertSMSParams(payload)
		if err != nil {
			return err
		}
		if err := utils2.SendSmsMultiple(
			r.SmsConfig.Client,
			[]string{delivery.Recipient},
			r.SmsConfig.SmsSignName,
			templateCode,
			params,
		); err != nil {
			return fmt.Errorf("send balance alert SMS: %w", err)
		}
		return nil
	case types.BalanceAlertChannelVoice:
		if r.VmsConfig == nil {
			return errors.New("voice messaging is unavailable")
		}
		templateCode := r.VmsConfig.TemplateCode[string(payload.AlertLevel)]
		if templateCode == "" {
			return fmt.Errorf("voice template is unavailable for %s", payload.AlertLevel)
		}
		if err := utils2.SendVmsWithOpenID(
			delivery.Recipient,
			templateCode,
			r.VmsConfig.NumberPoll,
			GetSendVmsTimeInUTCPlus8(time.Now()),
			forbidTimes,
			delivery.AlertEventID.String()+"-"+delivery.Recipient,
		); err != nil {
			return fmt.Errorf("send balance alert voice message: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported balance alert channel %q", delivery.Channel)
	}
}

func (r *DebtReconciler) finishBalanceAlertDelivery(
	ctx context.Context,
	delivery types.BalanceAlertDelivery,
	deliveryErr error,
) error {
	now := time.Now().UTC()
	updates := map[string]any{
		"attempts":    gorm.Expr("attempts + 1"),
		"lease_owner": "",
		"lease_until": nil,
		"updated_at":  now,
	}
	if deliveryErr == nil {
		updates["status"] = types.BalanceAlertDeliverySent
		updates["sent_at"] = now
		updates["last_error"] = ""
	} else {
		updates["status"] = types.BalanceAlertDeliveryRetry
		updates["available_at"] = now.Add(balanceAlertRetryDelay(delivery.Attempts + 1))
		updates["last_error"] = truncateBalanceAlertError(deliveryErr.Error())
	}
	result := r.AccountV2.GetGlobalDB().WithContext(ctx).
		Model(&types.BalanceAlertDelivery{}).
		Where("id = ? AND lease_owner = ?", delivery.ID, r.processID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("finish balance alert delivery %s: %w", delivery.ID, result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("balance alert delivery %s lease was lost", delivery.ID)
	}
	return nil
}

func balanceAlertRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	exponent := min(attempt-1, 10)
	delay := time.Minute * time.Duration(1<<exponent)
	return min(delay, 6*time.Hour)
}

func truncateBalanceAlertError(message string) string {
	const maxLength = 2048
	if len(message) <= maxLength {
		return message
	}
	return message[:maxLength]
}

func balanceAlertSMSParams(payload balanceAlertPayload) (string, error) {
	eta := formatBalanceAlertETA(payload.ETASeconds, payload.Language)
	params := map[string]string{
		"user_id": payload.UserUID.String(),
		"oweamount": fmt.Sprintf(
			"%d",
			int64(math.Abs(math.Ceil(float64(payload.AvailableBalance)/BaseUnit))),
		),
		"balance": fmt.Sprintf("%.2f", float64(payload.AvailableBalance)/BaseUnit),
		"eta":     eta,
	}
	if payload.ExhaustedAt != nil {
		params["exhaust_at"] = payload.ExhaustedAt.In(UTCPlus8).Format("2006-01-02 15:04")
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("encode balance alert SMS parameters: %w", err)
	}
	return string(encoded), nil
}

func formatBalanceAlertETA(etaSeconds *int64, language string) string {
	if etaSeconds == nil {
		if language == "zh" {
			return "暂无耗尽时间"
		}
		return "No exhaustion time"
	}
	if *etaSeconds <= 0 {
		if language == "zh" {
			return "已耗尽"
		}
		return "Exhausted"
	}
	duration := time.Duration(*etaSeconds) * time.Second
	days := int(duration / (24 * time.Hour))
	hours := int(duration % (24 * time.Hour) / time.Hour)
	if language == "zh" {
		if days > 0 {
			return fmt.Sprintf("%d天%d小时", days, hours)
		}
		return fmt.Sprintf("%d小时", max(hours, 1))
	}
	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	return fmt.Sprintf("%dh", max(hours, 1))
}

type balanceAlertEmailView struct {
	Language       string
	UserName       string
	HasETA         bool
	Balance        string
	ETA            string
	ExhaustedAt    string
	DailyRate      string
	ShortDailyRate string
	LongDailyRate  string
	TopConsumer    string
	Confidence     string
	CostCenterURL  string
}

func renderBalanceAlertEmail(payload balanceAlertPayload) (string, string, error) {
	language := payload.Language
	if language != "zh" {
		language = "en"
	}
	view := balanceAlertEmailView{
		Language:       language,
		UserName:       payload.UserName,
		HasETA:         payload.ETASeconds != nil,
		Balance:        fmt.Sprintf("%.2f", float64(payload.AvailableBalance)/BaseUnit),
		ETA:            formatBalanceAlertETA(payload.ETASeconds, language),
		DailyRate:      fmt.Sprintf("%.2f", payload.ForecastRate*24/BaseUnit),
		ShortDailyRate: fmt.Sprintf("%.2f", payload.ShortHourlyRate*24/BaseUnit),
		LongDailyRate:  fmt.Sprintf("%.2f", payload.LongHourlyRate*24/BaseUnit),
		Confidence:     formatBalanceAlertConfidence(payload.Confidence, language),
		CostCenterURL:  balanceAlertCostCenterURL(payload.Domain),
	}
	if payload.ExhaustedAt != nil {
		view.ExhaustedAt = payload.ExhaustedAt.In(UTCPlus8).Format("2006-01-02 15:04 MST")
	}
	consumer := strings.TrimSpace(payload.TopWorkspace)
	if payload.TopApplication != "" {
		if consumer != "" {
			consumer += " / "
		}
		consumer += payload.TopApplication
	}
	view.TopConsumer = consumer

	templateText := balanceAlertEmailTemplateEN
	if language == "zh" {
		templateText = balanceAlertEmailTemplateZH
	}
	subject := balanceAlertEmailSubject(payload.AlertLevel, language)
	tmpl, err := template.New("balance-alert").Parse(templateText)
	if err != nil {
		return "", "", fmt.Errorf("parse balance alert email template: %w", err)
	}
	var body bytes.Buffer
	if err := tmpl.Execute(&body, view); err != nil {
		return "", "", fmt.Errorf("render balance alert email: %w", err)
	}
	return subject, body.String(), nil
}

func formatBalanceAlertConfidence(
	confidence types.BalancePredictionConfidence,
	language string,
) string {
	if language == "zh" {
		if confidence == types.BalancePredictionConfidenceHigh {
			return "高"
		}
		return "低"
	}
	return string(confidence)
}

func balanceAlertEmailSubject(status types.DebtStatusType, language string) string {
	if language == "zh" {
		switch status {
		case types.CriticalBalancePeriod:
			return "重要提醒：账户余额预计 1 天内耗尽"
		case types.DebtPeriod:
			return "重要提醒：账户余额已耗尽"
		case types.DebtDeletionPeriod:
			return "重要提醒：资源即将释放"
		case types.FinalDeletionPeriod:
			return "重要提醒：资源已删除"
		default:
			return "Sealos 账户余额预计可用时间提醒"
		}
	}
	switch status {
	case types.CriticalBalancePeriod:
		return "Important: your account balance may run out within one day"
	case types.DebtPeriod:
		return "Important: your account balance is exhausted"
	case types.DebtDeletionPeriod:
		return "Important: your resources are scheduled for release"
	case types.FinalDeletionPeriod:
		return "Important: your resources have been deleted"
	default:
		return "Sealos account balance forecast alert"
	}
}

func balanceAlertCostCenterURL(domain string) string {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return ""
	}
	if !strings.HasPrefix(domain, "http://") && !strings.HasPrefix(domain, "https://") {
		domain = "https://" + domain
	}
	return strings.TrimRight(domain, "/") + "/?openapp=system-costcenter"
}

const balanceAlertEmailTemplateZH = `<!doctype html>
<html lang="zh-CN"><body style="font-family:Arial,sans-serif;color:#222;line-height:1.6">
<div style="max-width:600px;margin:auto;padding:32px;border:1px solid #e5e7eb">
<h2 style="margin-top:0">账户余额预计可用时间提醒</h2>
{{if .HasETA}}<p>{{if .UserName}}{{.UserName}}，{{end}}您的账户余额预计可用约 <strong>{{.ETA}}</strong>。</p>
{{else}}<p>{{if .UserName}}{{.UserName}}，{{end}}您的账户余额已进入告警范围，预测数据正在积累。</p>{{end}}
<table style="width:100%;border-collapse:collapse">
<tr><td>当前可用余额</td><td>¥{{.Balance}}</td></tr>
<tr><td>近期预测消耗</td><td>¥{{.DailyRate}}/天</td></tr>
<tr><td>最近 6 小时加权消耗</td><td>¥{{.ShortDailyRate}}/天</td></tr>
<tr><td>最近 48 小时平均消耗</td><td>¥{{.LongDailyRate}}/天</td></tr>
{{if .ExhaustedAt}}<tr><td>预计耗尽时间</td><td>{{.ExhaustedAt}}</td></tr>{{end}}
{{if .TopConsumer}}<tr><td>主要消费项</td><td>{{.TopConsumer}}</td></tr>{{end}}
<tr><td>预测置信度</td><td>{{.Confidence}}</td></tr>
</table>
{{if .CostCenterURL}}<p style="margin-top:28px"><a href="{{.CostCenterURL}}" style="color:#2563eb">打开 Cost Center 并充值</a></p>{{end}}
</div></body></html>`

const balanceAlertEmailTemplateEN = `<!doctype html>
<html lang="en"><body style="font-family:Arial,sans-serif;color:#222;line-height:1.6">
<div style="max-width:600px;margin:auto;padding:32px;border:1px solid #e5e7eb">
<h2 style="margin-top:0">Account balance forecast alert</h2>
{{if .HasETA}}<p>{{if .UserName}}{{.UserName}}, {{end}}your account balance is expected to last about <strong>{{.ETA}}</strong>.</p>
{{else}}<p>{{if .UserName}}{{.UserName}}, {{end}}your balance has entered the alert range while forecast data is accumulating.</p>{{end}}
<table style="width:100%;border-collapse:collapse">
<tr><td>Available balance</td><td>{{.Balance}}</td></tr>
<tr><td>Forecast spend</td><td>{{.DailyRate}}/day</td></tr>
<tr><td>Recent 6-hour weighted spend</td><td>{{.ShortDailyRate}}/day</td></tr>
<tr><td>48-hour average spend</td><td>{{.LongDailyRate}}/day</td></tr>
{{if .ExhaustedAt}}<tr><td>Estimated exhaustion</td><td>{{.ExhaustedAt}}</td></tr>{{end}}
{{if .TopConsumer}}<tr><td>Top consumer</td><td>{{.TopConsumer}}</td></tr>{{end}}
<tr><td>Forecast confidence</td><td>{{.Confidence}}</td></tr>
</table>
{{if .CostCenterURL}}<p style="margin-top:28px"><a href="{{.CostCenterURL}}" style="color:#2563eb">Open Cost Center and recharge</a></p>{{end}}
</div></body></html>`
