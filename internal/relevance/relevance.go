package relevance

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// FileInfo represents information about a file and its relevance score
type FileInfo struct {
	Path  string
	Score float64
}

// Options configures the relevance identification process
type Options struct {
	Query           string   // The user query
	TargetPath      string   // The root path of the search
	CandidateFiles  []string // Potential files to analyze
	MaxFilesToCheck int      // Maximum number of files to return
}

// DefaultOptions returns default configuration values
func DefaultOptions() Options {
	return Options{
		MaxFilesToCheck: 20, // By default, analyze top 20 files
	}
}

// IdentifyRelevantFiles finds the files most relevant to the query
func IdentifyRelevantFiles(opts Options) ([]FileInfo, error) {
	// Apply defaults for any unset options
	if opts.MaxFilesToCheck <= 0 {
		opts.MaxFilesToCheck = DefaultOptions().MaxFilesToCheck
	}

	// Extract keywords from the query for basic keyword matching
	keywords := extractKeywords(opts.Query)
	if len(keywords) == 0 {
		return nil, fmt.Errorf("could not extract meaningful keywords from query")
	}

	// Score each file based on keyword matching
	var scoredFiles []FileInfo
	for _, filePath := range opts.CandidateFiles {
		score, err := scoreFile(filepath.Join(opts.TargetPath, filePath), keywords)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Error scoring file %s: %v\n", filePath, err)
			continue
		}

		if score > 0 {
			scoredFiles = append(scoredFiles, FileInfo{
				Path:  filePath,
				Score: score,
			})
		}
	}

	// Sort files by score (highest first)
	sortFilesByScore(scoredFiles)

	// Limit the number of files to return
	maxFiles := opts.MaxFilesToCheck
	if maxFiles > len(scoredFiles) {
		maxFiles = len(scoredFiles)
	}

	return scoredFiles[:maxFiles], nil
}

// extractKeywords extracts meaningful keywords from a query
func extractKeywords(query string) []string {
	// Split the query into words
	words := strings.Fields(strings.ToLower(query))

	// Filter out common words and very short words
	var keywords []string
	for _, word := range words {
		// Clean the word of punctuation
		word = strings.Map(func(r rune) rune {
			if unicode.IsPunct(r) {
				return -1 // Remove punctuation
			}
			return r
		}, word)

		if len(word) < 3 {
			continue // Skip very short words
		}

		// Skip common words
		if isCommonWord(word) {
			continue
		}

		keywords = append(keywords, word)
	}

	return keywords
}

// Common English words to filter out
var commonWords = map[string]bool{
	"the": true, "and": true, "for": true, "this": true, "that": true,
	"are": true, "with": true, "what": true, "from": true, "how": true,
	"where": true, "when": true, "who": true, "why": true, "which": true,
}

// isCommonWord checks if a word is a common word
func isCommonWord(word string) bool {
	return commonWords[word]
}

// scoreFile scores a file based on how well it matches keywords
func scoreFile(filePath string, keywords []string) (float64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	var score float64
	lineNum := 0

	// Read through the file line by line
	for scanner.Scan() {
		lineNum++
		line := strings.ToLower(scanner.Text())

		// Check for each keyword
		for _, keyword := range keywords {
			if strings.Contains(line, keyword) {
				// Score based on keyword occurrence and line position
				// Higher scores for matches near the beginning of the file
				score += 1.0 / (0.1 + float64(lineNum)/100.0)
			}
		}

		// Optional: stop after a certain number of lines to improve performance
		if lineNum > 1000 {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return score, nil
}

// sortFilesByScore sorts files by score in descending order
func sortFilesByScore(files []FileInfo) {
	// Sort files by score (highest first)
	// This is a placeholder for a more sophisticated sorting algorithm
	for i := 0; i < len(files); i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i].Score < files[j].Score {
				files[i], files[j] = files[j], files[i]
			}
		}
	}
}

// ExtractQueryKeyword attempts to extract a single representative keyword from the query.
func ExtractQueryKeyword(query string) string {
	keywords := extractKeywords(query) // Reuse existing keyword extraction
	if len(keywords) == 0 {
		return "query" // Fallback keyword
	}

	// Simple heuristic: pick the longest keyword
	longestKeyword := keywords[0]
	for _, kw := range keywords {
		if len(kw) > len(longestKeyword) {
			longestKeyword = kw
		}
	}
	return longestKeyword
}
