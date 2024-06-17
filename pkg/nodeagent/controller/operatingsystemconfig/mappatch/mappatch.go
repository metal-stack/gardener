// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
package mappatch

import "fmt"

type (
	// MapPath is the path to a TOML config element
	MapPath []string
	// SetFn is the function to set a config element
	SetFn func(value any) (any, error)
)

// SetMapEntry sets an entry in the patch map to a setter function.
func SetMapEntry(m map[string]any, path MapPath, setFn SetFn) (map[string]any, error) {
	if setFn == nil {
		return nil, fmt.Errorf("setter function must not be nil")
	}
	if len(path) == 0 {
		return nil, fmt.Errorf("at least one path element for patching is required")
	}

	return setMapEntry(m, path, setFn)
}

func setMapEntry(m map[string]any, path MapPath, setFn SetFn) (map[string]any, error) {
	if m == nil {
		m = map[string]any{}
	}

	var (
		key = path[0]
	)

	if len(path) == 1 {
		value := m[key]

		var err error
		m[key], err = setFn(value)
		if err != nil {
			return nil, fmt.Errorf("error setting value: %w", err)
		}

		return m, nil
	}

	entry, ok := m[key]
	if !ok {
		entry = map[string]any{}
	}

	childMap, ok := entry.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unable to traverse into data structure because value at %q is not a map", key)
	}

	var err error
	m[key], err = setMapEntry(childMap, path[1:], setFn)
	if err != nil {
		return nil, err
	}

	return m, nil
}
