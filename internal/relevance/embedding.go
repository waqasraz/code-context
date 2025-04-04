package relevance

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"google.golang.org/api/option"

	"github.com/google/generative-ai-go/genai"
)

// --- Embedding Provider Abstraction ---

// EmbeddingAdapter defines the interface for generating embeddings.
type EmbeddingAdapter interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float64, error)
}

// EmbeddingOptions configures the embedding provider and relevance detection.
type EmbeddingOptions struct {
	Provider        string   // The embedding provider (e.g., "ollama", "gemini")
	Query           string   // The user query
	TargetPath      string   // The root path of the search
	CandidateFiles  []string // Potential files to analyze
	MaxFilesToCheck int      // Maximum number of files to return
	Model           string   // The embedding model to use
	Endpoint        string   // The endpoint URL (for Ollama/HTTP-based providers)
	APIKey          string   // API Key (for Gemini, OpenAI, etc.)
}

// DefaultEmbeddingOptions returns default configuration values.
func DefaultEmbeddingOptions() EmbeddingOptions {
	return EmbeddingOptions{
		Provider:        "ollama", // Default to Ollama
		MaxFilesToCheck: 20,
		Model:           "nomic-embed-text",
		Endpoint:        "http://localhost:11434/api/embeddings",
	}
}

// --- Ollama Adapter ---

// OllamaEmbeddingAdapter uses an Ollama-compatible HTTP endpoint.
type OllamaEmbeddingAdapter struct {
	Model    string
	Endpoint string
}

// ollamaEmbeddingRequest represents the request body for the Ollama embedding API
type ollamaEmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// ollamaEmbeddingResponse represents the response from the Ollama embedding API
type ollamaEmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

// GenerateEmbedding fetches embedding from an Ollama-like endpoint.
func (a *OllamaEmbeddingAdapter) GenerateEmbedding(ctx context.Context, text string) ([]float64, error) {
	if a.Endpoint == "" {
		return nil, fmt.Errorf("ollama endpoint is required")
	}
	reqBody := ollamaEmbeddingRequest{
		Model:  a.Model,
		Prompt: text,
	}
	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("ollama: error marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.Endpoint, bytes.NewBuffer(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("ollama: error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama: error making API request to %s: %w", a.Endpoint, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ollama: error reading response from %s: %w", a.Endpoint, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: API at %s returned status %d: %s", a.Endpoint, resp.StatusCode, string(respBody))
	}

	var embeddingResp ollamaEmbeddingResponse
	if err := json.Unmarshal(respBody, &embeddingResp); err != nil {
		return nil, fmt.Errorf("ollama: error parsing response from %s: %w", a.Endpoint, err)
	}

	return embeddingResp.Embedding, nil
}

// --- Gemini Adapter ---

// GeminiEmbeddingAdapter uses the Google AI Go SDK.
type GeminiEmbeddingAdapter struct {
	Model  string
	APIKey string
}

