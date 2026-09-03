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

package mongo

import (
	"testing"
	"time"

	"github.com/labring/sealos/controllers/pkg/resources"
)

func TestGenerateBillingDataPreservesTypedGroupKey(t *testing.T) {
	start := time.Date(2026, time.July, 29, 1, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	properties := resources.NewPropertyTypeLS([]resources.PropertyType{
		{
			Name: "cpu", Enum: 0, PriceType: resources.AVG, UnitPrice: 1,
		},
	})
	records := []resources.Monitor{
		{
			Time: start, Category: "ns-owner", Type: 1,
			ParentType: 255, ParentName: "parent/name", Name: "child",
			Used: resources.EnumUsedMap{0: 60},
		},
	}

	billings, err := GenerateBillingDataFromRecords(
		records, properties, start, end, "owner",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(billings) != 1 {
		t.Fatalf("billing count = %d, want 1", len(billings))
	}
	if billings[0].AppType != 255 || billings[0].AppName != "parent/name" {
		t.Fatalf(
			"billing group = (%d, %q), want (255, %q)",
			billings[0].AppType,
			billings[0].AppName,
			"parent/name",
		)
	}
}
