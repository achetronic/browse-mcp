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

package main

import (
	"log"
	"net/http"
	"time"

	"browse-mcp/internal/globals"
	"browse-mcp/internal/tools"
	"browse-mcp/internal/web"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// 0. Load configuration
	appCtx, err := globals.NewApplicationContext()
	if err != nil {
		log.Fatalf("failed creating application context: %v", err.Error())
	}

	// 1. Build HTTP client
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 2. Resolve default provider and API keys from config
	defaultProvider := appCtx.Config.Web.DefaultProvider
	if defaultProvider == "" {
		defaultProvider = web.ProviderDuckDuckGo
	}

	// 3. Create MCP server
	mcpServer := server.NewMCPServer(
		appCtx.Config.Server.Name,
		appCtx.Config.Server.Version,
		server.WithToolCapabilities(true),
	)

	// 4. Register tools
	tm := tools.NewToolsManager(tools.ToolsManagerDependencies{
		AppCtx:          appCtx,
		McpServer:       mcpServer,
		HTTPClient:      httpClient,
		DefaultProvider: defaultProvider,
		BraveAPIKey:     appCtx.Config.Web.Providers.Brave.APIKey,
		TavilyAPIKey:    appCtx.Config.Web.Providers.Tavily.APIKey,
		SerperAPIKey:    appCtx.Config.Web.Providers.Serper.APIKey,
	})
	tm.AddTools()

	// 5. Start transport
	switch appCtx.Config.Server.Transport.Type {
	case "http":
		httpServer := server.NewStreamableHTTPServer(mcpServer,
			server.WithHeartbeatInterval(30*time.Second),
			server.WithStateLess(false))

		mux := http.NewServeMux()
		mux.Handle("/mcp", httpServer)

		httpSrv := &http.Server{
			Addr:              appCtx.Config.Server.Transport.HTTP.Host,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       0,
		}

		appCtx.Logger.Info("starting StreamableHTTP server", "host", appCtx.Config.Server.Transport.HTTP.Host)
		if err := httpSrv.ListenAndServe(); err != nil {
			log.Fatal(err)
		}

	default:
		appCtx.Logger.Info("starting stdio server")
		if err := server.ServeStdio(mcpServer); err != nil {
			log.Fatal(err)
		}
	}
}
