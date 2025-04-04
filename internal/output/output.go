package output

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// GenerateMarkdown generates a Markdown file with the analysis results
func GenerateMarkdown(
	outputFileName string,
	query string,
	basePath string,
	includeTree bool,
	treeString string,
	summaries map[string]string,
) error {
	// Create or truncate the output file
	outputFile, err := os.Create(outputFileName)
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}
	defer outputFile.Close()

	// Start with a header
	fmt.Fprintf(outputFile, "# Code Context Summary\n\n")
	fmt.Fprintf(outputFile, "**Query:** %s\n\n", query)
	fmt.Fprintf(outputFile, "**Target Directory:** %s\n\n", basePath)
	fmt.Fprintf(outputFile, "**Generated on:** %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	// Include directory tree if requested
	if includeTree && treeString != "" {
		fmt.Fprintf(outputFile, "## Directory Structure\n\n")
		fmt.Fprintf(outputFile, "```\n%s\n```\n\n", treeString)
	}

	// Summary of relevant files
	fmt.Fprintf(outputFile, "## Relevant Files Summary\n\n")
	fmt.Fprintf(outputFile, "Found %d relevant files for the query.\n\n", len(summaries))

	// Write each file summary
	fmt.Fprintf(outputFile, "## File Summaries\n\n")

	// Sort file paths for consistent output
	var filePaths []string
	for path := range summaries {
		filePaths = append(filePaths, path)
	}
	sort.Strings(filePaths)

	for _, filePath := range filePaths {
		summary := summaries[filePath]

		// Add a section for each file
		fmt.Fprintf(outputFile, "### %s\n\n", filePath)
		fmt.Fprintf(outputFile, "%s\n\n", summary)

		// Add a line break between file summaries
		fmt.Fprintf(outputFile, "---\n\n")
	}

	return nil
}

// generateSingleServiceOutput creates output for a single service/directory
func generateSingleServiceOutput(
	file *os.File,
	showTree bool,
	treeString string,
	summaries map[string]string,
) {
	// Add tree if requested
	if showTree {
		fmt.Fprintln(file, "## Directory Structure")
		fmt.Fprintln(file)
		fmt.Fprintln(file, treeString)
		fmt.Fprintln(file, "(* Indicates files included in the summaries below)")
		fmt.Fprintln(file)
		fmt.Fprintln(file, "---")
		fmt.Fprintln(file)
	}

	// Write file summaries
	fmt.Fprintln(file, "## File Summaries")
	fmt.Fprintln(file)

	// Get the keys and sort them for consistent output
	var keys []string
	for k := range summaries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Write each summary
	for _, filePath := range keys {
		summary := summaries[filePath]
		fmt.Fprintf(file, "### `%s`\n", filePath)
		fmt.Fprintln(file, summary)
		fmt.Fprintln(file)
	}
}

// generateMultiServiceOutput organizes output by service (immediate subdirectories)
func generateMultiServiceOutput(
	file *os.File,
	targetPath string,
	showTree bool,
	treeString string,
	summaries map[string]string,
) {
	// Add tree if requested - the full tree above all services
	if showTree {
		fmt.Fprintln(file, "## Directory Structure")
		fmt.Fprintln(file)
		fmt.Fprintln(file, treeString)
		fmt.Fprintln(file, "(* Indicates files included in the summaries below)")
		fmt.Fprintln(file)
		fmt.Fprintln(file, "---")
		fmt.Fprintln(file)
	}

	// Group files by immediate subdirectory
	serviceFiles := make(map[string][]string)
	rootFiles := []string{}

	for filePath := range summaries {
		parts := strings.Split(filepath.ToSlash(filePath), "/")
		if len(parts) == 1 {
			// This is a file in the root directory
			rootFiles = append(rootFiles, filePath)
		} else {
			// This is a file in a subdirectory
			service := parts[0]
			serviceFiles[service] = append(serviceFiles[service], filePath)
		}
	}

	// Sort service names for consistent output
	var services []string
	for service := range serviceFiles {
		services = append(services, service)
	}
	sort.Strings(services)

	// Add root files first if any
	if len(rootFiles) > 0 {
		fmt.Fprintln(file, "## Root Directory")
		fmt.Fprintln(file)

		sort.Strings(rootFiles)
		for _, filePath := range rootFiles {
			fmt.Fprintf(file, "### `%s`\n", filePath)
			fmt.Fprintln(file, summaries[filePath])
			fmt.Fprintln(file)
		}
	}

	// Add each service's files
	for _, service := range services {
		fmt.Fprintf(file, "## Service: %s\n", service)
		fmt.Fprintln(file)

		// Sort files within this service
		sort.Strings(serviceFiles[service])
		for _, filePath := range serviceFiles[service] {
			fmt.Fprintf(file, "### `%s`\n", filePath)
			fmt.Fprintln(file, summaries[filePath])
			fmt.Fprintln(file)
		}
	}
}
