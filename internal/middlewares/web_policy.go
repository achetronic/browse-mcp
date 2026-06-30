// SPDX-FileCopyrightText: 2026 Alby Hernández <hola@achetronic.com>
// SPDX-License-Identifier: Apache-2.0

package middlewares

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"browse-mcp/internal/globals"

	"github.com/google/cel-go/cel"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// CompiledWebPolicy holds a precompiled CEL program and its allowed domains
type CompiledWebPolicy struct {
	Program        cel.Program
	AllowedDomains []string
}

// WebPolicyMiddlewareDependencies holds the dependencies for the web policy middleware
type WebPolicyMiddlewareDependencies struct {
	AppCtx *globals.ApplicationContext
}

// WebPolicyMiddleware enforces URL access policies based on JWT claims
type WebPolicyMiddleware struct {
	dependencies     WebPolicyMiddlewareDependencies
	compiledPolicies []CompiledWebPolicy
}

// NewWebPolicyMiddleware creates a new WebPolicyMiddleware
func NewWebPolicyMiddleware(deps WebPolicyMiddlewareDependencies) (*WebPolicyMiddleware, error) {
	mw := &WebPolicyMiddleware{dependencies: deps}

	env, err := cel.NewEnv(cel.Variable("payload", cel.DynType))
	if err != nil {
		return nil, fmt.Errorf("CEL environment creation error: %s", err.Error())
	}

	for _, policy := range deps.AppCtx.Config.Policies.Web {
		ast, issues := env.Compile(policy.Expression)
		if issues != nil && issues.Err() != nil {
			return nil, fmt.Errorf("CEL web policy compilation error for '%s': %s", policy.Expression, issues.Err())
		}
		prg, err := env.Program(ast)
		if err != nil {
			return nil, fmt.Errorf("CEL program construction error: %s", err.Error())
		}
		mw.compiledPolicies = append(mw.compiledPolicies, CompiledWebPolicy{
			Program:        prg,
			AllowedDomains: policy.AllowedDomains,
		})
	}

	return mw, nil
}

// Middleware wraps a tool handler and checks if the requested URL is allowed
func (mw *WebPolicyMiddleware) Middleware(next server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if len(mw.compiledPolicies) == 0 {
			return next(ctx, request)
		}

		// Extract URL from request arguments (works for web_fetch and web_download)
		var args map[string]any
		if a, ok := request.Params.Arguments.(map[string]any); ok {
			args = a
		}

		rawURL, _ := args["url"].(string)
		query, _ := args["query"].(string)

		// web_search: no URL to check, allow it through (results may contain URLs but we don't prefetch)
		if rawURL == "" && query != "" {
			return next(ctx, request)
		}

		payload, err := extractJWTPayload(ctx)
		if err != nil {
			mw.dependencies.AppCtx.Logger.Warn("could not extract JWT payload for web policy check", "error", err.Error())
			return mcp.NewToolResultError("Access denied: unable to verify permissions"), nil
		}

		for _, policy := range mw.compiledPolicies {
			out, _, err := policy.Program.Eval(map[string]interface{}{"payload": payload})
			if err != nil {
				mw.dependencies.AppCtx.Logger.Error("CEL web policy evaluation error", "error", err.Error())
				continue
			}
			if out.Value() == true {
				if mw.isDomainAllowed(rawURL, policy.AllowedDomains) {
					return next(ctx, request)
				}
				return mcp.NewToolResultError(fmt.Sprintf("Access denied: URL '%s' is not allowed for your group", rawURL)), nil
			}
		}

		return mcp.NewToolResultError("Access denied: no web policy matched your credentials"), nil
	}
}

// isDomainAllowed checks if the URL's hostname matches any allowed domain pattern
func (mw *WebPolicyMiddleware) isDomainAllowed(rawURL string, allowedDomains []string) bool {
	for _, d := range allowedDomains {
		if d == "*" {
			return true
		}
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := parsed.Hostname()

	for _, pattern := range allowedDomains {
		if pattern == host {
			return true
		}
		// Wildcard subdomain: *.github.com matches docs.github.com
		if strings.HasPrefix(pattern, "*.") {
			suffix := pattern[1:] // ".github.com"
			if strings.HasSuffix(host, suffix) {
				return true
			}
		}
	}
	return false
}

// extractJWTPayload extracts the JWT payload from context (set by JWT HTTP middleware)
func extractJWTPayload(ctx context.Context) (map[string]interface{}, error) {
	jwtToken, ok := ctx.Value(JWTContextKey).(string)
	if !ok || jwtToken == "" {
		return nil, fmt.Errorf("no JWT token in context")
	}
	parts := strings.Split(jwtToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed JWT token")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("error decoding JWT payload: %w", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("error parsing JWT payload: %w", err)
	}
	return payload, nil
}
