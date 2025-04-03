package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/waqasraz/code-context/internal/llm"
	"github.com/waqasraz/code-context/internal/output"
	"github.com/waqasraz/code-context/internal/relevance"
	"github.com/waqasraz/code-context/internal/tree"
	"github.com/waqasraz/code-context/internal/walker"
)

// stringSlice is a custom type to handle repeatable flags
type stringSlice []string

func (i *stringSlice) String() string {
	return fmt.Sprintf("%v", *i)
}

func (i *stringSlice) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	// --- Define Flags ---
	multiService := flag.Bool("m", false, "Treat immediate subdirectories as distinct services.")
	multiServiceLong := flag.Bool("multi-service", false, "Treat immediate subdirectories as distinct services (long form).")
	// Define these flags for documentation in --help, but we'll handle them manually
	_ = flag.String("o", "CODE_CONTEXT_SUMMARY.md", "Specify the output Markdown file name.")
	_ = flag.String("output", "CODE_CONTEXT_SUMMARY.md", "Specify the output Markdown file name (long form).")
	llmApiKey := flag.String("llm-api-key", "", "API key for the LLM service (or use LLM_API_KEY env var).")
	llmEndpoint := flag.String("llm-endpoint", "", "Endpoint for the LLM service (or use LLM_ENDPOINT env var).")
	llmProvider := flag.String("llm-provider", "", "LLM provider to use: 'openai', 'local', 'unified', or empty for placeholder.")
	llmModel := flag.String("llm-model", "", "Model name to use with the LLM provider.")
	useEmbeddings := flag.Bool("use-embeddings", false, "Use embedding-based relevance detection for more accurate results.")
	useHybridSearch := flag.Bool("use-hybrid", true, "Use hybrid approach combining embeddings with traditional relevance metrics.")
	embeddingModel := flag.String("embedding-model", "nomic-embed-text", "Model to use for embeddings when --use-embeddings is enabled.")
	embeddingURL := flag.String("embedding-url", "http://localhost:11434/api/embeddings", "URL for embedding API when --use-embeddings is enabled.")
	var llmHeaders stringSlice
	flag.Var(&llmHeaders, "llm-header", "Additional headers for LLM API requests in format 'key:value' (repeatable).")
	var ignorePatterns stringSlice
	flag.Var(&ignorePatterns, "ignore", "Glob patterns for files/directories to ignore (repeatable).")
	// Define show-tree flag for documentation, but handle it manually
	_ = flag.Bool("show-tree", false, "Include a directory tree structure in the output.")

	// --- Parse Flags ---
	flag.Parse()

	// Debug: Show raw command line arguments
	fmt.Println("DEBUG: Command line arguments:")
	for i, arg := range os.Args {
		fmt.Printf("  [%d] %s\n", i, arg)
	}

	// Debug: Show the parsed flag values
	fmt.Println("DEBUG: Flag values after parsing:")
	fmt.Printf("  -o: %q\n", "CODE_CONTEXT_SUMMARY.md")       // Default value
	fmt.Printf("  --output: %q\n", "CODE_CONTEXT_SUMMARY.md") // Default value
	fmt.Printf("  --show-tree: %t\n", false)                  // Default value
	fmt.Printf("  --llm-provider: %q\n", *llmProvider)
	fmt.Printf("  --llm-model: %q\n", *llmModel)

	// --- Get Mandatory Arguments ---
	args := flag.Args()
	if len(args) < 2 {
		fmt.Println("Error: TARGET_PATH and QUERY arguments are mandatory.")
		fmt.Println("Usage: code-context [options] TARGET_PATH QUERY")
		flag.PrintDefaults()
		os.Exit(1)
	}
	targetPath := args[0]
	query := args[1]

	// --- Handle Long/Short Flag Aliases ---
	if *multiServiceLong && !*multiService {
		*multiService = true
	}

	// Debug the multi-service flag
	fmt.Printf("DEBUG: Flag values: multiService=%t, multiServiceLong=%t\n", *multiService, *multiServiceLong)

	// Manual detection of -m flag
	for _, arg := range os.Args {
		if arg == "-m" || arg == "--multi-service" {
			*multiService = true
			fmt.Println("DEBUG: Found -m or --multi-service flag, setting multiService=true")
			break
		}
	}

	// Manual detection of -o flag
	outputFileName := "CODE_CONTEXT_SUMMARY.md" // Default value

	// Search through os.Args manually for -o or --output
	for i, arg := range os.Args {
		if (arg == "-o" || arg == "--output") && i+1 < len(os.Args) {
			outputFileName = os.Args[i+1]
			break
		}
	}

	// Manual detection of --show-tree flag
	showTreeFlag := false
	for _, arg := range os.Args {
		if arg == "--show-tree" {
			showTreeFlag = true
			break
		}
	}

	// --- Get LLM Config from Environment Variables if flags are not set ---
	if *llmApiKey == "" {
		*llmApiKey = os.Getenv("LLM_API_KEY")
	}
	if *llmEndpoint == "" {
		*llmEndpoint = os.Getenv("LLM_ENDPOINT")
	}

	// Try to detect --llm-provider and --llm-model if flag package didn't catch them
	if *llmProvider == "" {
		// Check for different formats: --llm-provider=value or --llm-provider value
		for i, arg := range os.Args {
			if strings.HasPrefix(arg, "--llm-provider=") {
				*llmProvider = strings.TrimPrefix(arg, "--llm-provider=")
				fmt.Println("DEBUG: Found --llm-provider= syntax:", *llmProvider)
				break
			} else if (arg == "--llm-provider" || arg == "-llm-provider") && i+1 < len(os.Args) {
				*llmProvider = os.Args[i+1]
				fmt.Println("DEBUG: Found --llm-provider with space syntax:", *llmProvider)
				break
			}
		}
		// If still empty, try environment
		if *llmProvider == "" {
			*llmProvider = os.Getenv("LLM_PROVIDER")
		}
	}

	if *llmModel == "" {
		// Check for different formats: --llm-model=value or --llm-model value
		for i, arg := range os.Args {
			if strings.HasPrefix(arg, "--llm-model=") {
				*llmModel = strings.TrimPrefix(arg, "--llm-model=")
				fmt.Println("DEBUG: Found --llm-model= syntax:", *llmModel)
				break
			} else if (arg == "--llm-model" || arg == "-llm-model") && i+1 < len(os.Args) {
				*llmModel = os.Args[i+1]
				fmt.Println("DEBUG: Found --llm-model with space syntax:", *llmModel)
				break
			}
		}
		// If still empty, try environment
		if *llmModel == "" {
			*llmModel = os.Getenv("LLM_MODEL")
		}
	}

	// --- Validate Target Path ---
	absTargetPath, err := filepath.Abs(targetPath)
	if err != nil {
		fmt.Printf("Error getting absolute path for %s: %v\n", targetPath, err)
		os.Exit(1)
	}
	if _, err := os.Stat(absTargetPath); os.IsNotExist(err) {
		fmt.Printf("Error: Target path %s does not exist.\n", absTargetPath)
		os.Exit(1)
	}

	// --- Print Parsed Config ---
	fmt.Println("--- Configuration ---")
	fmt.Printf("Target Path: %s\n", absTargetPath)
	fmt.Printf("Query: %s\n", query)
	fmt.Printf("Multi-Service Mode: %t\n", *multiService)
	fmt.Printf("Output File: %s\n", outputFileName)
	fmt.Printf("Ignore Patterns: %v\n", ignorePatterns)
	fmt.Printf("Show Tree: %t\n", showTreeFlag)
	fmt.Printf("LLM Provider: %s\n", *llmProvider)
	fmt.Printf("LLM Model: %s\n", *llmModel)
	fmt.Printf("LLM API Key Set: %t\n", *llmApiKey != "")
	fmt.Printf("LLM Endpoint Set: %t\n", *llmEndpoint != "")
	fmt.Printf("Using Embeddings: %t\n", *useEmbeddings)
	fmt.Printf("Using Hybrid Search: %t\n", *useHybridSearch)
	if *useEmbeddings || *useHybridSearch {
		fmt.Printf("Embedding Model: %s\n", *embeddingModel)
		fmt.Printf("Embedding URL: %s\n", *embeddingURL)
	}
	fmt.Println("---------------------")

	// --- Core Logic ---
	fmt.Println("\nStarting analysis...")

	// Configure the walker
	walkerOpts := walker.Options{
		TargetPath:     absTargetPath,
		IgnorePatterns: ignorePatterns, // Pass user-provided ignores
	}

	fmt.Println("\nWalking directory...")
	var foundFiles []string
	var foundDirs []string

	resultsChan := walker.Walk(walkerOpts)
	for result := range resultsChan {
		if result.Err != nil {
			fmt.Fprintf(os.Stderr, "Error during walk: %v\n", result.Err)
			continue // Or handle error more robustly
		}
		// Ignore the root '.' reported by walker itself if present
		if result.Path == "." {
			continue
		}
		if result.IsDir {
			foundDirs = append(foundDirs, result.Path)
		} else {
			foundFiles = append(foundFiles, result.Path)
		}
	}

	fmt.Printf("Found %d files and %d directories after filtering.\n", len(foundFiles), len(foundDirs))

	// Manual detection of --use-embeddings flag if not set via flag package
	for _, arg := range os.Args {
		if arg == "--use-embeddings" {
			*useEmbeddings = true
			fmt.Println("DEBUG: Found --use-embeddings flag, enabling embedding-based relevance detection")
			break
		}
	}

	// Manual detection of --use-hybrid flag
	for _, arg := range os.Args {
		if arg == "--use-hybrid" {
			*useHybridSearch = true
			fmt.Println("DEBUG: Found --use-hybrid flag, enabling hybrid relevance detection")
		} else if arg == "--no-hybrid" {
			*useHybridSearch = false
			fmt.Println("DEBUG: Found --no-hybrid flag, disabling hybrid relevance detection")
			break
		}
	}

	// Manual detection of --embedding-model flag
	for i, arg := range os.Args {
		if strings.HasPrefix(arg, "--embedding-model=") {
			*embeddingModel = strings.TrimPrefix(arg, "--embedding-model=")
			fmt.Printf("DEBUG: Found --embedding-model= syntax: %s\n", *embeddingModel)
			break
		} else if arg == "--embedding-model" && i+1 < len(os.Args) {
			*embeddingModel = os.Args[i+1]
			fmt.Printf("DEBUG: Found --embedding-model with space syntax: %s\n", *embeddingModel)
			break
		}
	}

	// Manual detection of --embedding-url flag
	for i, arg := range os.Args {
		if strings.HasPrefix(arg, "--embedding-url=") {
			*embeddingURL = strings.TrimPrefix(arg, "--embedding-url=")
			fmt.Printf("DEBUG: Found --embedding-url= syntax: %s\n", *embeddingURL)
			break
		} else if arg == "--embedding-url" && i+1 < len(os.Args) {
			*embeddingURL = os.Args[i+1]
			fmt.Printf("DEBUG: Found --embedding-url with space syntax: %s\n", *embeddingURL)
			break
		}
	}

	// --- Multi-Service Mode Preparation ---
	var serviceDirs []string
	if *multiService {
		fmt.Println("\nMulti-service mode detected. Identifying services...")

		// Find immediate subdirectories (services)
		for _, dir := range foundDirs {
			// Count path separators to ensure it's an immediate subdirectory
			if strings.Count(dir, string(os.PathSeparator)) == 0 &&
				!strings.Contains(dir, "/") {
				serviceDirs = append(serviceDirs, dir)
			}
		}

		// Print service list
		fmt.Printf("Found %d services to process:\n", len(serviceDirs))
		for i, service := range serviceDirs {
			fmt.Printf("  %d. %s\n", i+1, service)
		}
	}

	// --- Relevance Identification ---
	fmt.Println("\nIdentifying relevant files...")

	var relevantFileInfos []relevance.FileInfo
	var relevanceErr error

	// Configure embedding options if using embeddings or hybrid search
	embeddingOpts := relevance.EmbeddingOptions{
		Query:           query,
		TargetPath:      absTargetPath,
		CandidateFiles:  foundFiles,
		MaxFilesToCheck: 20, // Consider top 20 most relevant files
		EmbeddingModel:  *embeddingModel,
		EmbeddingURL:    *embeddingURL,
	}

	if *useHybridSearch {
		// Use hybrid approach (embeddings + keywords + path relevance)
		fmt.Println("Using hybrid relevance detection (embeddings + keywords + path relevance)...")
		relevantFileInfos, relevanceErr = relevance.IdentifyRelevantFilesWithHybridApproach(embeddingOpts)
		if relevanceErr != nil {
			fmt.Printf("Error with hybrid relevance detection: %v\n", relevanceErr)
			fmt.Println("Falling back to keyword-based relevance detection...")

			// Fall back to keyword-based method
			relevanceOpts := relevance.Options{
				Query:           query,
				TargetPath:      absTargetPath,
				CandidateFiles:  foundFiles,
				MaxFilesToCheck: 20, // Consider top 20 most relevant files
			}

			relevantFileInfos, relevanceErr = relevance.IdentifyRelevantFiles(relevanceOpts)
			if relevanceErr != nil {
				fmt.Printf("Error identifying relevant files: %v\n", relevanceErr)
				os.Exit(1)
			}
		}
	} else if *useEmbeddings {
		// Use embedding-based relevance detection
		fmt.Println("Using embedding-based relevance detection for more accurate results...")
		relevantFileInfos, relevanceErr = relevance.IdentifyRelevantFilesWithEmbeddings(embeddingOpts)
		if relevanceErr != nil {
			fmt.Printf("Error with embedding-based relevance detection: %v\n", relevanceErr)
			fmt.Println("Falling back to keyword-based relevance detection...")

			// Fall back to keyword-based method
			relevanceOpts := relevance.Options{
				Query:           query,
				TargetPath:      absTargetPath,
				CandidateFiles:  foundFiles,
				MaxFilesToCheck: 20, // Consider top 20 most relevant files
			}

			relevantFileInfos, relevanceErr = relevance.IdentifyRelevantFiles(relevanceOpts)
			if relevanceErr != nil {
				fmt.Printf("Error identifying relevant files: %v\n", relevanceErr)
				os.Exit(1)
			}
		}
	} else {
		// Use the original keyword-based method
		relevanceOpts := relevance.Options{
			Query:           query,
			TargetPath:      absTargetPath,
			CandidateFiles:  foundFiles,
			MaxFilesToCheck: 20, // Consider top 20 most relevant files
		}

		relevantFileInfos, relevanceErr = relevance.IdentifyRelevantFiles(relevanceOpts)
		if relevanceErr != nil {
			fmt.Printf("Error identifying relevant files: %v\n", relevanceErr)
			os.Exit(1)
		}
	}

	// Extract just the paths from the FileInfo objects
	var relevantFiles []string
	for _, fileInfo := range relevantFileInfos {
		relevantFiles = append(relevantFiles, fileInfo.Path)
		fmt.Printf("Relevant file: %s (score: %.2f)\n", fileInfo.Path, fileInfo.Score)
	}

	fmt.Printf("Identified %d relevant files out of %d total files.\n", len(relevantFiles), len(foundFiles))

	// Generate tree (after identifying relevant files)
	var treeString string // Variable to hold the generated tree
	if showTreeFlag {
		fmt.Println("\nGenerating directory tree...")
		// Pass the base path, all files, dirs, and the relevant files to mark them
		treeString = tree.Generate(absTargetPath, foundFiles, foundDirs, relevantFiles)
	}

	// --- LLM Interaction ---
	fmt.Println("\nGenerating summaries via LLM...")

	// Parse headers if provided
	headers := make(map[string]string)
	for _, header := range llmHeaders {
		parts := strings.SplitN(header, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			headers[key] = value
		} else {
			fmt.Fprintf(os.Stderr, "Warning: invalid header format (expected 'key:value'): %s\n", header)
		}
	}

	// Configure the LLM provider
	llmConfig := llm.Config{
		APIKey:    *llmApiKey,
		Endpoint:  *llmEndpoint,
		ModelName: *llmModel,
		Provider:  *llmProvider,
		Headers:   headers,
	}

	provider, err := llm.NewProvider(llmConfig)
	if err != nil {
		fmt.Printf("Error creating LLM provider: %v\n", err)
		os.Exit(1)
	}

	// --- Process Services (Multi-Service Mode) or Files (Single-Service Mode) ---
	if *multiService && len(serviceDirs) > 0 {
		// Multi-service mode
		fmt.Println("\nProcessing services one by one...")

		// Map to store summaries for all services
		allSummaries := make(map[string]map[string]string)

		// Process each service
		for i, service := range serviceDirs {
			fmt.Printf("\nProcessing service %d/%d: %s\n", i+1, len(serviceDirs), service)

			// Filter files for this service
			var serviceFiles []string
			for _, file := range relevantFiles {
				if strings.HasPrefix(file, service+string(os.PathSeparator)) ||
					strings.HasPrefix(file, service+"/") {
					serviceFiles = append(serviceFiles, file)
				}
			}

			if len(serviceFiles) == 0 {
				fmt.Printf("No relevant files found for service: %s, skipping...\n", service)
				continue
			}

			fmt.Printf("Found %d relevant files for service: %s\n", len(serviceFiles), service)

			// Generate summaries for this service's files
			serviceSummaries, err := llm.GenerateSummaries(provider, query, absTargetPath, serviceFiles)
			if err != nil {
				fmt.Printf("Error generating summaries for service %s: %v\n", service, err)
				continue // Continue with the next service
			}

			fmt.Printf("Generated %d summaries for service: %s\n", len(serviceSummaries), service)
			allSummaries[service] = serviceSummaries
		}

		// Merge all summaries for output generation
		mergedSummaries := make(map[string]string)
		for _, serviceSummary := range allSummaries {
			for path, summary := range serviceSummary {
				mergedSummaries[path] = summary
			}
		}

		// --- Output Generation (Markdown) ---
		fmt.Println("\nGenerating Markdown output...")
		err = output.GenerateMarkdown(outputFileName, query, absTargetPath, showTreeFlag, treeString, mergedSummaries, *multiService)
		if err != nil {
			fmt.Printf("Error generating Markdown: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Single-service mode (or multi-service mode with no services found)
		// Generate summaries for relevant files
		summaries, err := llm.GenerateSummaries(provider, query, absTargetPath, relevantFiles)
		if err != nil {
			fmt.Printf("Error generating summaries: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Generated %d summaries\n", len(summaries))

		// --- Output Generation (Markdown) ---
		fmt.Println("\nGenerating Markdown output...")
		err = output.GenerateMarkdown(outputFileName, query, absTargetPath, showTreeFlag, treeString, summaries, *multiService)
		if err != nil {
			fmt.Printf("Error generating Markdown: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("\nAnalysis complete. Output file saved to", outputFileName)
}
