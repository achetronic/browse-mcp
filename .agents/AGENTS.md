# AGENTS.md

This document helps AI agents work effectively in this repository.

## Project Overview

**Browse MCP** is a Model Context Protocol server that gives AI assistants access to the internet. Search, fetch page content, and download files. Supports JWT-based authentication, access logs, tool policies and URL-based web policies for production use.

## Key Technologies

- **Language**: Go 1.24+
- **MCP Library**: `github.com/mark3labs/mcp-go` v0.44.0
- **HTML parsing**: `github.com/PuerkitoBio/goquery`
- **HTML to text**: `github.com/JohannesKaufmann/html-to-markdown`
- **CEL policies**: `github.com/google/cel-go`
- **JWT validation**: `github.com/golang-jwt/jwt/v5`
- **Configuration**: YAML with environment variable expansion

## Code Organization

```
.
├── cmd/
│   └── main.go                       # Entrypoint — wires all components together
├── api/
│   └── config_types.go               # All config structs with inline documentation
├── internal/
│   ├── config/
│   │   └── config.go                 # YAML loader with env expansion
│   ├── globals/
│   │   └── globals.go                # ApplicationContext
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
│   │   ├── web_policy.go             # CEL-based per-URL access control (unique to this MCP)
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
│   ├── config-stdio.yaml             # Minimal stdio config
│   ├── config-http.yaml              # Full HTTP config with auth and policies
│   └── images/
│       └── header.svg                # README header
└── .github/workflows/
    └── release.yaml                  # CI/CD — binaries + Docker image
```

## Middleware Stack (HTTP mode)

```
Request
  → AccessLogsMiddleware    (logs method, URL, duration, headers)
  → JWTValidationMiddleware (validates JWT against JWKS, stores raw token in context)
  → MCP Handler
      → ToolPolicyMiddleware  (checks JWT claims against allowed_tools per policy)
      → WebPolicyMiddleware   (checks JWT claims against allowed_domains per policy)
      → actual tool handler
```

JWT payload flows through context: JWTValidationMiddleware stores the raw token string
under `JWTContextKey`. Tool and web policy middlewares re-parse it on each call to extract
the payload map used in CEL expressions.

## Available Tools

- `web_search` — Search the web. DuckDuckGo (no key), Tavily or Serper with API key
- `web_fetch` — Fetch a URL, strip HTML noise, return clean text
- `web_download` — Download a file from a URL to disk

## Security Model

### JWT validation (`middleware.jwt`)
- Always reads from `Authorization: Bearer`
- Validates signature against JWKS (fetched and cached from `jwks_uri`)
- `allow_conditions` — CEL expressions against JWT payload, all must be true (coarse-grained: issuer, audience, etc.)

### Tool policies (`policies.tools`)
- CEL expression against JWT payload → list of allowed tools
- First matching policy wins, supports `*` and `web_*` prefixes

### Web policies (`policies.web`)
- CEL expression against JWT payload → list of allowed domains
- Applies to `web_fetch` and `web_download` only
- `web_search` is not restricted (returns snippets, no content fetched)
- Supports exact domains and wildcard subdomains (`*.github.com`)

### OAuth metadata endpoints
- `/.well-known/oauth-authorization-server` — enabled via `oauth_authorization_server.enabled`
- `/.well-known/oauth-protected-resource` — enabled via `oauth_protected_resource.enabled`

## Search Providers

- **DuckDuckGo** — scrapes DDG HTML, no key needed. May rate-limit.
- **Tavily** — POST to `https://api.tavily.com/search`. 1,000 credits/month free.
- **Serper** — POST to `https://google.serper.dev/search`. Paid, credit-based.

## Adding New Providers

1. Add config struct in `api/config_types.go`
2. Add provider constant and search function in `internal/web/search.go`
3. Add the case in the `Search()` switch
4. Add API key field in `ToolsManagerDependencies` and pass it from `cmd/main.go`

## Common Issues

- **Empty search results**: DuckDuckGo rate-limiting. Switch to Tavily.
- **web_fetch partial content**: Page >5MB. Use `web_download` instead.
- **Access denied**: JWT doesn't match any policy, or domain not in allowlist.

## Guidelines

1. Release notes must always be written in **English**
2. Plain language — no corporate speak
3. Commits are authored as **Magec** (`magec@magec.dev`)
4. All public structs and functions must have a comment explaining what they do
