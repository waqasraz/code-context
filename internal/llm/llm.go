package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/waqasraz/code-context/internal/llm/adapters"
)

// Provider defines the interface for different LLM providers
type Provider interface {
	GenerateSummary(query string, fileContent string, filePath string) (string, error)
}

// Config holds the configuration for the LLM service
type Config struct {
	APIKey    string
	Endpoint  string
	ModelName string
	Provider  string            // "openai", "anthropic", "gemini", "local", "unified", etc.
	Headers   map[string]string // Additional headers for API requests
}

// NewProvider creates an appropriate LLM provider based on configuration
func NewProvider(cfg Config) (Provider, error) {
	switch strings.ToLower(cfg.Provider) {
	case "openai":
		return &OpenAIProvider{
			APIKey:    cfg.APIKey,
			Endpoint:  cfg.Endpoint,
			ModelName: cfg.ModelName,
		}, nil
	case "anthropic":
		return &adapters.AnthropicAdapter{
			APIKey:    cfg.APIKey,
			Endpoint:  cfg.Endpoint,
			ModelName: cfg.ModelName,
		}, nil
	case "gemini":
		return &adapters.GeminiAdapter{
			APIKey:    cfg.APIKey,
			ModelName: cfg.ModelName,
		}, nil
	case "deepseek":
		return &adapters.DeepSeekAdapter{
			APIKey:    cfg.APIKey,
			Endpoint:  cfg.Endpoint,
			ModelName: cfg.ModelName,
		}, nil
	case "local":
		return &LocalProvider{
			Endpoint:  cfg.Endpoint,
			ModelName: cfg.ModelName,
		}, nil
	case "unified":
		return &adapters.UnifiedAdapter{
			Endpoint:  cfg.Endpoint,
			APIKey:    cfg.APIKey,
			ModelName: cfg.ModelName,
			Headers:   cfg.Headers,
		}, nil
	default:
		// Default to a placeholder provider if not specified or invalid
		return &PlaceholderProvider{}, nil
	}
}

// OpenAIProvider implements the Provider interface for OpenAI models
type OpenAIProvider struct {
	APIKey    string
	Endpoint  string
	ModelName string
}

// OpenAIRequest represents the request structure for OpenAI API
type OpenAIRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIResponse represents the response structure from OpenAI API
type OpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// GenerateSummary generates a summary of a file based on the query
func (p *OpenAIProvider) GenerateSummary(query string, fileContent string, filePath string) (string, error) {
	if p.APIKey == "" {
		return "", fmt.Errorf("OpenAI API key is required")
	}

	endpoint := "https://api.openai.com/v1/chat/completions"
	if p.Endpoint != "" {
		endpoint = p.Endpoint
	}

	model := "gpt-3.5-turbo"
	if p.ModelName != "" {
		model = p.ModelName
	}

	// Construct the prompt
	prompt := fmt.Sprintf(`
You are a code summarizer. Analyze the following code file and respond to the user's query:

FILE PATH: %s

USER QUERY: %s

CODE CONTENT:
%s

Provide a concise summary focusing specifically on the user's query. 
Include relevant details such as functions, classes, or patterns that relate to the query.
Keep your response under 500 words.
`, filePath, query, fileContent)

	// Create the request body
	requestBody := OpenAIRequest{
		Model: model,
		Messages: []Message{
			{
				Role:    "system",
				Content: "You are a helpful assistant that summarizes code based on specific queries.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
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
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

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
	var openAIResp OpenAIResponse
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return "", fmt.Errorf("error parsing response: %w", err)
	}

	if openAIResp.Error != nil {
		return "", fmt.Errorf("API error: %s", openAIResp.Error.Message)
	}

	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	return openAIResp.Choices[0].Message.Content, nil
}

// LocalProvider implements the Provider interface for locally hosted models
type LocalProvider struct {
	Endpoint  string
	ModelName string
}

// OllamaResponse represents the response structure from Ollama API
type OllamaResponse struct {
	Response string `json:"response"`
	Context  []int  `json:"context,omitempty"`
	Done     bool   `json:"done,omitempty"`
	Error    string `json:"error,omitempty"`
}

// OllamaChatResponse represents the response structure from Ollama Chat API
type OllamaChatResponse struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done  bool   `json:"done,omitempty"`
	Error string `json:"error,omitempty"`
}

// GenerateSummary generates a summary using a locally hosted model
func (p *LocalProvider) GenerateSummary(query string, fileContent string, filePath string) (string, error) {
	fmt.Println("=== OLLAMA DEBUGGING ===")
	fmt.Println("Attempting to connect to Ollama...")

	// Use the newer chat API format which is more stable
	chatEndpoint := "http://localhost:11434/api/chat"
	if p.Endpoint != "" {
		chatEndpoint = p.Endpoint
		fmt.Println("Using custom endpoint:", chatEndpoint)
	} else {
		fmt.Println("Using default endpoint:", chatEndpoint)
	}

	if p.ModelName == "" {
		p.ModelName = "llama2"
		fmt.Println("Using default model:", p.ModelName)
	} else {
		fmt.Println("Using specified model:", p.ModelName)
	}

	// Construct the prompt
	fmt.Println("Creating prompt for file:", filePath)

	// Create the chat prompt message
	userPrompt := fmt.Sprintf(`Analyze the following code file and respond to the query:

FILE PATH: %s

USER QUERY: %s

CODE CONTENT:
%s

Provide a concise summary focusing specifically on the user's query. 
Include relevant details such as functions, classes, or patterns that relate to the query.
Keep your response under 500 words.
DO NOT include recommendations, suggestions, or any advice on how to improve the code.
DO NOT suggest tests that should be written.
Focus ONLY on describing what the code does related to the query.`, filePath, query, fileContent)

	// Create the request body for Ollama chat
	chatRequestBody := map[string]interface{}{
		"model": p.ModelName,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a helpful assistant that summarizes code based on specific queries.",
			},
			{
				"role":    "user",
				"content": userPrompt,
			},
		},
		"stream": false,
		"options": map[string]interface{}{
			"temperature": 0.2,
		},
	}

	chatRequestJSON, err := json.Marshal(chatRequestBody)
	if err != nil {
		fmt.Println("Error marshaling chat request:", err)
		goto fallback
	}

	// Make direct HTTP request
	{
		fmt.Println("Sending request to Ollama...")

		client := &http.Client{Timeout: 300 * time.Second} // Increase timeout to 5 minutes
		req, err := http.NewRequest("POST", chatEndpoint, bytes.NewBuffer(chatRequestJSON))
		if err != nil {
			fmt.Println("Error creating request:", err)
			goto fallback
		}

		req.Header.Set("Content-Type", "application/json")

		fmt.Println("Waiting for Ollama response...")
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println("Error connecting to Ollama:", err)
			goto fallback
		}
		defer resp.Body.Close()

		// Read the response
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error reading response:", err)
			goto fallback
		}

		fmt.Println("HTTP Status:", resp.StatusCode)

		if resp.StatusCode != http.StatusOK {
			fmt.Println("API error:", string(respBody))
			goto fallback
		}

		// Show a preview of the response
		if len(respBody) > 100 {
			fmt.Println("Response received (truncated):", string(respBody[:100]), "...")
		} else {
			fmt.Println("Response received:", string(respBody))
		}

		// Try to parse as Ollama chat response
		var ollamaChatResp OllamaChatResponse
		if err := json.Unmarshal(respBody, &ollamaChatResp); err != nil {
			fmt.Println("Error parsing chat response:", err)
			fmt.Println("Full response:", string(respBody))
			goto fallback
		}

		if ollamaChatResp.Message.Content != "" {
			fmt.Println("Successfully received valid chat response!")
			fmt.Println("=== END DEBUGGING ===")
			return ollamaChatResp.Message.Content, nil
		} else {
			fmt.Println("Response was empty or invalid")
			fmt.Println("Full response:", string(respBody))
		}
	}

