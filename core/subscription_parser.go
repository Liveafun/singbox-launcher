package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// DecodeSubscriptionContent decodes subscription content from base64 or returns plain text
// Returns decoded content and error if decoding fails
func DecodeSubscriptionContent(content []byte) ([]byte, error) {
	if len(content) == 0 {
		return nil, fmt.Errorf("content is empty")
	}

	// Try to decode as base64
	decoded, err := base64.URLEncoding.DecodeString(strings.TrimSpace(string(content)))
	if err != nil {
		// If URL encoding fails, try standard encoding
		decoded, err = base64.StdEncoding.DecodeString(strings.TrimSpace(string(content)))
		if err != nil {
			// If both fail, assume it's plain text
			log.Printf("DecodeSubscriptionContent: Content is not base64, treating as plain text")
			return content, nil
		}
	}

	// Check if decoded content is empty
	if len(decoded) == 0 {
		return nil, fmt.Errorf("decoded content is empty")
	}

	return decoded, nil
}

// FetchSubscription fetches subscription content from URL and decodes it
// Returns decoded content and error if fetch or decode fails
func FetchSubscription(url string) ([]byte, error) {
	// Создаем контекст с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), NetworkRequestTimeout)
	defer cancel()

	// Используем универсальный HTTP клиент
	client := createHTTPClient(NetworkRequestTimeout)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent to avoid blocking
	req.Header.Set("User-Agent", "singbox-launcher/1.0")

	resp, err := client.Do(req)
	if err != nil {
		// Проверяем тип ошибки
		if IsNetworkError(err) {
			return nil, fmt.Errorf("network error: %s", GetNetworkErrorMessage(err))
		}
		return nil, fmt.Errorf("failed to fetch subscription: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("subscription server returned status %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read subscription content: %w", err)
	}

	// Check if content is empty
	if len(content) == 0 {
		return nil, fmt.Errorf("subscription returned empty content")
	}

	// Decode base64 if needed
	decoded, err := DecodeSubscriptionContent(content)
	if err != nil {
		return nil, fmt.Errorf("failed to decode subscription content: %w", err)
	}

	return decoded, nil
}

// ParserConfig represents the configuration structure from @ParcerConfig block
type ParserConfig struct {
	Version      int `json:"version"`
	ParserConfig struct {
		Proxies   []ProxySource    `json:"proxies"`
		Outbounds []OutboundConfig `json:"outbounds"`
	} `json:"ParserConfig"`
}

// ProxySource represents a proxy subscription source
type ProxySource struct {
	Source string              `json:"source"`
	Skip   []map[string]string `json:"skip,omitempty"`
}

// OutboundConfig represents an outbound selector configuration
type OutboundConfig struct {
	Tag       string                 `json:"tag"`
	Type      string                 `json:"type"`
	Options   map[string]interface{} `json:"options,omitempty"`
	Outbounds struct {
		Proxies          map[string]interface{} `json:"proxies,omitempty"`
		AddOutbounds     []string               `json:"addOutbounds,omitempty"`
		PreferredDefault map[string]interface{} `json:"preferredDefault,omitempty"`
	} `json:"outbounds,omitempty"`
	Comment string `json:"comment,omitempty"`
}

// ExtractParcerConfig extracts the @ParcerConfig block from config.json
// Returns the parsed ParserConfig structure and error if extraction or parsing fails
func ExtractParcerConfig(configPath string) (*ParserConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config.json: %w", err)
	}

	// Find the @ParcerConfig block using regex
	// Pattern matches: /** @ParcerConfig ... */
	pattern := regexp.MustCompile(`/\*\*\s*@ParcerConfig\s*\n([\s\S]*?)\*/`)
	matches := pattern.FindSubmatch(data)

	if len(matches) < 2 {
		return nil, fmt.Errorf("@ParcerConfig block not found in config.json")
	}

	// Extract the JSON content from the comment block
	jsonContent := strings.TrimSpace(string(matches[1]))

	// Parse the JSON
	var parserConfig ParserConfig
	if err := json.Unmarshal([]byte(jsonContent), &parserConfig); err != nil {
		return nil, fmt.Errorf("failed to parse @ParcerConfig JSON: %w", err)
	}

	log.Printf("ExtractParcerConfig: Successfully extracted @ParcerConfig with %d proxy sources and %d outbounds",
		len(parserConfig.ParserConfig.Proxies),
		len(parserConfig.ParserConfig.Outbounds))

	return &parserConfig, nil
}
