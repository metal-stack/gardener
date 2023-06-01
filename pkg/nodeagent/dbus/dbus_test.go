// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dbus

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_db_Enable(t *testing.T) {
	tests := []struct {
		name          string
		d             Dbus
		unitNames     []string
		wantErr       bool
		wantedActions []FakeSystemdAction
	}{
		{
			name:      "enable a unit",
			unitNames: []string{"kubelet"},
			wantedActions: []FakeSystemdAction{
				{
					Action:    FakeEnable,
					UnitNames: []string{"kubelet"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &FakeDbus{}
			if err := d.Enable(context.Background(), tt.unitNames...); (err != nil) != tt.wantErr {
				t.Errorf("db.Enable() error = %v, wantErr %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(d.Actions, tt.wantedActions); diff != "" {
				t.Errorf("d.Enable did not call the enable action, diff was %q", diff)
			}
		})
	}
}
