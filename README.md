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
  <a href="#-docker">Docker</a>
</p>

---

## 🎯 What can it do?

This MCP gives your AI assistant real internet access:

- **Search** the web using DuckDuckGo (no setup), Tavily or Serper (with API key)
- **Fetch** any URL and get the content as clean readable text, with HTML noise stripped automatically
- **Download** files from the web directly to disk

---

## 🚀 Getting started

### 1. Configure

Create a `config.yaml`:

```yaml
server:
  name: "browse-mcp"
  version: "0.1.0"
  transport:
    type: "stdio"

web:
  default_provider: "duckduckgo"
```

That's it. No API key needed for basic usage. DuckDuckGo works out of the box.

To use a better provider, add your API key:

```yaml
web:
  default_provider: "tavily"
  providers:
    tavily:
      api_key: "$TAVILY_API_KEY"
```

See `docs/config-stdio.yaml` and `docs/config-http.yaml` for full examples.

### 2. Build and run

```bash
go mod tidy
make build
./bin/browse-mcp -config config.yaml
```

---

## 🛠️ Available tools

| Tool | What it does |
|------|--------------|
| `web_search` | Search the web and get a list of results with title, URL and snippet |
| `web_fetch` | Fetch a URL and return its content as clean text |
| `web_download` | Download a file from a URL and save it to disk |

### Recommended flow

```
1. web_search  → find relevant URLs
2. web_fetch   → read the full content of the best results
3. web_download → save files you need to keep
```

---

## 🔍 Search providers

| Provider | API Key needed | Notes |
|----------|---------------|-------|
| `duckduckgo` | No | Default. Works out of the box. May occasionally rate-limit. |
| `tavily` | Yes ([tavily.com](https://tavily.com)) | Built for AI. Returns extracted content alongside results. 1,000 credits/month free. |
| `serper` | Yes ([serper.dev](https://serper.dev)) | Scrapes Google. Paid, credit-based. |

You can switch provider per-request by passing `provider` to `web_search`, or set a default in config.

---

## 🧹 How web_fetch cleans pages

Before returning content, the fetcher:

1. Removes scripts, styles, nav, headers, footers, iframes and SVGs
2. Converts the remaining HTML to plain text
3. Collapses excessive whitespace

For pages larger than 50KB the content is saved to a temp file and the path is returned — use your filesystem tools to read it from there.

---

## 🐳 Docker

```bash
docker build -t browse-mcp .
docker run -v $(pwd)/config.yaml:/config/config.yaml browse-mcp
```

---

## ⚠️ Limitations

- Max fetch size: 5MB
- Only HTTP and HTTPS are supported
- Pages with heavy JavaScript may not render correctly — the fetcher doesn't run JS
- DuckDuckGo may occasionally block requests; switch to Tavily or Serper for production use

---

## 🔧 Troubleshooting

### Empty search results
DuckDuckGo is rate-limiting. Try again in a moment or switch to Tavily.

### web_fetch returns partial content
The page is larger than 5MB. Use `web_download` to save it to disk first, then read it with filesystem tools.

### API key error
Make sure the environment variable is set and the config references it with `$VAR_NAME` syntax.

---

## 🤝 Contributing

For AI agents working on this codebase, see [AGENTS.md](.agents/AGENTS.md).

---

## 📄 License

Apache 2.0
