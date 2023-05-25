// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package common

import (
	"reflect"
	"testing"

	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"k8s.io/utils/pointer"
)

func Test_calculateDiff(t *testing.T) {
	tests := []struct {
		name     string
		current  *v1alpha1.OperatingSystemConfig
		previous *v1alpha1.OperatingSystemConfig
		want     *OSCChanges
	}{
		{
			name: "one deleted file and unit",
			current: &v1alpha1.OperatingSystemConfig{
				Spec: v1alpha1.OperatingSystemConfigSpec{
					Units: []v1alpha1.Unit{
						{
							Name: "bla",
						},
					},
					Files: []v1alpha1.File{
						{
							Path: "/tmp/bla",
						},
					},
				},
			},
			previous: &v1alpha1.OperatingSystemConfig{
				Spec: v1alpha1.OperatingSystemConfigSpec{
					Units: []v1alpha1.Unit{
						{
							Name: "bla",
						},
						{
							Name: "blub",
						},
					},
					Files: []v1alpha1.File{
						{
							Path: "/tmp/bla",
						},
						{
							Path: "/tmp/blub",
						},
					},
				},
			},
			want: &OSCChanges{
				DeletedUnits: []v1alpha1.Unit{
					{
						Name: "blub",
					},
				},
				DeletedFiles: []v1alpha1.File{
					{
						Path: "/tmp/blub",
					},
				},
			},
		},
		{
			name: "one changed unit",
			current: &v1alpha1.OperatingSystemConfig{
				Spec: v1alpha1.OperatingSystemConfigSpec{
					Units: []v1alpha1.Unit{
						{
							Name:    "bla",
							Content: pointer.String("a unit content"),
						},
						{
							Name:    "blub",
							Content: pointer.String("changed unit content"),
						},
					},
				},
			},
			previous: &v1alpha1.OperatingSystemConfig{
				Spec: v1alpha1.OperatingSystemConfigSpec{
					Units: []v1alpha1.Unit{
						{
							Name:    "bla",
							Content: pointer.String("a unit content"),
						},
						{
							Name:    "blub",
							Content: pointer.String("b unit content"),
						},
					},
				},
			},
			want: &OSCChanges{
				ChangedUnits: []v1alpha1.Unit{
					{
						Name:    "blub",
						Content: pointer.String("changed unit content"),
					},
				},
			},
		},
		{
			name: "one changed unit, one added unit, one deleted unit",
			current: &v1alpha1.OperatingSystemConfig{
				Spec: v1alpha1.OperatingSystemConfigSpec{
					Units: []v1alpha1.Unit{
						{
							Name:    "blub",
							Content: pointer.String("changed unit content"),
						},
						{
							Name:    "chacka",
							Content: pointer.String("added unit content"),
						},
					},
				},
			},
			previous: &v1alpha1.OperatingSystemConfig{
				Spec: v1alpha1.OperatingSystemConfigSpec{
					Units: []v1alpha1.Unit{
						{
							Name:    "bla",
							Content: pointer.String("a unit content"),
						},
						{
							Name:    "blub",
							Content: pointer.String("b unit content"),
						},
					},
				},
			},
			want: &OSCChanges{
				ChangedUnits: []v1alpha1.Unit{
					{
						Name:    "blub",
						Content: pointer.String("changed unit content"),
					},
					{
						Name:    "chacka",
						Content: pointer.String("added unit content"),
					},
				},
				DeletedUnits: []v1alpha1.Unit{
					{
						Name:    "bla",
						Content: pointer.String("a unit content"),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateOSCChanges(tt.current, tt.previous); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("calculateDiff() = %v, want %v", got, tt.want)
			}
		})
	}
}
