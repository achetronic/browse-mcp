// SPDX-FileCopyrightText: 2026 Alby Hernández <hola@achetronic.com>
// SPDX-License-Identifier: Apache-2.0

package tools

import "github.com/mark3labs/mcp-go/mcp"

// getArgs safely extracts the Arguments map from a CallToolRequest
func getArgs(request mcp.CallToolRequest) map[string]any {
	if args, ok := request.Params.Arguments.(map[string]any); ok {
		return args
	}
	return make(map[string]any)
}

// getString extracts a string argument with a default value
func getString(args map[string]any, key string, defaultVal string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return defaultVal
}

// getInt extracts an int argument with a default value
func getInt(args map[string]any, key string, defaultVal int) int {
	if v, ok := args[key].(float64); ok {
		return int(v)
	}
	return defaultVal
}
