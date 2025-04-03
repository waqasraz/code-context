package adapters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AnthropicAdapter provides an interface for Anthropic's Claude models
type AnthropicAdapter struct {
	APIKey    string // Anthropic API key
	ModelName string // Model name (e.g., "claude-3-opus-20240229")
	Endpoint  string // API endpoint, defaults to Anthropic's standard endpoint
}

// AnthropicRequest represents the request structure for Anthropic's API
type AnthropicRequest struct {
	Model     string             `json:"model"`            // Model name (e.g., "claude-3-opus-20240229")
	Messages  []AnthropicMessage `json:"messages"`         // Array of messages
	MaxTokens int                `json:"max_tokens"`       // Maximum number of tokens to generate
	System    string             `json:"system,omitempty"` // Optional system prompt
}

// AnthropicMessage represents a message in Anthropic's API format
type AnthropicMessage struct {
	Role    string `json:"role"`    // "user" or "assistant"
	Content string `json:"content"` // Message content
}

// AnthropicResponse represents the response from Anthropic's API
type AnthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// GenerateSummary generates a summary using Anthropic's Claude
func (a *AnthropicAdapter) GenerateSummary(query string, fileContent string, filePath string) (string, error) {
	if a.APIKey == "" {
		return "", fmt.Errorf("Anthropic API key is required")
	}

	// Set default endpoint if not provided
	endpoint := "https://api.anthropic.com/v1/messages"
	if a.Endpoint != "" {
		endpoint = a.Endpoint
	}

	// Set default model if not provided
	model := "claude-3-opus-20240229"
	if a.ModelName != "" {
		model = a.ModelName
	}

	// Construct the prompt
	prompt := fmt.Sprintf(`
Analyze the following code file and respond to the user's query:

FILE PATH: %s

USER QUERY: %s

CODE CONTENT:
%s

Provide a concise summary focusing specifically on the user's query.
Include relevant details such as functions, classes, or patterns that relate to the query.
Keep your response under 500 words.
`, filePath, query, fileContent)

	// Create the request body
	requestBody := AnthropicRequest{
		Model: model,
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens: 1500, // Reasonable limit for summaries
		System:    "You are a helpful assistant that summarizes code based on specific queries.",
	}

	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %w", err)
	}

	// Create and send the HTTP request
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(requestJSON))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.APIKey)             // Anthropic uses x-api-key
	req.Header.Set("anthropic-version", "2023-06-01") // API version

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse the response
	var anthropicResp AnthropicResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return "", fmt.Errorf("error parsing response: %w", err)
	}

	if anthropicResp.Error != nil {
		return "", fmt.Errorf("API error: %s", anthropicResp.Error.Message)
	}

	if len(anthropicResp.Content) == 0 {
		return "", fmt.Errorf("no content returned")
	}

	return anthropicResp.Content[0].Text, nil
}
