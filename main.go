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

	// --- Relevance Identification ---
	fmt.Println("\nIdentifying relevant files...")
	relevanceOpts := relevance.Options{
		Query:           query,
		TargetPath:      absTargetPath,
		CandidateFiles:  foundFiles,
		MaxFilesToCheck: 20, // Consider top 20 most relevant files
	}

	relevantFileInfos, err := relevance.IdentifyRelevantFiles(relevanceOpts)
	if err != nil {
		fmt.Printf("Error identifying relevant files: %v\n", err)
		os.Exit(1)
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

	fmt.Println("\nAnalysis complete. Output file saved to", outputFileName)
}
