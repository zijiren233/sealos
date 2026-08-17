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
	"strings"
	"testing"
	"time"

	"github.com/labring/sealos/controllers/pkg/types"
)

func TestBalanceAlertRetryDelayIsCapped(t *testing.T) {
	if got := balanceAlertRetryDelay(1); got != time.Minute {
		t.Fatalf("first delay = %s, want 1m", got)
	}
	if got := balanceAlertRetryDelay(20); got != 6*time.Hour {
		t.Fatalf("capped delay = %s, want 6h", got)
	}
}

func TestRenderBalanceAlertEmailIncludesForecast(t *testing.T) {
	eta := int64((42 * time.Hour).Seconds())
	subject, body, err := renderBalanceAlertEmail(balanceAlertPayload{
		UserName: "tester", AvailableBalance: 18_600_000, ETASeconds: &eta,
		ForecastRate: 425_000, ShortHourlyRate: 500_000, LongHourlyRate: 400_000,
		TopWorkspace: "workspace-a", TopApplication: "app-a", Language: "zh",
		Domain: "example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"预计可用时间", "1天18小时", "workspace-a / app-a", "system-costcenter"} {
		if !strings.Contains(subject+body, expected) {
			t.Fatalf("rendered email is missing %q", expected)
		}
	}
}

func TestRenderBalanceAlertEmailExplainsLowConfidenceFallback(t *testing.T) {
	_, body, err := renderBalanceAlertEmail(balanceAlertPayload{
		AvailableBalance: 5_000_000,
		Language:         "zh",
		Confidence:       types.BalancePredictionConfidenceLow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "预测数据正在积累") {
		t.Fatalf("low-confidence email is missing fallback explanation: %s", body)
	}
}
