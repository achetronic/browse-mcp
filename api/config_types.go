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

package api

import "time"

// ServerTransportHTTPConfig holds the bind address for the HTTP transport.
type ServerTransportHTTPConfig struct {
	Host string `yaml:"host"`
}

// ServerTransportConfig defines how the MCP server is exposed.
// Type can be "stdio" (local, no network) or "http" (networked, supports auth).
type ServerTransportConfig struct {
	Type string                    `yaml:"type"`
	HTTP ServerTransportHTTPConfig `yaml:"http,omitempty"`
}

// ServerConfig holds the MCP server identity and transport configuration.
type ServerConfig struct {
	Name      string                `yaml:"name"`
	Version   string                `yaml:"version"`
	Transport ServerTransportConfig `yaml:"transport,omitempty"`
}

// AccessLogsConfig controls request logging behaviour.
// ExcludedHeaders are removed from logs entirely.
// RedactedHeaders are truncated to 10 characters + "***" to avoid leaking secrets.
type AccessLogsConfig struct {
	ExcludedHeaders []string `yaml:"excluded_headers"`
	RedactedHeaders []string `yaml:"redacted_headers"`
}

// JWTAllowCondition is a CEL expression evaluated against the JWT payload.
// All conditions must return true for the request to pass.
// Example: 'payload.iss == "https://my-idp.com"'
type JWTAllowCondition struct {
	Expression string `yaml:"expression"`
}

// JWTConfig enables JWT validation for the HTTP transport.
// Tokens are always read from the Authorization: Bearer header and validated
// against the JWKS fetched from jwks_uri.
//
// allow_conditions are coarse-grained CEL checks (e.g. verify issuer/audience).
// Fine-grained per-tool and per-URL restrictions live under policies.
type JWTConfig struct {
	Enabled         bool               `yaml:"enabled"`
	JWKSUri         string             `yaml:"jwks_uri,omitempty"`
	CacheInterval   time.Duration      `yaml:"cache_interval,omitempty"`
	AllowConditions []JWTAllowCondition `yaml:"allow_conditions,omitempty"`
}

// MiddlewareConfig groups all HTTP middleware configuration.
type MiddlewareConfig struct {
	AccessLogs AccessLogsConfig `yaml:"access_logs"`
	JWT        JWTConfig        `yaml:"jwt,omitempty"`
}

// OAuthAuthorizationServer configures the /.well-known/oauth-authorization-server endpoint.
// Required when exposing the MCP server publicly so clients can discover the auth server.
type OAuthAuthorizationServer struct {
	Enabled   bool   `yaml:"enabled"`
	UrlSuffix string `yaml:"url_suffix,omitempty"`
	IssuerUri string `yaml:"issuer_uri"`
}

// OAuthProtectedResourceConfig configures the /.well-known/oauth-protected-resource endpoint.
// Required when exposing the MCP server publicly so clients can discover the resource metadata
// and know which scopes and auth servers are accepted.
type OAuthProtectedResourceConfig struct {
	Enabled   bool   `yaml:"enabled"`
	UrlSuffix string `yaml:"url_suffix,omitempty"`

	Resource                              string   `yaml:"resource"`
	AuthServers                           []string `yaml:"auth_servers"`
	JWKSUri                               string   `yaml:"jwks_uri"`
	ScopesSupported                       []string `yaml:"scopes_supported"`
	BearerMethodsSupported                []string `yaml:"bearer_methods_supported,omitempty"`
	ResourceSigningAlgValuesSupported     []string `yaml:"resource_signing_alg_values_supported,omitempty"`
	ResourceName                          string   `yaml:"resource_name,omitempty"`
	ResourceDocumentation                 string   `yaml:"resource_documentation,omitempty"`
	ResourcePolicyUri                     string   `yaml:"resource_policy_uri,omitempty"`
	ResourceTosUri                        string   `yaml:"resource_tos_uri,omitempty"`
	TLSClientCertificateBoundAccessTokens bool     `yaml:"tls_client_certificate_bound_access_tokens,omitempty"`
	AuthorizationDetailsTypesSupported    []string `yaml:"authorization_details_types_supported,omitempty"`
	DPoPSigningAlgValuesSupported         []string `yaml:"dpop_signing_alg_values_supported,omitempty"`
	DPoPBoundAccessTokensRequired         bool     `yaml:"dpop_bound_access_tokens_required,omitempty"`
}

// ToolPolicyConfig restricts which MCP tools a user can call based on their JWT claims.
// Expression is a CEL expression evaluated against the JWT payload.
// AllowedTools supports exact names ("web_fetch"), wildcards ("*"), and prefixes ("web_*").
type ToolPolicyConfig struct {
	Expression   string   `yaml:"expression"`
	AllowedTools []string `yaml:"allowed_tools"`
}

// WebPolicyConfig restricts which domains a user can access via web_fetch and web_download.
// Expression is a CEL expression evaluated against the JWT payload.
// AllowedDomains supports exact hostnames ("docs.k8s.io") and wildcard subdomains ("*.github.com").
// Use ["*"] to allow all domains.
// Note: web_search is not restricted — it returns snippets only, no content is fetched.
type WebPolicyConfig struct {
	Expression     string   `yaml:"expression"`
	AllowedDomains []string `yaml:"allowed_domains"`
}

// PoliciesConfig groups tool and web access policies.
// Both use CEL expressions evaluated against the JWT payload.
// The first matching policy wins.
type PoliciesConfig struct {
	Tools []ToolPolicyConfig `yaml:"tools"`
	Web   []WebPolicyConfig  `yaml:"web"`
}

// WebConfig holds search provider configuration.
type WebConfig struct {
	// DefaultProvider sets which provider is used when none is specified in the request.
	// Options: duckduckgo (no key), tavily, serper.
	DefaultProvider string          `yaml:"default_provider,omitempty"`
	Providers       ProvidersConfig `yaml:"providers,omitempty"`
}

// ProvidersConfig holds API keys for each search provider.
type ProvidersConfig struct {
	Tavily TavilyConfig `yaml:"tavily,omitempty"`
	Serper SerperConfig `yaml:"serper,omitempty"`
}

// TavilyConfig holds the Tavily API key.
type TavilyConfig struct {
	APIKey string `yaml:"api_key"`
}

// SerperConfig holds the Serper API key.
type SerperConfig struct {
	APIKey string `yaml:"api_key"`
}

// Configuration is the top-level config structure loaded from config.yaml.
type Configuration struct {
	Server                   ServerConfig                 `yaml:"server,omitempty"`
	Middleware               MiddlewareConfig             `yaml:"middleware,omitempty"`
	Policies                 PoliciesConfig               `yaml:"policies,omitempty"`
	OAuthAuthorizationServer OAuthAuthorizationServer     `yaml:"oauth_authorization_server,omitempty"`
	OAuthProtectedResource   OAuthProtectedResourceConfig `yaml:"oauth_protected_resource,omitempty"`
	Web                      WebConfig                    `yaml:"web"`
}
