// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	. "github.com/gardener/gardener/pkg/utils/secrets"
)

var _ = Describe("Wireguard keypair test", func() {
	Describe("Configuration", func() {
		var wgConfig *WireguardConfig

		BeforeEach(func() {
			wgConfig = &WireguardConfig{
				Name: "vpn-seed",
			}
		})

		Describe("#Generate", func() {
			It("should properly generate Wireguard key pair", func() {
				obj, err := wgConfig.Generate()
				Expect(err).NotTo(HaveOccurred())

				wgkeys, ok := obj.(*WireguardKeys)
				Expect(ok).To(BeTrue())

				privateKey := wgkeys.SecretData()[WireguardPrivateKey]
				publicKey := wgkeys.SecretData()[WireguardPublicKey]
				_, err = wgtypes.NewKey(privateKey)
				Expect(err).NotTo(HaveOccurred())

				_, err = wgtypes.NewKey(publicKey)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
