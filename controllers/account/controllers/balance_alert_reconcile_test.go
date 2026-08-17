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
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labring/sealos/controllers/pkg/types"
)

func TestResolveBalanceDataThroughUsesSlowestCompleteRegion(t *testing.T) {
	now := time.Date(2026, 8, 4, 10, 30, 0, 0, time.UTC)
	regionA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	regionB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	dataThrough, reason := resolveBalanceDataThrough(
		[]types.Region{{UID: regionA}, {UID: regionB}},
		[]types.BalanceBillingWatermark{
			{RegionUID: regionA, DataThrough: now.Truncate(time.Hour), ConsecutiveHours: 48},
			{RegionUID: regionB, DataThrough: now.Truncate(time.Hour).Add(-time.Hour), ConsecutiveHours: 48},
		},
		now,
	)
	want := now.Truncate(time.Hour).Add(-time.Hour)
	if reason != "" || !dataThrough.Equal(want) {
		t.Fatalf("data through = %s, reason = %q, want %s", dataThrough, reason, want)
	}
}

func TestResolveBalanceDataThroughRequiresCompleteHistory(t *testing.T) {
	now := time.Date(2026, 8, 4, 10, 30, 0, 0, time.UTC)
	region := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	_, reason := resolveBalanceDataThrough(
		[]types.Region{{UID: region}},
		[]types.BalanceBillingWatermark{{
			RegionUID: region, DataThrough: now.Truncate(time.Hour), ConsecutiveHours: 47,
		}},
		now,
	)
	if reason == "" {
		t.Fatal("expected incomplete history to lower prediction confidence")
	}
}

func TestResolveBalanceDataThroughRejectsStaleWatermark(t *testing.T) {
	now := time.Date(2026, 8, 4, 10, 30, 0, 0, time.UTC)
	region := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	_, reason := resolveBalanceDataThrough(
		[]types.Region{{UID: region}},
		[]types.BalanceBillingWatermark{{
			RegionUID: region, DataThrough: now.Add(-3 * time.Hour), ConsecutiveHours: 48,
		}},
		now,
	)
	if reason == "" {
		t.Fatal("expected a stale watermark to lower prediction confidence")
	}
}
