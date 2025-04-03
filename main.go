package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/your-username/code-context/internal/tree"   // Import the tree package
	"github.com/your-username/code-context/internal/walker" // Import the walker package
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
	outputFile := flag.String("o", "CODE_CONTEXT_SUMMARY.md", "Specify the output Markdown file name.")
	outputFileLong := flag.String("output", "CODE_CONTEXT_SUMMARY.md", "Specify the output Markdown file name (long form).")
	llmApiKey := flag.String("llm-api-key", "", "API key for the LLM service (or use LLM_API_KEY env var).")
	llmEndpoint := flag.String("llm-endpoint", "", "Endpoint for the LLM service (or use LLM_ENDPOINT env var).")
	var ignorePatterns stringSlice
	flag.Var(&ignorePatterns, "ignore", "Glob patterns for files/directories to ignore (repeatable).")
	showTree := flag.Bool("show-tree", false, "Include a directory tree structure in the output.")

	// --- Parse Flags ---
	flag.Parse()

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
	// If the long form is set but the short form isn't (at default), use the long form value.
	// This prioritizes the long form if both are somehow set, though flag package usually handles this.
	if *multiServiceLong && !*multiService {
		*multiService = true
	}
	if *outputFileLong != "CODE_CONTEXT_SUMMARY.md" && *outputFile == "CODE_CONTEXT_SUMMARY.md" {
		*outputFile = *outputFileLong
	}

	// --- TODO: Get LLM Config from Environment Variables if flags are not set ---
	if *llmApiKey == "" {
		*llmApiKey = os.Getenv("LLM_API_KEY")
		// Optionally check if it's still empty and handle error/warning
	}
	if *llmEndpoint == "" {
		*llmEndpoint = os.Getenv("LLM_ENDPOINT")
		// Optionally check if it's still empty and handle error/warning
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

	// --- Print Parsed Config (for debugging) ---
	fmt.Println("--- Configuration ---")
	fmt.Printf("Target Path: %s\n", absTargetPath)
	fmt.Printf("Query: %s\n", query)
	fmt.Printf("Multi-Service Mode: %t\n", *multiService)
	fmt.Printf("Output File: %s\n", *outputFile)
	fmt.Printf("Ignore Patterns: %v\n", ignorePatterns)
	fmt.Printf("Show Tree: %t\n", *showTree)
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

	var treeString string // Variable to hold the generated tree
	if *showTree {
		fmt.Println("\nGenerating directory tree...")
		// Pass the base path and the collected files/dirs to the tree generator
		// TODO: Update tree.Generate to accept relevant files later for marking
		treeString = tree.Generate(absTargetPath, foundFiles, foundDirs)
		fmt.Println("--- Generated Tree ---") // Temporary print
		fmt.Println(treeString)               // Temporary print
		fmt.Println("----------------------") // Temporary print
	}

	// --- TODO: Relevance Identification (Pre-LLM) ---
	fmt.Println("\nIdentifying relevant files (placeholder)...")
	// Using all found files as relevant for now
	relevantFiles := foundFiles
	fmt.Printf("Identified %d potentially relevant files (currently all files found).\n", len(relevantFiles))

	// --- TODO: LLM Interaction (Placeholder/Future) ---
	fmt.Println("\nGenerating summaries via LLM (placeholder)...")
	// Placeholder summaries
	summaries := make(map[string]string)
	for _, file := range relevantFiles {
		summaries[file] = fmt.Sprintf("Placeholder summary for %s based on query: '%s'", file, query)
	}

	// --- TODO: Output Generation (Markdown) ---
	fmt.Println("\nGenerating Markdown output (placeholder)...")
	// output.GenerateMarkdown(*outputFile, query, absTargetPath, *showTree, treeString, summaries, *multiService)

	fmt.Println("\nAnalysis complete (placeholder steps). Output will be in", *outputFile)
}
