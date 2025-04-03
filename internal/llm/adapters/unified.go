package adapters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// UnifiedAdapter provides a single interface to multiple LLM providers
// This is inspired by LiteLLM and similar unified API services
type UnifiedAdapter struct {
	Endpoint  string            // API endpoint
	APIKey    string            // API key
	ModelName string            // Model name
	Headers   map[string]string // Additional headers
}

// ModelRequest represents a unified request format for different LLM providers
type ModelRequest struct {
	Model       string         `json:"model"`       // Model identifier
	Provider    string         `json:"provider"`    // Provider identifier (optional)
	Messages    []Message      `json:"messages"`    // Chat messages for chat models
	Prompt      string         `json:"prompt"`      // Text prompt for completion models
	Temperature float64        `json:"temperature"` // Sampling temperature
	MaxTokens   int            `json:"max_tokens"`  // Maximum tokens to generate
	Stream      bool           `json:"stream"`      // Stream response (not used here)
	Extra       map[string]any `json:"extra"`       // Extra provider-specific parameters
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`    // Role: "system", "user", or "assistant"
	Content string `json:"content"` // Message content
}

// ModelResponse represents a unified response format from different LLM providers
type ModelResponse struct {
	ID      string `json:"id"`      // Response ID
	Object  string `json:"object"`  // Object type
	Model   string `json:"model"`   // Model used
	Content string `json:"content"` // Generated content
	Error   string `json:"error"`   // Error message, if any
}

// GenerateSummary uses the unified adapter to generate a summary
func (a *UnifiedAdapter) GenerateSummary(query string, fileContent string, filePath string) (string, error) {
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

	// Determine if we're using a chat-based or completion-based model
	var isChatModel bool = true // Default to chat model
	if strings.Contains(a.ModelName, "completion") || strings.Contains(a.ModelName, "text-") {
		isChatModel = false
	}

	// Prepare the request based on model type
	request := ModelRequest{
		Model:       a.ModelName,
		Temperature: 0.3,   // Lower temperature for more factual responses
		MaxTokens:   1000,  // Reasonable limit for summaries
		Stream:      false, // No streaming
		Extra:       nil,   // No extra parameters
	}

	if isChatModel {
		// Chat-based models (GPT-4, Claude, etc.)
		request.Messages = []Message{
			{
				Role:    "system",
				Content: "You are a helpful assistant that summarizes code based on specific queries.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		}
	} else {
		// Completion-based models
		request.Prompt = prompt
	}

	// Marshal the request
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make the HTTP request
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest("POST", a.Endpoint, bytes.NewBuffer(requestJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if a.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+a.APIKey)
	}

	// Set additional headers if provided
	for key, value := range a.Headers {
		req.Header.Set(key, value)
	}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse the response
	var modelResp ModelResponse
	if err := json.Unmarshal(respBody, &modelResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if modelResp.Error != "" {
		return "", fmt.Errorf("API error: %s", modelResp.Error)
	}

	return modelResp.Content, nil
}