// GenerateEmbedding fetches embedding using the Gemini SDK.
func (a *GeminiEmbeddingAdapter) GenerateEmbedding(ctx context.Context, text string) ([]float64, error) {
	if a.APIKey == "" {
		return nil, fmt.Errorf("gemini: API key is required")
	}

	// Add retry logic with exponential backoff
	maxRetries := 5
	initialBackoff := 1000 // milliseconds
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate backoff with exponential increase and some jitter
			backoffMs := initialBackoff * (1 << (attempt - 1)) // 1s, 2s, 4s, 8s, 16s
			// Add some jitter (±20%)
			jitter := float64(backoffMs) * (0.8 + 0.4*float64(os.Getpid()%100)/100.0)
			backoffDuration := time.Duration(jitter) * time.Millisecond

			fmt.Printf("Rate limit hit. Retrying Gemini embedding request (attempt %d/%d) after %.1f second delay...\n",
				attempt+1, maxRetries, backoffDuration.Seconds())

			// Create a new context with timeout for this attempt
			retryCtx, cancel := context.WithTimeout(ctx, backoffDuration+30*time.Second)
			time.Sleep(backoffDuration)
			defer cancel()
			ctx = retryCtx
		}

		client, err := genai.NewClient(ctx, option.WithAPIKey(a.APIKey))
		if err != nil {
			lastErr = fmt.Errorf("gemini: error creating client for embedding: %w", err)
			// Only retry if this looks like a temporary error
			if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "rate") {
				continue
			}
			return nil, lastErr // Don't retry non-rate-limit errors
		}
		defer client.Close()

		em := client.EmbeddingModel(a.Model)
		res, err := em.EmbedContent(ctx, genai.Text(text))
		if err != nil {
			lastErr = err
			// Check if this is a rate limit error (usually 429 Too Many Requests)
			if strings.Contains(err.Error(), "429") ||
				strings.Contains(err.Error(), "rate") ||
				strings.Contains(err.Error(), "Resource has been exhausted") {
				fmt.Printf("Gemini embedding rate limit hit: %v\n", err)
				continue // Retry after backoff
			}
			return nil, fmt.Errorf("gemini: error getting embedding: %w", err)
		}

		if res == nil || res.Embedding == nil {
			lastErr = fmt.Errorf("gemini: received nil embedding")
			// This could be due to rate limiting as well
			continue
		}

		// Success! Convert []float32 to []float64
		embeddingF64 := make([]float64, len(res.Embedding.Values))
		for i, v := range res.Embedding.Values {
			embeddingF64[i] = float64(v)
		}

		if attempt > 0 {
			fmt.Printf("Successfully got Gemini embedding after %d retries\n", attempt)
		}

		return embeddingF64, nil
	}

	return nil, fmt.Errorf("gemini: exhausted retries (%d attempts): %w", maxRetries, lastErr)
}

// --- Provider Factory ---

// NewEmbeddingProvider creates an EmbeddingAdapter based on the options.
func NewEmbeddingProvider(opts EmbeddingOptions) (EmbeddingAdapter, error) {
	switch strings.ToLower(opts.Provider) {
	case "ollama", "local": // Treat "local" as an alias for "ollama" for now
		return &OllamaEmbeddingAdapter{
			Model:    opts.Model,
			Endpoint: opts.Endpoint,
		}, nil
	case "gemini":
		return &GeminiEmbeddingAdapter{
			Model:  opts.Model,
			APIKey: opts.APIKey,
		}, nil
	case "openai":
		// Placeholder for OpenAI adapter
		return nil, fmt.Errorf("OpenAI embedding provider not yet implemented")
	case "anthropic":
		// Placeholder for Anthropic adapter
		return nil, fmt.Errorf("Anthropic embedding provider not yet implemented")
	default:
		return nil, fmt.Errorf("unknown embedding provider: %s", opts.Provider)
	}
}

// --- Utility Functions (Cosine Similarity, File Reading) ---

// cosineSimilarity calculates the cosine similarity between two vectors
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, magnitudeA, magnitudeB float64
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		magnitudeA += a[i] * a[i]
		magnitudeB += b[i] * b[i]
	}

	if magnitudeA == 0 || magnitudeB == 0 {
		return 0
	}

	magnitudeA = math.Sqrt(magnitudeA)
	magnitudeB = math.Sqrt(magnitudeB)

	if magnitudeA == 0 || magnitudeB == 0 {
		return 0
	}

	return dotProduct / (magnitudeA * magnitudeB)
}

// readFileContent reads the content of a file, up to maxLines
func readFileContent(filePath string, maxLines int) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var content strings.Builder
	lineCount := 0

	// Use a scanner to read line by line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() && lineCount < maxLines {
		content.WriteString(scanner.Text())
		content.WriteString("\n")
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return content.String(), nil
}

// --- Relevance Identification Functions (Using Adapters) ---

