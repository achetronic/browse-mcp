<p align="center">
  <img src="docs/images/header.svg" alt="Browse MCP" width="800"/>
</p>

<p align="center">
  <em>A Model Context Protocol server that gives AI assistants access to the internet.<br/>Search, fetch pages, download files. No API key required to get started.</em>
</p>

<p align="center">
  <a href="#-what-can-it-do">What it does</a> •
  <a href="#-getting-started">Getting Started</a> •
  <a href="#-available-tools">Tools</a> •
  <a href="#-search-providers">Providers</a> •
  <a href="#-security-http-mode">Security</a> •
  <a href="#-docker">Docker</a>
</p>

---

## 🎯 What can it do?

This MCP gives your AI assistant real internet access:

- **Search** the web using DuckDuckGo (no setup), Tavily or Serper (with API key)
- **Fetch** any URL and get the content as clean readable text
- **Download** files from the web directly to disk
- **Restrict** which URLs each user can access based on their JWT claims

---

## 🚀 Getting started

### 1. Create a config file

Before running anything, create a `config.yaml`. The transport section defines how the MCP server is exposed.

**STDIO** — simplest option, no network exposure. Ideal for local use with tools like Claude Desktop or Cursor.

```yaml
server:
  name: "browse-mcp"
  version: "0.1.0"
  transport:
    type: "stdio"

web:
  default_provider: "duckduckgo"
```

**HTTP** — exposes the server over the network. Required for multi-user setups, production deployments, or when you need JWT auth and URL policies.

```yaml
server:
  name: "browse-mcp"
  version: "0.1.0"
  transport:
    type: "http"
    http:
      host: ":8080"

middleware:
  access_logs:
    redacted_headers: ["Authorization"]
  jwt:
    enabled: true
    validation:
      strategy: "local"
      local:
        jwks_uri: "https://your-idp.com/.well-known/jwks.json"
        cache_interval: 5m

web:
  default_provider: "tavily"
  providers:
    tavily:
      api_key: "$TAVILY_API_KEY"
```

See `docs/config-http.yaml` for the full example including policies.

### 2. Run it

**Binary** — lower overhead, direct access to the host filesystem (useful if you use `web_download` to save files locally).

```bash
go mod tidy
make build
./bin/browse-mcp -config config.yaml
```

**Docker** — fully isolated, no host dependencies. The downloaded files go inside the container unless you mount a volume.

```bash
docker build -t browse-mcp .
docker run \
  -v $(pwd)/config.yaml:/config/config.yaml \
  -v $(pwd)/downloads:/downloads \
  browse-mcp
```

---

## 🛠️ Available tools

| Tool | What it does |
|------|--------------|
| `web_search` | Search the web — returns title, URL and snippet per result |
| `web_fetch` | Fetch a URL and return clean readable text (HTML noise removed) |
| `web_download` | Download a file from a URL and save it to disk |

### Recommended flow

```
1. web_search  → find relevant URLs
2. web_fetch   → read the full content of the best results
3. web_download → save files you need to keep
```

---

## 🔍 Search providers

| Provider | API Key | Notes |
|----------|---------|-------|
| `duckduckgo` | No | Default. Works out of the box. May occasionally rate-limit. |
| `tavily` | Yes ([tavily.com](https://tavily.com)) | Built for AI. 1,000 credits/month free. |
| `serper` | Yes ([serper.dev](https://serper.dev)) | Scrapes Google. Paid, credit-based. |

Switch provider per-request by passing `provider` to `web_search`, or set a default in config.

---

## ⚠️ Limitations

**Fetch size** — The fetcher reads up to 5MB per request. Pages larger than 50KB are saved to a temp file instead of returned inline. Use your filesystem tools to read them.

**JavaScript** — The fetcher doesn't run JS. Pages that render entirely client-side will return little or no content. For those, consider `web_download` to save the raw HTML and inspect it manually.

**DuckDuckGo** — Works without a key but may rate-limit under heavy use. Switch to Tavily for production workloads.

**Protocols** — Only HTTP and HTTPS are supported. No FTP, no websockets.

---

## 🔐 Security (HTTP mode)

When running in HTTP mode, Browse MCP supports a full security stack:

### JWT validation

Validates incoming JWTs against a JWKS endpoint. The token is read from the `Authorization: Bearer` header by default, or from a custom header if you set `forwarded_header` (useful when a proxy like Istio or Envoy has already validated the token).

```yaml
middleware:
  jwt:
    enabled: true
    validation:
      forwarded_header: "X-Validated-JWT"  # optional, defaults to Authorization
      local:
        jwks_uri: "https://your-idp.com/.well-known/jwks.json"
        cache_interval: 5m
        allow_conditions:
          - expression: 'payload.iss == "https://your-idp.com"'
```

### Access logs

Logs every request with method, URL, duration and headers. Sensitive headers can be redacted or excluded entirely.

```yaml
middleware:
  access_logs:
    excluded_headers: ["X-Internal-Token"]
    redacted_headers: ["Authorization"]
```

### Tool policies

Control which tools each group or claim can call. Uses CEL expressions evaluated against the JWT payload.

```yaml
policies:
  tools:
    - expression: 'payload.groups.exists(g, g == "admins")'
      allowed_tools: ["*"]
    - expression: 'payload.scope.contains("web:read")'
      allowed_tools: ["web_search", "web_fetch"]
```

Supported patterns: exact match (`"web_fetch"`), wildcard (`"*"`), prefix (`"web_*"`).

### URL policies

Control which domains each group can access via `web_fetch` and `web_download`. CEL expression against JWT payload, domain allowlist with wildcard subdomain support.

```yaml
policies:
  web:
    - expression: 'payload.groups.exists(g, g == "admins")'
      allowed_domains: ["*"]

    - expression: 'payload.groups.exists(g, g == "developers")'
      allowed_domains:
        - "*.github.com"
        - "docs.k8s.io"
        - "pkg.go.dev"

    - expression: 'payload.scope.contains("web:restricted")'
      allowed_domains:
        - "internal.company.com"
        - "*.internal.company.com"
```

`web_search` is not URL-restricted by design — results are snippets, no content is fetched. Restriction applies at fetch/download time.

---

## 🐳 Docker

```bash
docker build -t browse-mcp .
docker run -v $(pwd)/config.yaml:/config/config.yaml browse-mcp
```

---

## ⚠️ Limitations

**Fetch size** — The fetcher reads up to 5MB per request. Pages larger than 50KB are saved to a temp file instead of returned inline. Use your filesystem tools to read them.

**JavaScript** — The fetcher doesn't run JS. Pages that render entirely client-side will return little or no content. For those, consider `web_download` to save the raw HTML and inspect it manually.

**DuckDuckGo** — Works without a key but may rate-limit under heavy use. Switch to Tavily for production workloads.

**Protocols** — Only HTTP and HTTPS are supported. No FTP, no websockets.

---

## 🤝 Contributing

For AI agents working on this codebase, see [AGENTS.md](.agents/AGENTS.md).

---

## 📄 License

Apache 2.0
