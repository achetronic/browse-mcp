// SPDX-FileCopyrightText: 2026 Alby Hernández <hola@achetronic.com>
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"browse-mcp/api"

	"gopkg.in/yaml.v3"
)

// ReadFile reads and parses a configuration file
func ReadFile(filepath string) (config api.Configuration, err error) {
	fileBytes, err := os.ReadFile(filepath)
	if err != nil {
		return config, err
	}

	fileExpandedEnv := os.ExpandEnv(string(fileBytes))
	err = yaml.Unmarshal([]byte(fileExpandedEnv), &config)
	return config, err
}
