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
)

func TestBalanceUsageHour(t *testing.T) {
	exactHour := time.Date(2026, 8, 4, 10, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))
	if got := balanceUsageHour(exactHour); !got.Equal(exactHour) {
		t.Fatalf("exact hour = %s, want %s", got, exactHour)
	}
	want := exactHour.Add(time.Hour)
	if got := balanceUsageHour(exactHour.Add(20 * time.Minute)); !got.Equal(want) {
		t.Fatalf("partial hour = %s, want %s", got, want)
	}
}

func TestBalanceUsageEventIDIncludesRegion(t *testing.T) {
	regionA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	regionB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	if balanceUsageEventID(regionA, "same-order") == balanceUsageEventID(regionB, "same-order") {
		t.Fatal("regional billing events must have distinct global IDs")
	}
}
