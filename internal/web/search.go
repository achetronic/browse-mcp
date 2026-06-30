// SPDX-FileCopyrightText: 2026 Alby Hernández <hola@achetronic.com>
// SPDX-License-Identifier: Apache-2.0

package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	ProviderDuckDuckGo = "duckduckgo"
	
	ProviderTavily     = "tavily"
	ProviderSerper     = "serper"
)

// SearchResult represents a single search result
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// SearchConfig holds provider configuration
type SearchConfig struct {
	
	TavilyAPIKey string
	SerperAPIKey string
}

// Search performs a web search using the specified provider
func Search(ctx context.Context, client *http.Client, query, provider string, maxResults int, cfg SearchConfig) ([]SearchResult, error) {
	if maxResults <= 0 {
		maxResults = 10
	}
	if maxResults > 20 {
		maxResults = 20
	}

	switch provider {
	case ProviderTavily:
		if cfg.TavilyAPIKey == "" {
			return nil, fmt.Errorf("tavily API key not configured (set web.providers.tavily.api_key)")
		}
		return searchTavily(ctx, client, query, maxResults, cfg.TavilyAPIKey)
	case ProviderSerper:
		if cfg.SerperAPIKey == "" {
			return nil, fmt.Errorf("serper API key not configured (set web.providers.serper.api_key)")
		}
		return searchSerper(ctx, client, query, maxResults, cfg.SerperAPIKey)
	default:
		return searchDuckDuckGo(ctx, client, query, maxResults)
	}
}

// searchDuckDuckGo searches using DuckDuckGo HTML (no API key required)
func searchDuckDuckGo(ctx context.Context, client *http.Client, query string, maxResults int) ([]SearchResult, error) {
	searchURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var results []SearchResult
	doc.Find(".result").Each(func(i int, s *goquery.Selection) {
		if i >= maxResults {
			return
		}
		title := strings.TrimSpace(s.Find(".result__title").Text())
		snippet := strings.TrimSpace(s.Find(".result__snippet").Text())
		link, _ := s.Find(".result__url").Attr("href")
		if link == "" {
			link, _ = s.Find("a.result__a").Attr("href")
		}
		if title != "" {
			results = append(results, SearchResult{
				Title:   title,
				URL:     link,
				Snippet: snippet,
			})
		}
	})

	return results, nil
}

// searchTavily searches using Tavily API
func searchTavily(ctx context.Context, client *http.Client, query string, maxResults int, apiKey string) ([]SearchResult, error) {
	payload := map[string]interface{}{
		"api_key":         apiKey,
		"query":           query,
		"max_results":     maxResults,
		"include_answer":  false,
		"search_depth":    "basic",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.tavily.com/search", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var data struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var results []SearchResult
	for _, r := range data.Results {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
		})
	}
	return results, nil
}

// searchSerper searches using Serper API
func searchSerper(ctx context.Context, client *http.Client, query string, maxResults int, apiKey string) ([]SearchResult, error) {
	payload := map[string]interface{}{
		"q": query,
		"num": maxResults,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://google.serper.dev/search", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("X-API-KEY", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var data struct {
		Organic []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"organic"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var results []SearchResult
	for _, r := range data.Organic {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.Link,
			Snippet: r.Snippet,
		})
	}
	return results, nil
}

// NewHTTPClient creates a default HTTP client
func NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
	}
}
