// SPDX-FileCopyrightText: 2026 Alby Hernández <hola@achetronic.com>
// SPDX-License-Identifier: Apache-2.0

package middlewares

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"browse-mcp/internal/globals"

	"github.com/google/cel-go/cel"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// CompiledToolPolicy holds a precompiled CEL program and the list of tools it allows.
// The program is evaluated against the JWT payload on every tool call.
type CompiledToolPolicy struct {
	Program      cel.Program
	AllowedTools []string
}

// ToolPolicyMiddlewareDependencies holds the dependencies for the tool policy middleware.
type ToolPolicyMiddlewareDependencies struct {
	AppCtx *globals.ApplicationContext
}

// ToolPolicyMiddleware enforces which MCP tools a user can call based on their JWT claims.
// Policies are CEL expressions evaluated against the JWT payload.
// The first matching policy wins. If no policy matches, the request is denied.
// The JWT payload is extracted from the request context (set by JWTValidationMiddleware).
type ToolPolicyMiddleware struct {
	dependencies     ToolPolicyMiddlewareDependencies
	compiledPolicies []CompiledToolPolicy
	toolPrefix       string
}

func NewToolPolicyMiddleware(deps ToolPolicyMiddlewareDependencies) (*ToolPolicyMiddleware, error) {
	mw := &ToolPolicyMiddleware{
		dependencies: deps,
		toolPrefix:   deps.AppCtx.ToolPrefix,
	}

	// Create CEL environment for policy evaluation
	env, err := cel.NewEnv(
		cel.Variable("payload", cel.DynType),
	)
	if err != nil {
		return nil, fmt.Errorf("CEL environment creation error: %s", err.Error())
	}

	// Precompile all policy expressions
	for _, policy := range deps.AppCtx.Config.Policies.Tools {
		ast, issues := env.Compile(policy.Expression)
		if issues != nil && issues.Err() != nil {
			return nil, fmt.Errorf("CEL policy compilation error for expression '%s': %s", policy.Expression, issues.Err())
		}

		prg, err := env.Program(ast)
		if err != nil {
			return nil, fmt.Errorf("CEL program construction error: %s", err.Error())
		}

		mw.compiledPolicies = append(mw.compiledPolicies, CompiledToolPolicy{
			Program:      prg,
			AllowedTools: policy.AllowedTools,
		})
	}

	return mw, nil
}

// Middleware wraps a tool handler and checks if the tool is allowed based on JWT claims
func (mw *ToolPolicyMiddleware) Middleware(next server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// If no policies configured, allow all
		if len(mw.compiledPolicies) == 0 {
			return next(ctx, request)
		}

		// Extract JWT payload from context or request
		// The JWT should have been validated and stored by the HTTP middleware
		payload, err := mw.extractJWTPayloadFromContext(ctx)
		if err != nil {
			// If we can't extract JWT and policies are configured, deny by default
			mw.dependencies.AppCtx.Logger.Warn("could not extract JWT payload for policy check", "error", err.Error())
			return mcp.NewToolResultError("Access denied: unable to verify permissions"), nil
		}

		toolName := strings.TrimPrefix(request.Params.Name, mw.toolPrefix)

		// Check each policy - first matching policy wins
		for _, policy := range mw.compiledPolicies {
			out, _, err := policy.Program.Eval(map[string]interface{}{
				"payload": payload,
			})

			if err != nil {
				mw.dependencies.AppCtx.Logger.Error("CEL policy evaluation error", "error", err.Error())
				continue
			}

			// If expression matches, check if tool is allowed
			if out.Value() == true {
				if mw.isToolAllowed(toolName, policy.AllowedTools) {
					return next(ctx, request)
				}
			}
		}

		// No policy matched or tool not in allowed list
		mw.dependencies.AppCtx.Logger.Warn("tool access denied by policy",
			"tool", toolName,
		)
		return mcp.NewToolResultError(fmt.Sprintf("Access denied: you don't have permission to use '%s'", toolName)), nil
	}
}

// isToolAllowed checks if a tool is in the allowed list
func (mw *ToolPolicyMiddleware) isToolAllowed(toolName string, allowedTools []string) bool {
	for _, allowed := range allowedTools {
		if allowed == "*" {
			return true
		}
		if allowed == toolName {
			return true
		}
		// Support prefix matching with * (e.g., "get_*" matches "get_timeline")
		if strings.HasSuffix(allowed, "*") {
			prefix := strings.TrimSuffix(allowed, "*")
			if strings.HasPrefix(toolName, prefix) {
				return true
			}
		}
	}
	return false
}

// extractJWTPayloadFromContext extracts the JWT payload from the context
// This assumes the JWT was validated by the HTTP middleware and stored in context
func (mw *ToolPolicyMiddleware) extractJWTPayloadFromContext(ctx context.Context) (map[string]interface{}, error) {
	// Try to get JWT from context (set by HTTP middleware)
	jwtToken, ok := ctx.Value(JWTContextKey).(string)
	if !ok || jwtToken == "" {
		return nil, fmt.Errorf("no JWT token in context")
	}

	// Parse the payload from the JWT
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
