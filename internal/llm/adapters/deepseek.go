package adapters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DeepSeekAdapter provides an interface for DeepSeek AI models
type DeepSeekAdapter struct {
	APIKey    string // DeepSeek API key
	ModelName string // Model name (e.g., "deepseek-chat" or "deepseek-reasoner")
	Endpoint  string // API endpoint, defaults to DeepSeek's standard endpoint
}

// DeepSeekRequest represents the request structure for DeepSeek's API
// Compatible with OpenAI format
type DeepSeekRequest struct {
	Model       string            `json:"model"`
	Messages    []DeepSeekMessage `json:"messages"`
	Stream      bool              `json:"stream,omitempty"`
	Temperature float64           `json:"temperature,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
}

// DeepSeekMessage represents a message in DeepSeek's API format
type DeepSeekMessage struct {
	Role    string `json:"role"`    // "system", "user", or "assistant"
	Content string `json:"content"` // Message content
}

// DeepSeekResponse represents the response from DeepSeek's API
// Compatible with OpenAI format
type DeepSeekResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// GenerateSummary generates a summary using DeepSeek's models
func (d *DeepSeekAdapter) GenerateSummary(query string, fileContent string, filePath string) (string, error) {
	if d.APIKey == "" {
		return "", fmt.Errorf("DeepSeek API key is required")
	}

	// Set default endpoint if not provided
	endpoint := "https://api.deepseek.com/chat/completions"
	if d.Endpoint != "" {
		endpoint = d.Endpoint
	}

	// Set default model if not provided
	model := "deepseek-chat"
	if d.ModelName != "" {
		model = d.ModelName
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
	requestBody := DeepSeekRequest{
		Model: model,
		Messages: []DeepSeekMessage{
			{
				Role:    "system",
				Content: "You are a helpful assistant that summarizes code based on specific queries.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Stream:      false,
		Temperature: 0.3,
		MaxTokens:   1500, // Reasonable limit for summaries
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
	req.Header.Set("Authorization", "Bearer "+d.APIKey)

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
	var deepSeekResp DeepSeekResponse
	if err := json.Unmarshal(respBody, &deepSeekResp); err != nil {
		return "", fmt.Errorf("error parsing response: %w", err)
	}

	if deepSeekResp.Error != nil {
		return "", fmt.Errorf("API error: %s", deepSeekResp.Error.Message)
	}

	if len(deepSeekResp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned")
	}

	return deepSeekResp.Choices[0].Message.Content, nil
}
