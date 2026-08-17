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
	"math"
	"sort"
	"time"

	v1 "github.com/labring/sealos/controllers/account/api/v1"
	"github.com/labring/sealos/controllers/pkg/types"
)

const (
	balanceForecastLongWindowHours  = 48
	balanceForecastShortWindowHours = 6
	balanceForecastHalfLifeHours    = 3.0
	balanceRiskWindow               = 3 * 24 * time.Hour
	balanceCriticalWindow           = 24 * time.Hour
	balanceRecoveryWindow           = 4 * 24 * time.Hour
	balanceWatermarkMaxLag          = 2 * time.Hour
)

type hourlyBalanceUsage struct {
	Hour   time.Time
	Amount int64
}

type expiringCredit struct {
	Amount   int64
	ExpireAt time.Time
}

type balancePrediction struct {
	AvailableBalance int64
	DataThrough      time.Time
	LongRate         float64
	ShortRate        float64
	ForecastRate     float64
	ETA              *time.Duration
	ExhaustedAt      *time.Time
	Confidence       types.BalancePredictionConfidence
	ConfidenceReason string
	TopWorkspace     string
	TopApplication   string
}

func calculateForecastRates(
	dataThrough time.Time,
	usage []hourlyBalanceUsage,
) (longRate, shortRate, forecastRate float64) {
	dataThrough = dataThrough.UTC().Truncate(time.Hour)
	amountByHour := make(map[time.Time]int64, len(usage))
	for i := range usage {
		hour := usage[i].Hour.UTC().Truncate(time.Hour)
		amountByHour[hour] += usage[i].Amount
	}

	longStart := dataThrough.Add(-(balanceForecastLongWindowHours - 1) * time.Hour)
	var longTotal int64
	for i := range balanceForecastLongWindowHours {
		longTotal += amountByHour[longStart.Add(time.Duration(i)*time.Hour)]
	}
	longRate = float64(longTotal) / balanceForecastLongWindowHours

	shortStart := dataThrough.Add(-(balanceForecastShortWindowHours - 1) * time.Hour)
	var weightedTotal, totalWeight float64
	for i := range balanceForecastShortWindowHours {
		age := float64(balanceForecastShortWindowHours - 1 - i)
		weight := math.Pow(0.5, age/balanceForecastHalfLifeHours)
		weightedTotal += float64(amountByHour[shortStart.Add(time.Duration(i)*time.Hour)]) * weight
		totalWeight += weight
	}
	if totalWeight > 0 {
		shortRate = weightedTotal / totalWeight
	}
	forecastRate = math.Max(longRate, shortRate)
	return longRate, shortRate, forecastRate
}

func projectBalanceExhaustion(
	from time.Time,
	cashBalance int64,
	credits []expiringCredit,
	hourlyRate float64,
) *time.Time {
	if hourlyRate <= 0 {
		return nil
	}

	type creditBucket struct {
		amount   float64
		expireAt time.Time
	}
	buckets := make([]creditBucket, 0, len(credits))
	for i := range credits {
		if credits[i].Amount <= 0 || !credits[i].ExpireAt.After(from) {
			continue
		}
		buckets = append(buckets, creditBucket{
			amount:   float64(credits[i].Amount),
			expireAt: credits[i].ExpireAt,
		})
	}
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].expireAt.Before(buckets[j].expireAt)
	})
	cash := float64(cashBalance)
	totalAvailable := func() float64 {
		total := cash
		for i := range buckets {
			total += buckets[i].amount
		}
		return total
	}
	consume := func(amount float64) {
		for i := range buckets {
			used := math.Min(buckets[i].amount, amount)
			buckets[i].amount -= used
			amount -= used
			if amount <= 0 {
				return
			}
		}
		cash -= amount
	}

	if totalAvailable() <= 0 {
		exhaustedAt := from
		return &exhaustedAt
	}
	cursor := from
	for i := 0; i < len(buckets); {
		expireAt := buckets[i].expireAt
		available := totalAvailable()
		spendUntilExpiry := hourlyRate * expireAt.Sub(cursor).Hours()
		if available <= spendUntilExpiry {
			exhaustedAt := cursor.Add(time.Duration(available / hourlyRate * float64(time.Hour)))
			return &exhaustedAt
		}
		consume(spendUntilExpiry)
		cursor = expireAt
		for i < len(buckets) && buckets[i].expireAt.Equal(expireAt) {
			buckets[i].amount = 0
			i++
		}
		if totalAvailable() <= 0 {
			exhaustedAt := cursor
			return &exhaustedAt
		}
	}

	exhaustedAt := cursor.Add(
		time.Duration(totalAvailable() / hourlyRate * float64(time.Hour)),
	)
	return &exhaustedAt
}

