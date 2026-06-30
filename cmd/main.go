// SPDX-FileCopyrightText: 2026 Alby Hernández <hola@achetronic.com>
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"log"
	"net/http"
	"time"

	"browse-mcp/internal/globals"
	"browse-mcp/internal/handlers"
	"browse-mcp/internal/middlewares"
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

	// 1. HTTP client
	httpClient := &http.Client{Timeout: 30 * time.Second}

	// 2. Default provider
	defaultProvider := appCtx.Config.Web.DefaultProvider
	if defaultProvider == "" {
		defaultProvider = web.ProviderDuckDuckGo
	}

	// 3. Initialize HTTP middlewares
	accessLogsMw := middlewares.NewAccessLogsMiddleware(middlewares.AccessLogsMiddlewareDependencies{
		AppCtx: appCtx,
	})

	jwtValidationMw, err := middlewares.NewJWTValidationMiddleware(middlewares.JWTValidationMiddlewareDependencies{
		AppCtx: appCtx,
	})
	if err != nil {
		appCtx.Logger.Info("failed starting JWT validation middleware", "error", err.Error())
	}

	// 4. Initialize tool middlewares
	toolPolicyMw, err := middlewares.NewToolPolicyMiddleware(middlewares.ToolPolicyMiddlewareDependencies{
		AppCtx: appCtx,
	})
	if err != nil {
		appCtx.Logger.Info("failed starting tool policy middleware", "error", err.Error())
	}

	webPolicyMw, err := middlewares.NewWebPolicyMiddleware(middlewares.WebPolicyMiddlewareDependencies{
		AppCtx: appCtx,
	})
	if err != nil {
		appCtx.Logger.Info("failed starting web policy middleware", "error", err.Error())
	}

	// Collect tool middlewares
	var toolMiddlewares []middlewares.ToolMiddleware
	if toolPolicyMw != nil && len(appCtx.Config.Policies.Tools) > 0 {
		toolMiddlewares = append(toolMiddlewares, toolPolicyMw)
	}
	if webPolicyMw != nil && len(appCtx.Config.Policies.Web) > 0 {
		toolMiddlewares = append(toolMiddlewares, webPolicyMw)
	}

	// 5. Create MCP server
	mcpServer := server.NewMCPServer(
		appCtx.Config.Server.Name,
		appCtx.Config.Server.Version,
		server.WithToolCapabilities(true),
	)

	// 6. Initialize OAuth handlers
	hm := handlers.NewHandlersManager(handlers.HandlersManagerDependencies{
		AppCtx: appCtx,
	})

	// 6. Register tools
	tm := tools.NewToolsManager(tools.ToolsManagerDependencies{
		AppCtx:          appCtx,
		McpServer:       mcpServer,
		HTTPClient:      httpClient,
		Middlewares:     toolMiddlewares,
		DefaultProvider: defaultProvider,
		DownloadDir:     appCtx.Config.Web.DownloadDir,
		TavilyAPIKey:    appCtx.Config.Web.Providers.Tavily.APIKey,
		SerperAPIKey:    appCtx.Config.Web.Providers.Serper.APIKey,
	})
	tm.AddTools()

	// 7. Start transport
	switch appCtx.Config.Server.Transport.Type {
	case "http":
		httpServer := server.NewStreamableHTTPServer(mcpServer,
			server.WithHeartbeatInterval(30*time.Second),
			server.WithStateLess(false))

		mux := http.NewServeMux()
		mux.Handle("/mcp", accessLogsMw.Middleware(jwtValidationMw.Middleware(httpServer)))

		if appCtx.Config.OAuthAuthorizationServer.Enabled {
			mux.Handle("/.well-known/oauth-authorization-server"+appCtx.Config.OAuthAuthorizationServer.UrlSuffix,
				accessLogsMw.Middleware(http.HandlerFunc(hm.HandleOauthAuthorizationServer)))
		}

		if appCtx.Config.OAuthProtectedResource.Enabled {
			mux.Handle("/.well-known/oauth-protected-resource"+appCtx.Config.OAuthProtectedResource.UrlSuffix,
				accessLogsMw.Middleware(http.HandlerFunc(hm.HandleOauthProtectedResources)))
		}

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