// IdentifyRelevantFilesWithEmbeddings finds files using embeddings via the configured provider.
func IdentifyRelevantFilesWithEmbeddings(opts EmbeddingOptions) ([]FileInfo, error) {
	ctx := context.Background()

	// Apply defaults
	if opts.MaxFilesToCheck <= 0 {
		opts.MaxFilesToCheck = DefaultEmbeddingOptions().MaxFilesToCheck
	}
	if opts.Provider == "" {
		opts.Provider = DefaultEmbeddingOptions().Provider
	}
	if opts.Model == "" {
		opts.Model = DefaultEmbeddingOptions().Model
	}
	// Only set default endpoint if provider is ollama/local
	if opts.Endpoint == "" && (strings.ToLower(opts.Provider) == "ollama" || strings.ToLower(opts.Provider) == "local") {
		opts.Endpoint = DefaultEmbeddingOptions().Endpoint
	}

	// Create the embedding provider
	embeddingProvider, err := NewEmbeddingProvider(opts)
	if err != nil {
		return nil, fmt.Errorf("error creating embedding provider: %w", err)
	}

	// Get embedding for the query
	queryEmbedding, err := embeddingProvider.GenerateEmbedding(ctx, opts.Query)
	if err != nil {
		return nil, fmt.Errorf("error getting query embedding: %w", err)
	}

	// Score each file based on embedding similarity
	var scoredFiles []FileInfo
	for _, filePath := range opts.CandidateFiles {
		// Skip very large files
		fullPath := filepath.Join(opts.TargetPath, filePath)
		fileInfo, err := os.Stat(fullPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Error getting file info for %s: %v\n", filePath, err)
			continue
		}

		if fileInfo.Size() > 1024*1024 { // Skip files larger than 1MB
			fmt.Fprintf(os.Stderr, "Warning: Skipping large file %s (%d bytes)\n", filePath, fileInfo.Size())
			continue
		}

		// Read file content
		content, err := readFileContent(fullPath, 500) // Limit to 500 lines
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Error reading file %s: %v\n", filePath, err)
			continue
		}

		// Get embedding for the file content using the adapter
		fileEmbedding, err := embeddingProvider.GenerateEmbedding(ctx, content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Error getting embedding for %s: %v\n", filePath, err)
			continue
		}

		// Calculate similarity score
		score := cosineSimilarity(queryEmbedding, fileEmbedding)
		if score > 0 {
			scoredFiles = append(scoredFiles, FileInfo{
				Path:  filePath,
				Score: score,
			})
		}
	}

	// Sort files by score (highest first)
	sort.Slice(scoredFiles, func(i, j int) bool {
		return scoredFiles[i].Score > scoredFiles[j].Score
	})

	// Limit the number of files to return
	maxFiles := opts.MaxFilesToCheck
	if maxFiles > len(scoredFiles) {
		maxFiles = len(scoredFiles)
	}

	return scoredFiles[:maxFiles], nil
}

