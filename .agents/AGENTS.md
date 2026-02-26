# AGENTS.md

This document helps AI agents work effectively in this repository.

## Project Overview

**Browse MCP** is a Model Context Protocol server that gives AI assistants access to the internet. Search, fetch page content, and download files — all from a single MCP server. Built in Go, no API key required for basic usage.

## Key Technologies

- **Language**: Go 1.24+
- **MCP Library**: `github.com/mark3labs/mcp-go` v0.44.0
- **HTML parsing**: `github.com/PuerkitoBio/goquery`
- **HTML to text**: `github.com/JohannesKaufmann/html-to-markdown`
- **Configuration**: YAML with environment variable expansion

## Code Organization

```
.
├── cmd/
│   └── main.go                  # Entrypoint
├── api/
│   └── config_types.go          # Configuration types (no Brave — requires credit card)
├── internal/
│   ├── config/
│   │   └── config.go            # YAML config loader
│   ├── globals/
│   │   └── globals.go           # ApplicationContext
│   ├── web/
│   │   ├── search.go            # Search providers: DuckDuckGo, Tavily, Serper
│   │   └── fetch.go             # Fetch + Download + HTML cleaning
│   └── tools/
│       ├── tools.go             # ToolsManager + tool registration
│       ├── handlers.go          # Tool handler implementations
│       └── helpers.go           # getArgs, getString, getInt
├── docs/
│   ├── config-stdio.yaml        # Stdio config example
│   ├── config-http.yaml         # HTTP config example
│   └── images/
│       └── header.svg           # README header
└── .github/workflows/
    └── release.yaml             # CI/CD release pipeline
```

## Available Tools

- `web_search` — Search the web. DuckDuckGo by default (no key), Tavily or Serper with API key
- `web_fetch` — Fetch a URL and return clean text. HTML noise removed automatically
- `web_download` — Download a file from a URL to disk

## Recommended Flow

```
1. web_search(query: "...") → get list of URLs with snippets
2. web_fetch(url: "...") → read full content of the most relevant URL
3. web_download(url: "...", file_path: "...") → save files to disk if needed
```

## Search Providers

Three providers implemented and verified:

- **DuckDuckGo** — default, no key needed, scrapes DDG HTML. May occasionally rate-limit.
- **Tavily** — POST to `https://api.tavily.com/search`. Free tier: 1,000 credits/month. Best for AI use cases.
- **Serper** — POST to `https://google.serper.dev/search`. Starter tier: $50 for 50k credits (~$1/1k queries), valid 6 months. Scrapes Google.

Brave was removed — free tier requires credit card.

Config example:

```yaml
web:
  default_provider: "tavily"
  providers:
    tavily:
      api_key: "$TAVILY_API_KEY"
    serper:
      api_key: "$SERPER_API_KEY"
```

## HTML Cleaning

web_fetch strips scripts, styles, nav, header, footer, iframes and SVGs before converting to text. For large pages (>50KB) content is saved to a temp file and the path is returned.

## Adding New Providers

1. Add provider constant and config struct in `api/config_types.go`
2. Add the search function in `internal/web/search.go`
3. Add the case in the `Search()` switch statement
4. Add the API key field in `ToolsManagerDependencies` in `internal/tools/tools.go`
5. Pass the key from config in `cmd/main.go`

## Common Issues

- **DuckDuckGo returns empty results**: DDG is rate-limiting. Switch to Tavily.
- **web_fetch returns partial content**: Page is larger than 5MB. Use web_download instead.
- **Provider API key not found**: Check env var is set and config uses `$VAR_NAME` syntax.

## Guidelines

1. Release notes must always be written in **English**
2. Plain language — no corporate speak
3. Commits are authored as **Magec** (`magec@magec.dev`)