type balanceRiskAssessment struct {
	Status    types.DebtStatusType
	Risk      bool
	Immediate bool
	Recovered bool
}

func assessBalanceRisk(
	prediction balancePrediction,
	lastStatus types.DebtStatusType,
	statusAgeSeconds int64,
) balanceRiskAssessment {
	if prediction.AvailableBalance <= 0 {
		status := determineCurrentStatus(
			prediction.AvailableBalance,
			statusAgeSeconds,
			v1.DebtStatusType(lastStatus),
		)
		return balanceRiskAssessment{Status: types.DebtStatusType(status), Risk: true, Immediate: true}
	}

	if prediction.Confidence == types.BalancePredictionConfidenceLow {
		status := types.DebtStatusType(determineCurrentStatus(
			prediction.AvailableBalance,
			statusAgeSeconds,
			v1.DebtStatusType(lastStatus),
		))
		return balanceRiskAssessment{
			Status:    status,
			Risk:      status == types.LowBalancePeriod || status == types.CriticalBalancePeriod,
			Immediate: status == types.CriticalBalancePeriod,
			Recovered: status == types.NormalPeriod,
		}
	}

	if prediction.ETA == nil {
		return balanceRiskAssessment{Status: types.NormalPeriod, Recovered: true}
	}
	eta := *prediction.ETA
	switch {
	case eta <= balanceCriticalWindow:
		return balanceRiskAssessment{
			Status: types.CriticalBalancePeriod, Risk: true, Immediate: true,
		}
	case eta <= balanceRiskWindow:
		return balanceRiskAssessment{
			Status: types.LowBalancePeriod,
			Risk:   true,
			Immediate: types.ContainDebtStatus(
				types.DebtStates,
				lastStatus,
			),
		}
	case eta > balanceRecoveryWindow:
		return balanceRiskAssessment{Status: types.NormalPeriod, Recovered: true}
	default:
		return balanceRiskAssessment{Status: types.NormalPeriod}
	}
}

type episodeTransitionInput struct {
	Active        bool
	RiskCount     int
	RecoveryCount int
	HighestLevel  types.DebtStatusType
	Assessment    balanceRiskAssessment
}

type episodeTransition struct {
	RiskCount      int
	RecoveryCount  int
	HighestLevel   types.DebtStatusType
	CreateEpisode  bool
	CloseEpisode   bool
	EmitAlertEvent bool
}

func nextEpisodeTransition(input episodeTransitionInput) episodeTransition {
	result := episodeTransition{
		RiskCount:     input.RiskCount,
		RecoveryCount: input.RecoveryCount,
		HighestLevel:  input.HighestLevel,
	}
	if !input.Active {
		result.RecoveryCount = 0
		if !input.Assessment.Risk {
			result.RiskCount = 0
			return result
		}
		result.RiskCount++
		if !input.Assessment.Immediate && result.RiskCount < 2 {
			return result
		}
		result.RiskCount = 0
		result.CreateEpisode = true
		result.EmitAlertEvent = true
		result.HighestLevel = input.Assessment.Status
		return result
	}

	result.RiskCount = 0
	if input.Assessment.Recovered {
		result.RecoveryCount++
		if result.RecoveryCount >= 2 {
			result.RecoveryCount = 0
			result.CloseEpisode = true
		}
		return result
	}
	result.RecoveryCount = 0
	if input.Assessment.Risk &&
		types.StatusMap[input.Assessment.Status] > types.StatusMap[input.HighestLevel] {
		result.HighestLevel = input.Assessment.Status
		result.EmitAlertEvent = true
	}
	return result
}
