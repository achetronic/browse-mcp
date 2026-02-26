// Copyright 2024 Alby Hernández
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package middlewares

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"browse-mcp/internal/globals"

	"github.com/google/cel-go/cel"
)

// JWTContextKey is the context key used to pass the raw JWT token string
// to downstream tool middlewares (tool_policy, web_policy).
type contextKey string

const JWTContextKey contextKey = "jwt_token"

// JWTValidationMiddlewareDependencies holds the dependencies for this middleware.
type JWTValidationMiddlewareDependencies struct {
	AppCtx *globals.ApplicationContext
}

// JWTValidationMiddleware validates incoming JWTs against a JWKS endpoint.
//
// When enabled:
//   - Reads the token from the Authorization: Bearer header
//   - Validates signature using JWKS (fetched and cached from jwks_uri)
//   - Evaluates optional CEL allow_conditions against the JWT payload
//   - Stores the raw token string in the request context under JWTContextKey
//     so that tool middlewares (tool_policy, web_policy) can access the payload
//
// If validation fails at any step, the request is rejected with 401.
// If jwt.enabled is false, the middleware is a no-op.
type JWTValidationMiddleware struct {
	dependencies JWTValidationMiddlewareDependencies

	// jwks is the cached set of public keys used to verify JWT signatures.
	// It is refreshed periodically by the cacheJWKS goroutine.
	jwks  *JWKS
	mutex sync.Mutex

	// celPrograms holds precompiled CEL programs for allow_conditions.
	// All conditions must evaluate to true for the request to be allowed.
	celPrograms []*cel.Program
}

// NewJWTValidationMiddleware creates the middleware and precompiles any CEL expressions
// defined in allow_conditions. Starts the JWKS cache goroutine if JWT is enabled.
func NewJWTValidationMiddleware(deps JWTValidationMiddlewareDependencies) (*JWTValidationMiddleware, error) {
	mw := &JWTValidationMiddleware{dependencies: deps}

	// Start JWKS cache goroutine only if JWT validation is enabled
	if mw.dependencies.AppCtx.Config.Middleware.JWT.Enabled {
		go mw.cacheJWKS()
	}

	// Precompile CEL allow_conditions at startup to catch syntax errors early
	// and avoid per-request compilation overhead.
	allowConditionsEnv, err := cel.NewEnv(cel.Variable("payload", cel.DynType))
	if err != nil {
		return nil, fmt.Errorf("CEL environment creation error: %s", err.Error())
	}

	for _, allowCondition := range mw.dependencies.AppCtx.Config.Middleware.JWT.AllowConditions {
		ast, issues := allowConditionsEnv.Compile(allowCondition.Expression)
		if issues != nil && issues.Err() != nil {
			return nil, fmt.Errorf("CEL expression compilation error for '%s': %s", allowCondition.Expression, issues.Err())
		}
		prg, err := allowConditionsEnv.Program(ast)
		if err != nil {
			return nil, fmt.Errorf("CEL program construction error: %s", err.Error())
		}
		mw.celPrograms = append(mw.celPrograms, &prg)
	}

	return mw, nil
}

// Middleware returns an http.Handler that validates the JWT on every request.
// The token is read from the Authorization: Bearer header.
//
// On success, the raw token string is stored in the request context under
// JWTContextKey so that tool-level middlewares can extract the payload for
// CEL policy evaluation without re-fetching the JWKS.
func (mw *JWTValidationMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {

		// If JWT validation is disabled, pass through immediately
		if !mw.dependencies.AppCtx.Config.Middleware.JWT.Enabled {
			next.ServeHTTP(rw, req)
			return
		}

		// Set WWW-Authenticate header pointing to our protected resource metadata.
		// This is required by the MCP OAuth spec and will be cleared on success.
		// Ref: https://modelcontextprotocol.io/specification/draft/basic/authorization
		wwwAuthURL := fmt.Sprintf("%s://%s/.well-known/oauth-protected-resource%s",
			getRequestScheme(req), req.Host, mw.dependencies.AppCtx.Config.OAuthProtectedResource.UrlSuffix)
		wwwAuthScope := strings.Join(mw.dependencies.AppCtx.Config.OAuthProtectedResource.ScopesSupported, " ")
		rw.Header().Set("WWW-Authenticate",
			`Bearer error="invalid_token", resource_metadata="`+wwwAuthURL+`", scope="`+wwwAuthScope+`"`)

		// Extract the Bearer token from the Authorization header
		authHeader := req.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(rw, "RBAC: Access Denied: Authorization header not found", http.StatusUnauthorized)
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// Validate the token signature against the cached JWKS
		if _, err := mw.isTokenValid(tokenString); err != nil {
			http.Error(rw, fmt.Sprintf("RBAC: Access Denied: Invalid token: %v", err.Error()), http.StatusUnauthorized)
			return
		}

		// Decode the JWT payload (middle segment) for CEL condition evaluation
		parts := strings.Split(tokenString, ".")
		if len(parts) != 3 {
			http.Error(rw, "RBAC: Access Denied: Malformed token", http.StatusUnauthorized)
			return
		}
		payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			mw.dependencies.AppCtx.Logger.Error("error decoding JWT payload from base64", "error", err.Error())
			http.Error(rw, "RBAC: Access Denied: JWT payload cannot be decoded", http.StatusUnauthorized)
			return
		}
		tokenPayload := map[string]any{}
		if err := json.Unmarshal(payloadBytes, &tokenPayload); err != nil {
			mw.dependencies.AppCtx.Logger.Error("error parsing JWT payload JSON", "error", err.Error())
			http.Error(rw, "RBAC: Access Denied: Internal issue", http.StatusUnauthorized)
			return
		}

		// Evaluate allow_conditions — all must return true.
		// This is a coarse-grained check (e.g. correct issuer, audience).
		// Fine-grained per-tool and per-URL checks happen in tool middlewares.
		for _, celProgram := range mw.celPrograms {
			out, _, err := (*celProgram).Eval(map[string]interface{}{"payload": tokenPayload})
			if err != nil {
				mw.dependencies.AppCtx.Logger.Error("CEL allow_condition evaluation error", "error", err.Error())
				http.Error(rw, "RBAC: Access Denied: Internal issue", http.StatusUnauthorized)
				return
			}
			if out.Value() != true {
				http.Error(rw, "RBAC: Access Denied: JWT does not meet conditions", http.StatusUnauthorized)
				return
			}
		}

		// Token is valid — clear the WWW-Authenticate header and store the
		// raw token string in the context for downstream tool middlewares.
		rw.Header().Del("WWW-Authenticate")
		ctx := context.WithValue(req.Context(), JWTContextKey, tokenString)
		next.ServeHTTP(rw, req.WithContext(ctx))
	})
}
