package adapters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GeminiAdapter provides an interface for Google's Gemini models
type GeminiAdapter struct {
	APIKey    string // Google API key
	ModelName string // Model name (e.g., "gemini-pro")
	Endpoint  string // API endpoint, defaults to Google's standard endpoint
}

// GeminiRequest represents the request structure for Google Gemini API
type GeminiRequest struct {
	Contents         []GeminiContent        `json:"contents"`
	GenerationConfig GeminiGenerationConfig `json:"generationConfig,omitempty"`
}

// GeminiContent represents content in the request
type GeminiContent struct {
	Role  string       `json:"role,omitempty"` // For multi-turn conversations
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart represents a part of the content
type GeminiPart struct {
	Text string `json:"text"`
}

// GeminiGenerationConfig contains generation parameters
type GeminiGenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	TopP            float64 `json:"topP,omitempty"`
	TopK            int     `json:"topK,omitempty"`
}

// GeminiResponse represents the response from the Gemini API
type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// GenerateSummary generates a summary using Google's Gemini
func (g *GeminiAdapter) GenerateSummary(query string, fileContent string, filePath string) (string, error) {
	if g.APIKey == "" {
		return "", fmt.Errorf("Google API key is required")
	}

	// Set default model if not provided
	model := "gemini-pro"
	if g.ModelName != "" {
		model = g.ModelName
	}

	// Set default endpoint if not provided
	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, g.APIKey)
	if g.Endpoint != "" {
		endpoint = g.Endpoint
	}

	// Construct the prompt
	prompt := fmt.Sprintf(`You are a helpful assistant that summarizes code based on specific queries.

Analyze the following code file and respond to the user's query:

FILE PATH: %s

USER QUERY: %s

CODE CONTENT:
%s

Provide a concise summary focusing specifically on the user's query.
Include relevant details such as functions, classes, or patterns that relate to the query.
Keep your response under 500 words.`, filePath, query, fileContent)

	// Create the request body
	requestBody := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{
						Text: prompt,
					},
				},
			},
		},
		GenerationConfig: GeminiGenerationConfig{
			Temperature:     0.2,  // Lower temperature for more factual responses
			MaxOutputTokens: 1500, // Reasonable limit for summaries
			TopP:            0.95,
			TopK:            40,
		},
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
	var geminiResp GeminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return "", fmt.Errorf("error parsing response: %w", err)
	}

	if geminiResp.Error != nil {
		return "", fmt.Errorf("API error: %s", geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no content returned")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}