// IdentifyRelevantFilesWithHybridApproach finds files using a mix of embeddings, keywords, and path relevance.
func IdentifyRelevantFilesWithHybridApproach(embeddingOpts EmbeddingOptions) ([]FileInfo, error) {
	ctx := context.Background()

	// Apply defaults (similar to embedding-only function)
	if embeddingOpts.MaxFilesToCheck <= 0 {
		embeddingOpts.MaxFilesToCheck = DefaultEmbeddingOptions().MaxFilesToCheck
	}
	if embeddingOpts.Provider == "" {
		embeddingOpts.Provider = DefaultEmbeddingOptions().Provider
	}
	if embeddingOpts.Model == "" {
		embeddingOpts.Model = DefaultEmbeddingOptions().Model
	}
	if embeddingOpts.Endpoint == "" && (strings.ToLower(embeddingOpts.Provider) == "ollama" || strings.ToLower(embeddingOpts.Provider) == "local") {
		embeddingOpts.Endpoint = DefaultEmbeddingOptions().Endpoint
	}

	// Create the embedding provider
	embeddingProvider, err := NewEmbeddingProvider(embeddingOpts)
	if err != nil {
		// Don't fail entirely in hybrid mode, just warn and proceed without embeddings
		fmt.Fprintf(os.Stderr, "Warning: Failed to create embedding provider for hybrid search: %v. Proceeding with keyword and path relevance only.\n", err)
		embeddingProvider = nil // Set to nil to signal skipping embedding steps
	}

	// Get embedding for the query (only if provider was created)
	var queryEmbedding []float64
	if embeddingProvider != nil {
		queryEmbedding, err = embeddingProvider.GenerateEmbedding(ctx, embeddingOpts.Query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to get query embedding for hybrid search: %v. Proceeding without embedding scores.\n", err)
			queryEmbedding = nil // Signal to skip file embeddings
		} else {
			fmt.Println("Successfully generated query embedding for hybrid search.")
		}
	}

	// Extract keywords for traditional matching
	keywords := extractKeywords(embeddingOpts.Query)
	fmt.Printf("Keywords extracted from query: %v\n", keywords)

	var scoredFiles []FileInfo
	for _, filePath := range embeddingOpts.CandidateFiles {
		// File skipping logic (keep existing)
		fullPath := filepath.Join(embeddingOpts.TargetPath, filePath)
		fileInfo, err := os.Stat(fullPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Error getting file info for %s: %v\n", filePath, err)
			continue
		}
		if fileInfo.Size() > 1024*1024*2 { // Skip files larger than 2MB
			fmt.Fprintf(os.Stderr, "Warning: Skipping large file %s (%d bytes)\n", filePath, fileInfo.Size())
			continue
		}

		// Read file content (keep existing)
		content, err := readFileContent(fullPath, 800)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Error reading file %s: %v\n", filePath, err)
			continue
		}

		// --- Calculate Scores ---
		var embeddingScore float64
		if embeddingProvider != nil && queryEmbedding != nil { // Only calculate if provider and query embedding are valid
			fileEmbedding, err := embeddingProvider.GenerateEmbedding(ctx, content)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Error getting embedding for file %s: %v\n", filePath, err)
				embeddingScore = 0
			} else {
				embeddingScore = cosineSimilarity(queryEmbedding, fileEmbedding)
			}
		} else {
			embeddingScore = 0 // Assign 0 if embeddings are skipped
		}

		// Keyword score (keep existing)
		keywordScore, _ := scoreFile(fullPath, keywords) // Ignore error for hybrid scoring
		keywordScore = keywordScore / 10.0               // Normalize keyword score roughly

		// Path relevance score (keep existing)
		pathRelevance := getPathRelevanceScore(filePath, keywords)

		// Combine scores (keep existing weights for now)
		combinedScore := (embeddingScore * 0.7) + (keywordScore * 0.2) + (pathRelevance * 0.1)

		if combinedScore > 0 {
			scoredFiles = append(scoredFiles, FileInfo{
				Path:  filePath,
				Score: combinedScore,
			})
			fmt.Printf("File: %s, Embedding: %.2f, Keyword: %.2f, Path: %.2f, Combined: %.2f\n",
				filePath, embeddingScore, keywordScore, pathRelevance, combinedScore)
		}
	}

	// Sort files by combined score
	sort.Slice(scoredFiles, func(i, j int) bool {
		return scoredFiles[i].Score > scoredFiles[j].Score
	})

	// Limit the number of files
	maxFiles := embeddingOpts.MaxFilesToCheck
	if maxFiles > len(scoredFiles) {
		maxFiles = len(scoredFiles)
	}

	return scoredFiles[:maxFiles], nil
}

// --- Helper functions used by relevance logic ---

// getPathRelevanceScore calculates a score based on path matching (used by hybrid approach)
// (Keep existing function)
func getPathRelevanceScore(filePath string, keywords []string) float64 {
	pathLower := strings.ToLower(filePath)
	var score float64
	for _, keyword := range keywords {
		if containsAny(pathLower, keyword) {
			score += 0.5 // Base score for keyword in path
			// Bonus if keyword is in the filename itself
			if containsAny(strings.ToLower(filepath.Base(filePath)), keyword) {
				score += 0.5
			}
		}
	}
	return score
}

// containsAny checks if a string contains any of the substrings
// (Keep existing function)
func containsAny(s string, substr ...string) bool {
	for _, sub := range substr {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// extractKeywords extracts meaningful keywords from a query
// (Function definition removed, assuming it exists earlier)
