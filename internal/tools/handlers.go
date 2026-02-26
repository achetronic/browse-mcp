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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"browse-mcp/internal/web"

	"github.com/mark3labs/mcp-go/mcp"
)

// HandleToolWebSearch handles the web_search tool
func (tm *ToolsManager) HandleToolWebSearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	query := getString(args, "query", "")
	provider := getString(args, "provider", "")
	maxResults := getInt(args, "max_results", 10)

	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}

	// Use default provider from config if not specified
	if provider == "" {
		provider = tm.dependencies.DefaultProvider
	}
	if provider == "" {
		provider = web.ProviderDuckDuckGo
	}

	cfg := web.SearchConfig{
		TavilyAPIKey: tm.dependencies.TavilyAPIKey,
		SerperAPIKey: tm.dependencies.SerperAPIKey,
	}

	results, err := web.Search(ctx, tm.dependencies.HTTPClient, query, provider, maxResults, cfg)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %s", err.Error())), nil
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("No results found."), nil
	}

	output, err := json.Marshal(results)
	if err != nil {
		return mcp.NewToolResultError("failed to serialize results"), nil
	}

	return mcp.NewToolResultText(string(output)), nil
}

// HandleToolWebFetch handles the web_fetch tool
func (tm *ToolsManager) HandleToolWebFetch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	rawURL := getString(args, "url", "")
	timeout := getInt(args, "timeout", 30)

	if rawURL == "" {
		return mcp.NewToolResultError("url is required"), nil
	}

	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return mcp.NewToolResultError("url must start with http:// or https://"), nil
	}

	client := &http.Client{}
	if tm.dependencies.HTTPClient != nil {
		client = tm.dependencies.HTTPClient
	}
	_ = timeout // timeout handled by context in production; http.Client timeout is already set

	result, err := web.Fetch(ctx, client, rawURL, timeout)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("fetch failed: %s", err.Error())), nil
	}

	return mcp.NewToolResultText(result.Content), nil
}

// HandleToolWebDownload handles the web_download tool.
// If web.download_dir is configured, all file paths are resolved relative to it
// and any path traversal attempt (e.g. ../../etc/passwd) is rejected.
func (tm *ToolsManager) HandleToolWebDownload(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	rawURL := getString(args, "url", "")
	filePath := getString(args, "file_path", "")

	if rawURL == "" {
		return mcp.NewToolResultError("url is required"), nil
	}
	if filePath == "" {
		return mcp.NewToolResultError("file_path is required"), nil
	}

	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return mcp.NewToolResultError("url must start with http:// or https://"), nil
	}

	// If download_dir is configured, resolve file_path relative to it
	// and reject any path that escapes the directory.
	if tm.dependencies.DownloadDir != "" {
		resolvedPath := filepath.Join(tm.dependencies.DownloadDir, filePath)
		cleanBase := filepath.Clean(tm.dependencies.DownloadDir)
		cleanPath := filepath.Clean(resolvedPath)
		if !strings.HasPrefix(cleanPath, cleanBase+string(filepath.Separator)) && cleanPath != cleanBase {
			return mcp.NewToolResultError(fmt.Sprintf("file_path must be inside %s", tm.dependencies.DownloadDir)), nil
		}
		filePath = cleanPath
	}

	written, err := web.Download(ctx, tm.dependencies.HTTPClient, rawURL, filePath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("download failed: %s", err.Error())), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf(`{"success": true, "bytes_written": %d, "file_path": "%s"}`, written, filePath)), nil
}
