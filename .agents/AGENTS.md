# AGENTS.md

This document helps AI agents work effectively in this repository.

## Project Overview

**Browse MCP** is a Model Context Protocol server that gives AI assistants access to the internet. It provides three core tools: search, fetch page content, and download files. The server supports two transport modes (Stdio and HTTP) with JWT-based authentication, access logs, CEL-based tool policies and URL-based web policies for production use.

## Essential Commands

```bash
# Build
make build                    # Build for current platform → bin/browse-mcp
make build-all                # Cross-compile for linux/amd64, linux/arm64, darwin/arm64, windows/amd64

# Run
make run                      # Build and run with config.yaml
./bin/browse-mcp -config path/to/config.yaml

# Development
make fmt                      # Format code (go fmt ./...)
make vet                      # Run go vet ./...
make test                     # Run tests (go test -v ./...)
make tidy                     # go mod tidy

# Clean
make clean                    # Remove bin/ directory
```

### Docker

```bash
docker build -t browse-mcp .
docker run -v $(pwd)/config.yaml:/config/config.yaml browse-mcp
```

## Key Technologies

| Component | Library | Purpose |
|-----------|---------|---------|
| Language | Go 1.24+ | Core runtime |
| MCP Library | `github.com/mark3labs/mcp-go` v0.44.0 | Model Context Protocol server |
| HTML parsing | `github.com/PuerkitoBio/goquery` | DOM manipulation |
| HTML to text | `github.com/JohannesKaufmann/html-to-markdown` | Clean HTML conversion |
| CEL policies | `github.com/google/cel-go` | Expression-based access control |
| JWT validation | `github.com/golang-jwt/jwt/v5` | Token verification |
| Configuration | `gopkg.in/yaml.v3` | YAML with env expansion |

## Code Organization

```
.
├── cmd/
│   └── main.go                       # Entrypoint — wires all components together
├── api/
│   └── config_types.go               # All config structs with inline documentation
├── internal/
│   ├── config/
│   │   └── config.go                 # YAML loader with os.ExpandEnv for variables
│   ├── globals/
│   │   └── globals.go                # ApplicationContext (Context, Logger, Config)
│   ├── handlers/
│   │   ├── handlers.go               # HandlersManager
│   │   ├── oauth_authorization_server.go   # /.well-known/oauth-authorization-server
│   │   └── oauth_protected_resource.go     # /.well-known/oauth-protected-resource
│   ├── middlewares/
│   │   ├── interfaces.go             # ToolMiddleware and HttpMiddleware interfaces
│   │   ├── logging.go                # Access logs (redact/exclude headers)
│   │   ├── jwt_validation.go         # JWT validation against JWKS + CEL allow_conditions
│   │   ├── jwt_validation_utils.go   # JWKS caching, key type conversion (RSA, EC, HMAC)
│   │   ├── tool_policy.go            # CEL-based per-tool access control
│   │   ├── web_policy.go             # CEL-based per-URL access control
│   │   ├── noop.go                   # No-op middleware for testing
│   │   └── utils.go                  # getRequestScheme helper
│   ├── web/
│   │   ├── search.go                 # Search providers: DuckDuckGo, Tavily, Serper
│   │   └── fetch.go                  # Fetch + Download + HTML cleaning
│   └── tools/
│       ├── tools.go                  # ToolsManager, tool registration, middleware wiring
│       ├── handlers.go               # web_search, web_fetch, web_download handlers
│       └── helpers.go                # getArgs, getString, getInt
├── docs/
│   ├── config-stdio.yaml             # Minimal Stdio config example
│   ├── config-http.yaml              # Full HTTP config with auth and policies
│   └── images/
│       └── header.svg                # README header
└── .github/workflows/
    └── release.yaml                  # CI/CD — binaries + Docker image on release
```

## Architecture

### Transport Modes

1. **Stdio** (default): Local use with tools like Claude Desktop or Cursor. No network exposure.
2. **HTTP**: Networked server at `/mcp` endpoint. Supports JWT auth, access logs, and policies.

### Middleware Stack (HTTP mode)

```
Request
  → AccessLogsMiddleware    (logs method, URL, duration, headers)
  → JWTValidationMiddleware (validates JWT against JWKS, stores raw token in context)
  → MCP Handler
      → ToolPolicyMiddleware  (checks JWT claims against allowed_tools per policy)
      → WebPolicyMiddleware   (checks JWT claims against allowed_domains per policy)
      → actual tool handler
```

