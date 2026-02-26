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

package web

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

const (
	// MaxFetchSize is the maximum response size to read (5MB)
	MaxFetchSize = 5 * 1024 * 1024
	// LargeContentThreshold — above this, content is saved to a temp file
	LargeContentThreshold = 50 * 1024
)

var multipleNewlinesRe = regexp.MustCompile(`\n{3,}`)

// browserUA mimics a real browser to avoid bot blocks
const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// FetchResult holds the result of a web fetch
type FetchResult struct {
	URL         string
	Content     string
	SavedToFile string // non-empty if content was saved to a temp file
}

// Fetch downloads a URL and returns its content as clean text.
// HTML is stripped of noise and converted to plain text.
// If content is large, it's saved to a temp file.
func Fetch(ctx context.Context, client *http.Client, rawURL string, timeout int) (*FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxFetchSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	content := string(body)

	if strings.Contains(contentType, "text/html") {
		cleaned := removeNoisyElements(content)
		converted, err := convertHTMLToText(cleaned)
		if err != nil {
			return nil, fmt.Errorf("failed to convert HTML: %w", err)
		}
		content = cleanupText(converted)
	}

	result := &FetchResult{URL: rawURL}

	if len(content) > LargeContentThreshold {
		tmpFile, err := os.CreateTemp("", "web-fetch-*.txt")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp file: %w", err)
		}
		if _, err := tmpFile.WriteString(content); err != nil {
			tmpFile.Close()
			return nil, fmt.Errorf("failed to write temp file: %w", err)
		}
		tmpFile.Close()
		result.SavedToFile = tmpFile.Name()
		result.Content = fmt.Sprintf("Content saved to: %s\n\nFirst 2000 chars:\n\n%s", tmpFile.Name(), content[:min(2000, len(content))])
	} else {
		result.Content = content
	}

	return result, nil
}

// Download saves a URL's content to a file on disk
func Download(ctx context.Context, client *http.Client, rawURL, filePath string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", browserUA)

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return 0, fmt.Errorf("failed to create directories: %w", err)
	}

	outFile, err := os.Create(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	written, err := io.Copy(outFile, resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to write file: %w", err)
	}

	return written, nil
}

// removeNoisyElements strips scripts, styles, nav, header, footer etc.
func removeNoisyElements(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent
	}

	noisyTags := map[string]bool{
		"script": true, "style": true, "nav": true,
		"header": true, "footer": true, "aside": true,
		"noscript": true, "iframe": true, "svg": true,
	}

	var removeNodes func(*html.Node)
	removeNodes = func(n *html.Node) {
		var toRemove []*html.Node
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && noisyTags[c.Data] {
				toRemove = append(toRemove, c)
			} else {
				removeNodes(c)
			}
		}
		for _, node := range toRemove {
			n.RemoveChild(node)
		}
	}
	removeNodes(doc)

	var buf strings.Builder
	if err := html.Render(&buf, doc); err != nil {
		return htmlContent
	}
	return buf.String()
}

// convertHTMLToText converts HTML to plain text using goquery
func convertHTMLToText(htmlContent string) (string, error) {
	// Try markdown conversion first for better structure
	converter := md.NewConverter("", true, nil)
	result, err := converter.ConvertString(htmlContent)
	if err != nil {
		// Fallback: plain text extraction
		doc, err2 := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
		if err2 != nil {
			return "", fmt.Errorf("failed to parse HTML: %w", err)
		}
		return doc.Find("body").Text(), nil
	}
	return result, nil
}

// cleanupText removes excessive whitespace
func cleanupText(content string) string {
	content = multipleNewlinesRe.ReplaceAllString(content, "\n\n")
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
