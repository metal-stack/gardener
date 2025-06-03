// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package charts

import (
	"embed"
	"path/filepath"
)

var (
	// ChartShootSystemComponents is the Helm chart for the shoot-system-components chart.
	//go:embed shoot-system-components
	ChartShootSystemComponents embed.FS
	// ChartPathShootSystemComponents is the path to the shoot-system-components chart.
	ChartPathShootSystemComponents = filepath.Join("shoot-system-components")
)
