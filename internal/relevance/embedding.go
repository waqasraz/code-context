package relevance

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// EmbeddingOptions configures the embedding-based relevance detection
type EmbeddingOptions struct {
	Query           string   // The user query
	TargetPath      string   // The root path of the search
	CandidateFiles  []string // Potential files to analyze
	MaxFilesToCheck int      // Maximum number of files to return
	EmbeddingModel  string   // The embedding model to use
	EmbeddingURL    string   // The URL of the embedding service
}

// DefaultEmbeddingOptions returns default configuration values for embedding-based relevance
func DefaultEmbeddingOptions() EmbeddingOptions {
	return EmbeddingOptions{
		MaxFilesToCheck: 20,
		EmbeddingModel:  "nomic-embed-text",
		EmbeddingURL:    "http://localhost:11434/api/embeddings",
	}
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

// getEmbedding generates an embedding for the given text using Ollama
func getEmbedding(text, model, url string) ([]float64, error) {
	// Prepare request body
	reqBody := ollamaEmbeddingRequest{
		Model:  model,
		Prompt: text,
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	// Make the API request
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("error making API request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	// Check for non-200 status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var embeddingResp ollamaEmbeddingResponse
	if err := json.Unmarshal(respBody, &embeddingResp); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return embeddingResp.Embedding, nil
}

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

// IdentifyRelevantFilesWithEmbeddings finds the files most relevant to the query using embeddings
func IdentifyRelevantFilesWithEmbeddings(opts EmbeddingOptions) ([]FileInfo, error) {
	// Apply defaults for any unset options
	if opts.MaxFilesToCheck <= 0 {
		opts.MaxFilesToCheck = DefaultEmbeddingOptions().MaxFilesToCheck
	}
	if opts.EmbeddingModel == "" {
		opts.EmbeddingModel = DefaultEmbeddingOptions().EmbeddingModel
	}
	if opts.EmbeddingURL == "" {
		opts.EmbeddingURL = DefaultEmbeddingOptions().EmbeddingURL
	}

	// Get embedding for the query
	queryEmbedding, err := getEmbedding(opts.Query, opts.EmbeddingModel, opts.EmbeddingURL)
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

		// Get embedding for the file content
		fileEmbedding, err := getEmbedding(content, opts.EmbeddingModel, opts.EmbeddingURL)
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

// IdentifyRelevantFilesWithHybridApproach finds files most relevant to the query using both embeddings and keywords
func IdentifyRelevantFilesWithHybridApproach(embeddingOpts EmbeddingOptions) ([]FileInfo, error) {
	// Apply defaults for any unset options
	if embeddingOpts.MaxFilesToCheck <= 0 {
		embeddingOpts.MaxFilesToCheck = DefaultEmbeddingOptions().MaxFilesToCheck
	}
	if embeddingOpts.EmbeddingModel == "" {
		embeddingOpts.EmbeddingModel = DefaultEmbeddingOptions().EmbeddingModel
	}
	if embeddingOpts.EmbeddingURL == "" {
		embeddingOpts.EmbeddingURL = DefaultEmbeddingOptions().EmbeddingURL
	}

	// Get embedding for the query
	queryEmbedding, err := getEmbedding(embeddingOpts.Query, embeddingOpts.EmbeddingModel, embeddingOpts.EmbeddingURL)
	if err != nil {
		return nil, fmt.Errorf("error getting query embedding: %w", err)
	}

	// Extract keywords for traditional matching
	keywords := extractKeywords(embeddingOpts.Query)
	fmt.Printf("Keywords extracted from query: %v\n", keywords)

	// Score each file based on both embedding similarity and keyword matching
	var scoredFiles []FileInfo
	for _, filePath := range embeddingOpts.CandidateFiles {
		// Skip very large files
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

		// Read file content
		content, err := readFileContent(fullPath, 800) // Increased from 500 to 800 lines
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Error reading file %s: %v\n", filePath, err)
			continue
		}

		// Get embedding for the file content
		fileEmbedding, err := getEmbedding(content, embeddingOpts.EmbeddingModel, embeddingOpts.EmbeddingURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Error getting embedding for %s: %v\n", filePath, err)
			continue
		}

		// Calculate semantic similarity score (weighted at 70%)
		semanticScore := cosineSimilarity(queryEmbedding, fileEmbedding)

		// Calculate keyword-based score (weighted at 30%)
		var keywordScore float64
		lowercaseContent := strings.ToLower(content)
		for _, keyword := range keywords {
			if strings.Contains(lowercaseContent, strings.ToLower(keyword)) {
				keywordScore += 1.0
			}
		}

		// Normalize keyword score (0-1 range)
		if len(keywords) > 0 {
			keywordScore = keywordScore / float64(len(keywords))
		}

		// Calculate path relevance for language-specific boosts
		pathRelevance := getPathRelevanceScore(filePath, embeddingOpts.Query)

		// Combine scores with weightings
		// 70% semantic, 20% keyword, 10% path relevance
		combinedScore := (semanticScore * 0.7) + (keywordScore * 0.2) + (pathRelevance * 0.1)

		if combinedScore > 0 {
			scoredFiles = append(scoredFiles, FileInfo{
				Path:  filePath,
				Score: combinedScore,
			})
			fmt.Printf("File: %s, Semantic: %.2f, Keyword: %.2f, Path: %.2f, Combined: %.2f\n",
				filePath, semanticScore, keywordScore, pathRelevance, combinedScore)
		}
	}

	// Sort files by score (highest first)
	sort.Slice(scoredFiles, func(i, j int) bool {
		return scoredFiles[i].Score > scoredFiles[j].Score
	})

	// Limit the number of files to return
	maxFiles := embeddingOpts.MaxFilesToCheck
	if maxFiles > len(scoredFiles) {
		maxFiles = len(scoredFiles)
	}

	return scoredFiles[:maxFiles], nil
}

// getPathRelevanceScore assigns a relevance score (0-1) based on file path and extension
// This helps prioritize certain file types based on the query
func getPathRelevanceScore(path string, query string) float64 {
	lowerPath := strings.ToLower(path)
	lowerQuery := strings.ToLower(query)

	var score float64 = 0.0

	// Base score if path contains words from query
	pathParts := strings.Split(lowerPath, "/")
	for _, part := range pathParts {
		if strings.Contains(lowerQuery, part) || strings.Contains(part, lowerQuery) {
			score += 0.3
			break
		}
	}

	// Check file extensions and boost certain types based on query topics
	if strings.HasSuffix(lowerPath, ".go") && containsAny(lowerQuery, []string{"go", "golang"}) {
		score += 0.2
	} else if strings.HasSuffix(lowerPath, ".java") && containsAny(lowerQuery, []string{"java", "spring"}) {
		score += 0.2
	} else if strings.HasSuffix(lowerPath, ".py") && containsAny(lowerQuery, []string{"python", "django"}) {
		score += 0.2
	} else if strings.HasSuffix(lowerPath, ".js") && containsAny(lowerQuery, []string{"javascript", "nodejs"}) {
		score += 0.2
	} else if strings.HasSuffix(lowerPath, ".ts") && containsAny(lowerQuery, []string{"typescript", "angular"}) {
		score += 0.2
	} else if strings.HasSuffix(lowerPath, ".cs") && containsAny(lowerQuery, []string{"c#", "dotnet", ".net"}) {
		score += 0.2
	} else if strings.HasSuffix(lowerPath, ".php") && containsAny(lowerQuery, []string{"php", "laravel"}) {
		score += 0.2
	}

	// Check for typical important file patterns
	if strings.Contains(lowerPath, "main.") || strings.Contains(lowerPath, "index.") {
		score += 0.1
	}
	if strings.Contains(lowerPath, "controller") || strings.Contains(lowerPath, "service") {
		score += 0.1
	}
	if strings.Contains(lowerPath, "api") || strings.Contains(lowerPath, "handler") {
		score += 0.1
	}

	// Cap the score at 1.0
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// containsAny checks if the string contains any of the substrings
func containsAny(s string, substrings []string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
