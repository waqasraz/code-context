package adapters

import (
	"context"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiAdapter provides an interface for Google's Gemini models using the Go SDK
type GeminiAdapter struct {
	APIKey    string // Google API key
	ModelName string // Model name (e.g., "gemini-1.5-flash")
	// Endpoint is no longer needed as the SDK handles it
}

// GenerateSummary generates a summary using Google's Gemini via the Go SDK
func (g *GeminiAdapter) GenerateSummary(query string, fileContent string, filePath string) (string, error) {
	if g.APIKey == "" {
		return "", fmt.Errorf("Google API key is required")
	}

	// Set default model if not provided
	modelName := "gemini-1.5-flash" // Use a recent default model
	if g.ModelName != "" {
		modelName = g.ModelName
	}

	// Use context.Background for the client and generation calls
	ctx := context.Background()

	// Create the Gemini client
	client, err := genai.NewClient(ctx, option.WithAPIKey(g.APIKey))
	if err != nil {
		return "", fmt.Errorf("error creating Gemini client: %w", err)
	}
	defer client.Close()

	// Get the generative model
	model := client.GenerativeModel(modelName)

	// Configure generation parameters
	model.GenerationConfig.Temperature = genai.Ptr[float32](0.2)
	model.GenerationConfig.MaxOutputTokens = genai.Ptr[int32](1500)
	model.GenerationConfig.TopP = genai.Ptr[float32](0.95)
	model.GenerationConfig.TopK = genai.Ptr[int32](40) // Note: TopK might not be supported by all models or configurations

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

	// Generate content using the SDK
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("error generating content via Gemini SDK: %w", err)
	}

	// Check for blocked response or missing candidates/parts
	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil || len(resp.Candidates[0].Content.Parts) == 0 {
		// Check for prompt feedback if available
		if resp != nil && resp.PromptFeedback != nil {
			return "", fmt.Errorf("gemini response blocked or empty, reason: %s", resp.PromptFeedback.BlockReason)
		}
		return "", fmt.Errorf("gemini response blocked or empty, no specific reason provided")
	}

	// Extract the text from the first candidate's first part
	// The SDK represents parts as an interface{}, so we need a type assertion
	firstPart := resp.Candidates[0].Content.Parts[0]
	textPart, ok := firstPart.(genai.Text)
	if !ok {
		return "", fmt.Errorf("unexpected response part type: %T", firstPart)
	}

	return string(textPart), nil
}
