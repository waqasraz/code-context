package output

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GenerateMarkdown creates the final Markdown output file
func GenerateMarkdown(
	outputPath string,
	query string,
	targetPath string,
	showTree bool,
	treeString string,
	summaries map[string]string,
	multiService bool,
) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Write the header
	fmt.Fprintln(file, "# Code Context Summary")
	fmt.Fprintln(file)
	fmt.Fprintf(file, "**Query:** %s\n", query)
	fmt.Fprintf(file, "**Target Directory:** %s\n", targetPath)
	fmt.Fprintln(file)
	fmt.Fprintln(file, "---")
	fmt.Fprintln(file)

	// If multi-service, organize by immediate subdirectories
	if multiService {
		generateMultiServiceOutput(file, targetPath, showTree, treeString, summaries)
	} else {
		generateSingleServiceOutput(file, showTree, treeString, summaries)
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
