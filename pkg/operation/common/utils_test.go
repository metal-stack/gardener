// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package common_test

import (
	"fmt"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/operation/common"
)

var _ = Describe("common", func() {
	Describe("utils", func() {
		Describe("#ComputeOffsetIP", func() {
			Context("IPv4", func() {
				It("should return a cluster IPv4 IP", func() {
					_, subnet, _ := net.ParseCIDR("100.64.0.0/13")
					result, err := ComputeOffsetIP(subnet, 10)

					Expect(err).NotTo(HaveOccurred())

					Expect(result).To(HaveLen(net.IPv4len))
					Expect(result).To(Equal(net.ParseIP("100.64.0.10").To4()))
				})

				It("should return error if subnet nil is passed", func() {
					result, err := ComputeOffsetIP(nil, 10)

					Expect(err).To(HaveOccurred())
					Expect(result).To(BeNil())
				})

				It("should return error if subnet is not big enough is passed", func() {
					_, subnet, _ := net.ParseCIDR("100.64.0.0/32")
					result, err := ComputeOffsetIP(subnet, 10)

					Expect(err).To(HaveOccurred())
					Expect(result).To(BeNil())
				})

				It("should return error if ip address is broadcast ip", func() {
					_, subnet, _ := net.ParseCIDR("10.0.0.0/24")
					result, err := ComputeOffsetIP(subnet, 255)

					Expect(err).To(HaveOccurred())
					Expect(result).To(BeNil())
				})
			})

			Context("IPv6", func() {
				It("should return a cluster IPv6 IP", func() {
					_, subnet, _ := net.ParseCIDR("fc00::/8")
					result, err := ComputeOffsetIP(subnet, 10)

					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(HaveLen(net.IPv6len))
					Expect(result).To(Equal(net.ParseIP("fc00::a")))
				})

				It("should return error if subnet nil is passed", func() {
					result, err := ComputeOffsetIP(nil, 10)

					Expect(err).To(HaveOccurred())
					Expect(result).To(BeNil())
				})

				It("should return error if subnet is not big enough is passed", func() {
					_, subnet, _ := net.ParseCIDR("fc00::/128")
					result, err := ComputeOffsetIP(subnet, 10)

					Expect(err).To(HaveOccurred())
					Expect(result).To(BeNil())
				})
			})
		})

		Describe("#GenerateAddonConfig", func() {
			Context("values=nil and enabled=false", func() {
				It("should return a map with key enabled=false", func() {
					var (
						values  map[string]interface{}
						enabled = false
					)

					result := GenerateAddonConfig(values, enabled)

					Expect(result).To(SatisfyAll(
						HaveKeyWithValue("enabled", enabled),
						HaveLen(1),
					))
				})
			})

			Context("values=nil and enabled=true", func() {
				It("should return a map with key enabled=true", func() {
					var (
						values  map[string]interface{}
						enabled = true
					)

					result := GenerateAddonConfig(values, enabled)

					Expect(result).To(SatisfyAll(
						HaveKeyWithValue("enabled", enabled),
						HaveLen(1),
					))
				})
			})

			Context("values=<empty map> and enabled=true", func() {
				It("should return a map with key enabled=true", func() {
					var (
						values  = map[string]interface{}{}
						enabled = true
					)

					result := GenerateAddonConfig(values, enabled)

					Expect(result).To(SatisfyAll(
						HaveKeyWithValue("enabled", enabled),
						HaveLen(1),
					))
				})
			})

			Context("values=<non-empty map> and enabled=true", func() {
				It("should return a map with the values and key enabled=true", func() {
					var (
						values = map[string]interface{}{
							"foo": "bar",
						}
						enabled = true
					)

					result := GenerateAddonConfig(values, enabled)

					for key := range values {
						_, ok := result[key]
						Expect(ok).To(BeTrue())
					}
					Expect(result).To(SatisfyAll(
						HaveKeyWithValue("enabled", enabled),
						HaveLen(1+len(values)),
					))
				})
			})

			Context("values=<non-empty map> and enabled=false", func() {
				It("should return a map with key enabled=false", func() {
					var (
						values = map[string]interface{}{
							"foo": "bar",
						}
						enabled = false
					)

					result := GenerateAddonConfig(values, enabled)

					Expect(result).To(SatisfyAll(
						HaveKeyWithValue("enabled", enabled),
						HaveLen(1),
					))
				})
			})
		})
	})

	Describe("#FilterEntriesByPrefix", func() {
		var (
			prefix  string
			entries []string
		)

		BeforeEach(func() {
			prefix = "role"
			entries = []string{
				"foo",
				"bar",
			}
		})

		It("should only return entries with prefix", func() {
			expectedEntries := []string{
				fmt.Sprintf("%s-%s", prefix, "foo"),
				fmt.Sprintf("%s-%s", prefix, "bar"),
			}

			entries = append(entries, expectedEntries...)

			result := FilterEntriesByPrefix(prefix, entries)
			Expect(result).To(ContainElements(expectedEntries))
		})

		It("should return all entries", func() {
			expectedEntries := []string{
				fmt.Sprintf("%s-%s", prefix, "foo"),
				fmt.Sprintf("%s-%s", prefix, "bar"),
			}

			entries = expectedEntries

			result := FilterEntriesByPrefix(prefix, entries)
			Expect(result).To(ContainElements(expectedEntries))
		})

		It("should return no entries", func() {
			result := FilterEntriesByPrefix(prefix, entries)
			Expect(result).To(BeEmpty())
		})
	})
})
