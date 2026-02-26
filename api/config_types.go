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

package api

// Configuration represents the complete configuration structure
type Configuration struct {
	Server  ServerConfig `yaml:"server"`
	Web     WebConfig    `yaml:"web,omitempty"`
}

// ServerConfig holds MCP server configuration
type ServerConfig struct {
	Name      string          `yaml:"name"`
	Version   string          `yaml:"version"`
	Transport TransportConfig `yaml:"transport"`
}

// TransportConfig holds transport configuration
type TransportConfig struct {
	Type string     `yaml:"type"`
	HTTP HTTPConfig `yaml:"http,omitempty"`
}

// HTTPConfig holds HTTP transport configuration
type HTTPConfig struct {
	Host string `yaml:"host"`
}

// WebConfig holds web search/fetch configuration
type WebConfig struct {
	DefaultProvider string        `yaml:"default_provider,omitempty"`
	Providers       ProvidersConfig `yaml:"providers,omitempty"`
}

// ProvidersConfig holds API keys for each search provider
type ProvidersConfig struct {
	Tavily TavilyConfig `yaml:"tavily,omitempty"`
	Serper SerperConfig `yaml:"serper,omitempty"`
}

// BraveConfig removed — requires credit card even for free tier
// TavilyConfig holds Tavily API configuration
type TavilyConfig struct {
	APIKey string `yaml:"api_key"`
}

// SerperConfig holds Serper API configuration
type SerperConfig struct {
	APIKey string `yaml:"api_key"`
}