fallback:
	fmt.Println("Falling back to placeholder provider")
	fmt.Println("=== END DEBUGGING ===")

	placeholder := &PlaceholderProvider{}
	return placeholder.GenerateSummary(query, fileContent, filePath)
}

// Helper function to truncate long strings for logging
func truncateString(str string, num int) string {
	if len(str) <= num {
		return str
	}
	return str[:num] + "..."
}

// PlaceholderProvider is used when no LLM integration is available
type PlaceholderProvider struct{}

// GenerateSummary generates a placeholder summary
func (p *PlaceholderProvider) GenerateSummary(query string, fileContent string, filePath string) (string, error) {
	// Create a reasonable placeholder based on file content
	lines := strings.Split(fileContent, "\n")
	var summary strings.Builder

	fmt.Fprintf(&summary, "**Placeholder Summary for %s**\n\n", filePath)
	fmt.Fprintf(&summary, "This is an automated placeholder summary as no LLM provider was configured.\n\n")

	// Count some basic stats
	fmt.Fprintf(&summary, "* File contains %d lines of code\n", len(lines))

	// Try to identify file type based on extension
	ext := strings.ToLower(string([]rune(filePath)[len(filePath)-3:]))
	if len(ext) >= 2 && ext[0] == '.' {
		fmt.Fprintf(&summary, "* File type: %s\n", ext[1:])
	}

	// Try to extract some info from the file content
	if len(lines) > 0 {
		importCount := 0
		funcCount := 0
		classCount := 0

		for _, line := range lines {
			if strings.Contains(line, "import ") || strings.Contains(line, "require ") {
				importCount++
			}
			if strings.Contains(line, "func ") || strings.Contains(line, "function ") {
				funcCount++
			}
			if strings.Contains(line, "class ") || strings.Contains(line, "struct ") {
				classCount++
			}
		}

		if importCount > 0 {
			fmt.Fprintf(&summary, "* Contains approximately %d import/require statements\n", importCount)
		}
		if funcCount > 0 {
			fmt.Fprintf(&summary, "* Contains approximately %d function declarations\n", funcCount)
		}
		if classCount > 0 {
			fmt.Fprintf(&summary, "* Contains approximately %d class/struct declarations\n", classCount)
		}
	}

	fmt.Fprintf(&summary, "\nQuery: \"%s\" (No AI-generated response available)\n", query)

	return summary.String(), nil
}

// GenerateSummaries processes multiple files to generate summaries based on the query
func GenerateSummaries(provider Provider, query string, targetPath string, relevantFiles []string) (map[string]string, error) {
	summaries := make(map[string]string)

	for _, filePath := range relevantFiles {
		fullPath := filepath.Join(targetPath, filePath)

		// Read file content
		content, err := os.ReadFile(fullPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not read file %s: %v\n", filePath, err)
			summaries[filePath] = fmt.Sprintf("Error: Could not read file: %v", err)
			continue
		}

		// Generate summary
		fmt.Printf("Generating summary for %s...\n", filePath)
		summary, err := provider.GenerateSummary(query, string(content), filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to generate summary for %s: %v\n", filePath, err)
			summaries[filePath] = fmt.Sprintf("Error: Failed to generate summary: %v", err)
			continue
		}

		summaries[filePath] = summary
	}

	return summaries, nil
}
