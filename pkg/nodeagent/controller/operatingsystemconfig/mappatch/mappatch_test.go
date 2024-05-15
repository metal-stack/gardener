// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
package mappatch_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig/mappatch"
)

var _ = Describe("mappatch", func() {
	Describe("SetMapEntry", func() {
		It("should set the value into a existing map", func() {
			var (
				m = map[string]any{
					"a": map[string]any{
						"b": map[string]any{
							"c": 1,
						},
					},
				}
				want = map[string]any{
					"a": map[string]any{
						"b": map[string]any{
							"c": 2,
						},
					},
				}
			)

			got, err := mappatch.SetMapEntry(m, mappatch.MapPath{"a", "b", "c"}, func(value any) (any, error) { return 2, nil })
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal(want))
		})

		It("should populate a nil map", func() {
			var (
				m    map[string]any
				want = map[string]any{
					"a": map[string]any{
						"b": map[string]any{
							"c": true,
						},
					},
				}
			)

			got, err := mappatch.SetMapEntry(m, mappatch.MapPath{"a", "b", "c"}, func(value any) (any, error) { return true, nil })
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal(want))
		})

		It("should error when there are no path elements", func() {
			got, err := mappatch.SetMapEntry(nil, mappatch.MapPath{}, func(value any) (any, error) { return true, nil })
			Expect(err).To(MatchError(fmt.Errorf("at least one path element for patching is required")))
			Expect(got).To(BeNil())
		})

		It("should error when no setter function is given", func() {
			got, err := mappatch.SetMapEntry(nil, mappatch.MapPath{"a"}, nil)
			Expect(err).To(MatchError(fmt.Errorf("setter function must not be nil")))
			Expect(got).To(BeNil())
		})

		It("should error when the setter function returns an error", func() {
			got, err := mappatch.SetMapEntry(nil, mappatch.MapPath{"a", "b"}, func(value any) (any, error) { return nil, fmt.Errorf("unable to set value") })
			Expect(err).To(MatchError(fmt.Errorf("unable to set value")))
			Expect(got).To(BeNil())
		})

		It("traversing into a non-map turns into an error", func() {
			got, err := mappatch.SetMapEntry(map[string]any{"a": true}, mappatch.MapPath{"a", "b", "c"}, func(value any) (any, error) { return nil, nil })
			Expect(err).To(MatchError(fmt.Errorf(`unable to traverse into data structure because value at "a" is not a map`)))
			Expect(got).To(BeNil())
		})
	})
})