JWT payload flows through context: `JWTValidationMiddleware` stores the raw token string under `JWTContextKey`. Tool and web policy middlewares re-parse it on each call to extract the payload map used in CEL expressions.

### Middleware Interfaces

Two interface types in `internal/middlewares/interfaces.go`:

```go
// Wraps MCP tool handlers (tool-level policies)
type ToolMiddleware interface {
    Middleware(next server.ToolHandlerFunc) server.ToolHandlerFunc
}

// Wraps HTTP handlers (request-level auth/logging)
type HttpMiddleware interface {
    Middleware(next http.Handler) http.Handler
}
```

## Available Tools

| Tool | Description | URL-restricted |
|------|-------------|----------------|
| `web_search` | Search the web. Returns title, URL and snippet per result. | No |
| `web_fetch` | Fetch a URL, strip HTML noise, return clean text. Max 5MB. | Yes |
| `web_download` | Download a file from a URL to disk. | Yes |

### Tool Parameters

**web_search**:
- `query` (required): Search query string
- `max_results` (optional): 1-20, default 10
- `provider` (optional): `duckduckgo` (default), `tavily`, `serper`

**web_fetch**:
- `url` (required): Must start with `http://` or `https://`
- `timeout` (optional): Seconds, default 30, max 120

**web_download**:
- `url` (required): Must start with `http://` or `https://`
- `file_path` (required): Local path to save file
- `timeout` (optional): Seconds, default 120, max 600

### Content Handling

- **HTML cleanup**: Strips `<script>`, `<style>`, `<nav>`, `<header>`, `<footer>`, `<aside>`, `<noscript>`, `<iframe>`, `<svg>`
- **Conversion**: HTML → Markdown first (via `html-to-markdown`), falls back to plain text extraction
- **Large content**: Content >50KB saved to temp file, path returned with first 2000 chars preview
- **Max fetch size**: 5MB (`MaxFetchSize` in `internal/web/fetch.go`)

## Security Model

### JWT Validation (`middleware.jwt`)

- Reads token from `Authorization: Bearer` header
- Validates signature against JWKS fetched from `jwks_uri` (cached, refreshed periodically)
- `allow_conditions`: CEL expressions against JWT payload, all must return true
- On failure: 401 with `WWW-Authenticate` header pointing to OAuth metadata

### Tool Policies (`policies.tools`)

- CEL expression against JWT payload → list of allowed tools
- First matching policy wins
- Supported patterns:
  - Exact match: `"web_fetch"`
  - Wildcard all: `"*"`
  - Prefix: `"web_*"` matches `web_search`, `web_fetch`, `web_download`

### Web Policies (`policies.web`)

- CEL expression against JWT payload → list of allowed domains
- Applies to `web_fetch` and `web_download` only
- `web_search` is **not** restricted (returns snippets, no content fetched)
- Domain patterns:
  - Exact: `"docs.k8s.io"`
  - Wildcard subdomains: `"*.github.com"`
  - Allow all: `"*"`

### OAuth Metadata Endpoints

- `/.well-known/oauth-authorization-server{url_suffix}` — enabled via `oauth_authorization_server.enabled`
- `/.well-known/oauth-protected-resource{url_suffix}` — enabled via `oauth_protected_resource.enabled`

## Search Providers

| Provider | API Key | Endpoint | Notes |
|----------|---------|----------|-------|
| `duckduckgo` | No | `https://html.duckduckgo.com/html/` | Scrapes HTML. May rate-limit. |
| `tavily` | Yes | `https://api.tavily.com/search` | 1,000 credits/month free. |
| `serper` | Yes | `https://google.serper.dev/search` | Paid, credit-based. |

## Configuration

Configuration is loaded from YAML with automatic environment variable expansion via `os.ExpandEnv`.

### Environment Variables

Reference env vars in config.yaml with `$VAR_NAME` or `${VAR_NAME}`:

```yaml
web:
  providers:
    tavily:
      api_key: "$TAVILY_API_KEY"
```

### Config Structure

All config types are documented in `api/config_types.go`. Key sections:

- `server`: Name, version, transport (stdio/http)
- `middleware`: Access logs, JWT validation
- `policies`: Tool and web access control
- `oauth_authorization_server`: Discovery endpoint config
- `oauth_protected_resource`: Protected resource metadata
- `web`: Default provider, provider API keys

