// Copyright © 2023 sealos.
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
)

func Test_splitSmsCodeMap(t *testing.T) {
	codeMap, err := splitSmsCodeMap("0:SMS_123456,1:SMS_654321,2:SMS_987654")
	if err != nil {
		t.Fatal(err)
	}
	if len(codeMap) != 3 {
		t.Fatal("invalid codeMap")
	}
	if codeMap["0"] != "SMS_123456" {
		t.Fatal("invalid codeMap")
	}
	if codeMap["1"] != "SMS_654321" {
		t.Fatal("invalid codeMap")
	}
	if codeMap["2"] != "SMS_987654" {
		t.Fatal("invalid codeMap")
	}
}

func TestGetTimeInUTCPlus8(t *testing.T) {
	times := []time.Time{
		time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2023, time.January, 1, 1, 0, 0, 0, time.UTC),
		time.Date(2023, time.January, 1, 9, 0, 0, 0, time.UTC),
		time.Date(2023, time.January, 1, 11, 0, 0, 0, time.UTC),
		time.Date(2023, time.January, 1, 12, 0, 0, 0, time.UTC),
		time.Date(2023, time.January, 1, 13, 0, 0, 0, time.UTC),
		time.Date(2023, time.January, 1, 23, 0, 0, 0, time.UTC),
	}
	for _, testTime := range times {
		t.Logf("time: %v, timeInUTCPlus8: %v", testTime, GetSendVmsTimeInUTCPlus8(testTime))
	}
}
