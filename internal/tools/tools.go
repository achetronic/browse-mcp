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

package tools

import (
	"net/http"
	"browse-mcp/internal/globals"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ToolsManagerDependencies holds the dependencies for the tools manager
type ToolsManagerDependencies struct {
	AppCtx          *globals.ApplicationContext
	McpServer       *server.MCPServer
	HTTPClient      *http.Client
	DefaultProvider string
	BraveAPIKey     string
	TavilyAPIKey    string
	SerperAPIKey    string
}

// ToolsManager manages the MCP tools registration
type ToolsManager struct {
	dependencies ToolsManagerDependencies
}

// NewToolsManager creates a new ToolsManager
func NewToolsManager(deps ToolsManagerDependencies) *ToolsManager {
	return &ToolsManager{dependencies: deps}
}

// AddTools registers all web tools into the MCP server
func (tm *ToolsManager) AddTools() {

	// web_search
	tool := mcp.NewTool("web_search",
		mcp.WithDescription(`Search the web and return a list of results with title, URL and snippet.
Default provider is DuckDuckGo (no API key needed). Set provider to use Brave, Tavily or Serper if you have API keys configured.
Recommended flow: web_search to find relevant URLs, then web_fetch to read the full content of the most relevant ones.`),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The search query"),
		),
		mcp.WithNumber("max_results",
			mcp.Description("Maximum number of results to return (default: 10, max: 20)"),
		),
		mcp.WithString("provider",
			mcp.Description("Search provider: duckduckgo (default, no key needed), brave, tavily, serper"),
		),
	)
	tm.dependencies.McpServer.AddTool(tool, tm.HandleToolWebSearch)

	// web_fetch
	tool = mcp.NewTool("web_fetch",
		mcp.WithDescription(`Fetch a URL and return its content as clean text.
HTML is automatically stripped of noise (scripts, nav, ads) and converted to readable text.
For large pages (>50KB) the content is saved to a temp file and the path is returned — use your filesystem tools to read it.
Max response size: 5MB. Only HTTP and HTTPS are supported.`),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("The URL to fetch (must start with http:// or https://)"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Request timeout in seconds (default: 30, max: 120)"),
		),
	)
	tm.dependencies.McpServer.AddTool(tool, tm.HandleToolWebFetch)

	// web_download
	tool = mcp.NewTool("web_download",
		mcp.WithDescription(`Download a file from a URL and save it to disk.
Use this for binary files, PDFs, images, or any file you want to save locally.
Returns the number of bytes written and the final file path.`),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("The URL to download from (must start with http:// or https://)"),
		),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Local path where the file should be saved (e.g. /tmp/file.pdf)"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Request timeout in seconds (default: 120, max: 600)"),
		),
	)
	tm.dependencies.McpServer.AddTool(tool, tm.HandleToolWebDownload)
}