## Adding New Functionality

### Adding a New Search Provider

1. Add config struct in `api/config_types.go`:
   ```go
   type NewProviderConfig struct {
       APIKey string `yaml:"api_key"`
   }
   ```
2. Add to `ProvidersConfig` struct
3. Add provider constant in `internal/web/search.go`:
   ```go
   const ProviderNew = "newprovider"
   ```
4. Implement search function: `searchNewProvider(ctx, client, query, maxResults, apiKey)`
5. Add case in `Search()` switch
6. Add API key field to `ToolsManagerDependencies` in `internal/tools/tools.go`
7. Pass key from `cmd/main.go`

### Adding a New Tool

1. Define tool in `internal/tools/tools.go` `AddTools()`:
   ```go
   tool := mcp.NewTool("tool_name",
       mcp.WithDescription("..."),
       mcp.WithString("param", mcp.Required(), mcp.Description("...")),
   )
   tm.dependencies.McpServer.AddTool(tool, tm.wrapWithMiddlewares(tm.HandleToolName))
   ```
2. Implement handler in `internal/tools/handlers.go`:
   ```go
   func (tm *ToolsManager) HandleToolName(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
       // ...
   }
   ```
3. Use `getArgs`, `getString`, `getInt` helpers from `internal/tools/helpers.go`

### Adding a New Middleware

**HTTP Middleware** (request-level):
1. Implement `HttpMiddleware` interface
2. Wire in `cmd/main.go` on the `/mcp` handler chain

**Tool Middleware** (tool-level):
1. Implement `ToolMiddleware` interface
2. Add to `toolMiddlewares` slice in `cmd/main.go`
3. Middlewares are applied in reverse order (last added runs first)

## Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| Empty search results | DuckDuckGo rate-limiting | Switch to Tavily or Serper |
| `web_fetch` partial content | Page >5MB | Use `web_download` instead |
| Access denied | JWT doesn't match any policy | Check policy expressions and allowed_tools/domains |
| Tool access denied | Tool not in any matching policy's allowed_tools | Add tool to policy or use wildcard |
| Domain not allowed | Domain not in any matching web policy | Add domain to allowed_domains |
| JWKS fetch error | Invalid jwks_uri or network issue | Check jwks_uri, ensure server can reach it |

## Code Conventions

### Comments

All public structs and functions must have a comment explaining what they do. Example from codebase:

```go
// JWTValidationMiddleware validates incoming JWTs against a JWKS endpoint.
//
// When enabled:
//   - Reads the token from the Authorization: Bearer header
//   - Validates signature using JWKS (fetched and cached from jwks_uri)
//   ...
type JWTValidationMiddleware struct { ... }
```

### Error Handling

- Return `mcp.NewToolResultError("message")` for tool errors (user-facing)
- Use `fmt.Errorf("context: %w", err)` for wrapping internal errors
- Log errors with `appCtx.Logger.Error("message", "key", value)`

### Naming

- Middleware types: `*Middleware` (e.g., `ToolPolicyMiddleware`)
- Middleware constructors: `New*Middleware` returning `(*T, error)`
- Handler methods: `Handle*` (e.g., `HandleToolWebSearch`)
- Dependencies structs: `*Dependencies` (e.g., `ToolsManagerDependencies`)

### Logging

Uses structured logging via `log/slog` with JSON handler to stderr:

```go
appCtx.Logger.Info("message", "key", value)
appCtx.Logger.Warn("message", "key", value)
appCtx.Logger.Error("message", "error", err.Error())
```

## Testing

```bash
make test    # Runs go test -v ./...
```

No test files currently exist in the repository.

## CI/CD

Release workflow (`.github/workflows/release.yaml`) triggers on:
- GitHub release publish
- Manual dispatch with version input

Builds:
- Binaries for linux/amd64, linux/arm64, darwin/arm64, windows/amd64
- Docker image pushed to `ghcr.io` (linux/amd64, linux/arm64)

## Guidelines

1. Release notes must always be written in **English**
2. Plain language — no corporate speak
3. Commits are authored as **Magec** (`magec@magec.dev`)
4. All public structs and functions must have a comment explaining what they do
5. Configuration types go in `api/config_types.go` with inline documentation
6. Business logic belongs in `internal/` packages, not `cmd/`
