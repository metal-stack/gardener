// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"encoding/hex"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	// WireguardPrivateKey is the key in a secret data holding the wireguard private key.
	WireguardPrivateKey = "privateKey"
	// WireguardPublicKey is the key in a secret data holding the wireguard public key.
	WireguardPublicKey = "publicKey"
)

type WireguardConfig struct {
	Name string
}

// WireguardKeys contains the private key and the public key.
type WireguardKeys struct {
	Name string

	PrivateKey wgtypes.Key
	PublicKey  wgtypes.Key
}

// GetName returns the name of the secret.
func (s *WireguardConfig) GetName() string {
	return s.Name
}

// Generate implements ConfigInterface.
func (s *WireguardConfig) Generate() (DataInterface, error) {
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, err
	}

	keys := &WireguardKeys{
		Name: s.Name,
		PrivateKey: privateKey,
		PublicKey: privateKey.PublicKey(),
	}
	return keys, nil
}

// SecretData computes the data map which can be used in a Wireguars key pair.
func (r *WireguardKeys) SecretData() map[string][]byte {

	data := map[string][]byte{
		WireguardPrivateKey: []byte(hex.EncodeToString(r.PrivateKey[:])),
		WireguardPublicKey:  []byte(hex.EncodeToString(r.PublicKey[:])),
	}

	return data
}
