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
	"testing"
	"time"

	"github.com/labring/sealos/controllers/pkg/types"
)

func TestCalculateForecastRatesUsesRecentSpike(t *testing.T) {
	dataThrough := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	usage := make([]hourlyBalanceUsage, 0, balanceForecastLongWindowHours)
	for i := balanceForecastLongWindowHours - 1; i >= 0; i-- {
		amount := int64(10)
		if i < balanceForecastShortWindowHours {
			amount = 100
		}
		usage = append(usage, hourlyBalanceUsage{
			Hour: dataThrough.Add(-time.Duration(i) * time.Hour), Amount: amount,
		})
	}

	longRate, shortRate, forecastRate := calculateForecastRates(dataThrough, usage)
	if math.Abs(longRate-21.25) > 0.001 {
		t.Fatalf("long rate = %.3f, want 21.25", longRate)
	}
	if math.Abs(shortRate-100) > 0.001 {
		t.Fatalf("short rate = %.3f, want 100", shortRate)
	}
	if forecastRate != shortRate {
		t.Fatalf("forecast rate = %.3f, want short rate %.3f", forecastRate, shortRate)
	}
}

func TestProjectBalanceExhaustionRemovesExpiredCredits(t *testing.T) {
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	exhaustedAt := projectBalanceExhaustion(now, 10, []expiringCredit{{
		Amount: 10, ExpireAt: now.Add(time.Hour),
	}}, 5)
	if exhaustedAt == nil {
		t.Fatal("expected an exhaustion time")
	}
	want := now.Add(3 * time.Hour)
	if !exhaustedAt.Equal(want) {
		t.Fatalf("exhausted at %s, want %s", exhaustedAt, want)
	}
}

func TestProjectBalanceExhaustionConsumesEarliestCreditsFirst(t *testing.T) {
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	exhaustedAt := projectBalanceExhaustion(now, 10, []expiringCredit{{
		Amount: 10, ExpireAt: now.Add(time.Hour),
	}}, 15)
	want := now.Add(80 * time.Minute)
	if exhaustedAt == nil || !exhaustedAt.Equal(want) {
		t.Fatalf("exhausted at %v, want %s", exhaustedAt, want)
	}
}

func TestAssessBalanceRiskUsesFixedThresholdForLowConfidence(t *testing.T) {
	assessment := assessBalanceRisk(balancePrediction{
		AvailableBalance: 7 * BaseUnit,
		Confidence:       types.BalancePredictionConfidenceLow,
	}, types.NormalPeriod, 0)
	if assessment.Status != types.LowBalancePeriod || !assessment.Risk || assessment.Immediate {
		t.Fatalf("unexpected assessment: %+v", assessment)
	}
}

func TestAssessBalanceRiskETAThresholds(t *testing.T) {
	tests := []struct {
		name      string
		eta       time.Duration
		want      types.DebtStatusType
		recovered bool
	}{
		{name: "critical", eta: 24 * time.Hour, want: types.CriticalBalancePeriod},
		{name: "warning", eta: 72 * time.Hour, want: types.LowBalancePeriod},
		{name: "hysteresis", eta: 80 * time.Hour, want: types.NormalPeriod},
		{name: "recovered", eta: 97 * time.Hour, want: types.NormalPeriod, recovered: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eta := tt.eta
			assessment := assessBalanceRisk(balancePrediction{
				AvailableBalance: 100 * BaseUnit,
				ETA:              &eta,
				Confidence:       types.BalancePredictionConfidenceHigh,
			}, types.NormalPeriod, 0)
			if assessment.Status != tt.want || assessment.Recovered != tt.recovered {
				t.Fatalf("assessment = %+v, want status %s recovered=%t", assessment, tt.want, tt.recovered)
			}
		})
	}
}

func TestEpisodeTransitionDebouncesAndEscalates(t *testing.T) {
	low := balanceRiskAssessment{Status: types.LowBalancePeriod, Risk: true}
	first := nextEpisodeTransition(episodeTransitionInput{Assessment: low})
	if first.CreateEpisode || first.EmitAlertEvent || first.RiskCount != 1 {
		t.Fatalf("unexpected first transition: %+v", first)
	}
	second := nextEpisodeTransition(episodeTransitionInput{
		RiskCount: first.RiskCount, Assessment: low,
	})
	if !second.CreateEpisode || !second.EmitAlertEvent || second.HighestLevel != types.LowBalancePeriod {
		t.Fatalf("unexpected second transition: %+v", second)
	}
	escalated := nextEpisodeTransition(episodeTransitionInput{
		Active: true, HighestLevel: second.HighestLevel,
		Assessment: balanceRiskAssessment{
			Status: types.CriticalBalancePeriod, Risk: true, Immediate: true,
		},
	})
	if !escalated.EmitAlertEvent || escalated.HighestLevel != types.CriticalBalancePeriod {
		t.Fatalf("unexpected escalation: %+v", escalated)
	}
}

func TestEpisodeTransitionCreatesCriticalEpisodeImmediately(t *testing.T) {
	transition := nextEpisodeTransition(episodeTransitionInput{Assessment: balanceRiskAssessment{
		Status: types.CriticalBalancePeriod, Risk: true, Immediate: true,
	}})
	if !transition.CreateEpisode || !transition.EmitAlertEvent {
		t.Fatalf("unexpected transition: %+v", transition)
	}
}

func TestEpisodeTransitionRequiresTwoRecoveries(t *testing.T) {
	first := nextEpisodeTransition(episodeTransitionInput{
		Active: true, HighestLevel: types.LowBalancePeriod,
		Assessment: balanceRiskAssessment{Status: types.NormalPeriod, Recovered: true},
	})
	if first.CloseEpisode || first.RecoveryCount != 1 {
		t.Fatalf("unexpected first recovery: %+v", first)
	}
	second := nextEpisodeTransition(episodeTransitionInput{
		Active: true, HighestLevel: types.LowBalancePeriod,
		RecoveryCount: first.RecoveryCount,
		Assessment:    balanceRiskAssessment{Status: types.NormalPeriod, Recovered: true},
	})
	if !second.CloseEpisode || second.RecoveryCount != 0 {
		t.Fatalf("unexpected second recovery: %+v", second)
	}
}

func TestEpisodeTransitionDoesNotEmitLowerLevel(t *testing.T) {
	transition := nextEpisodeTransition(episodeTransitionInput{
		Active: true, HighestLevel: types.CriticalBalancePeriod,
		Assessment: balanceRiskAssessment{Status: types.LowBalancePeriod, Risk: true},
	})
	if transition.EmitAlertEvent || transition.HighestLevel != types.CriticalBalancePeriod {
		t.Fatalf("unexpected lower-level transition: %+v", transition)
	}
}
