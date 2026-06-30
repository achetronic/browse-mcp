// SPDX-FileCopyrightText: 2026 Alby Hernández <hola@achetronic.com>
// SPDX-License-Identifier: Apache-2.0

package middlewares

import (
	"net/http"

	"github.com/mark3labs/mcp-go/server"
)

// ToolMiddleware is implemented by any middleware that wraps MCP tool handlers.
type ToolMiddleware interface {
	Middleware(next server.ToolHandlerFunc) server.ToolHandlerFunc
}

// HttpMiddleware is implemented by any middleware that wraps HTTP handlers.
type HttpMiddleware interface {
	Middleware(next http.Handler) http.Handler
}
