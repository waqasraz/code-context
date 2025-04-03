package walker

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// DefaultIgnorePatterns are common patterns to ignore.
var DefaultIgnorePatterns = []string{
	"**/node_modules/**",
	"**/.git/**",
	"**/vendor/**",
	"**/dist/**",
	"**/build/**",
	"**/.venv/**",
	"**/__pycache__/**",
	"**/*.log",
	"**/*.lock",
	"**/*.bin",
	"**/*.exe",
	"**/*.dll",
	"**/*.so",
	"**/*.dylib",
	"**/*.png",
	"**/*.jpg",
	"**/*.jpeg",
	"**/*.gif",
	"**/*.svg",
	"**/*.ico",
	"**/*.pdf",
	"**/*.zip",
	"**/*.tar.gz",
	"**/*.gz",
	"**/*.rar",
	"**/*.7z",
	// Add other common binary/non-source file extensions or build artifacts
}

// Options defines the configuration for the directory walk.
type Options struct {
	TargetPath     string
	IgnorePatterns []string
	// Potentially add .gitignore reading logic here later
}

// Result holds information about a processed file or directory.
type Result struct {
	Path  string
	IsDir bool
	Err   error // Error encountered while accessing this path
}

// Walk traverses the directory structure based on the provided options,
// yielding Result objects for each file/directory encountered after filtering.
func Walk(opts Options) <-chan Result {
	out := make(chan Result)

	go func() {
		defer close(out)

		// Combine default and user-provided ignore patterns
		allIgnores := append([]string{}, DefaultIgnorePatterns...)
		allIgnores = append(allIgnores, opts.IgnorePatterns...)

		// Use doublestar's Walk functionality which respects gitignore style patterns
		fsys := os.DirFS(opts.TargetPath)

		err := doublestar.Walk(fsys, ".", func(path string, d fs.DirEntry) error {
			// Construct the full path relative to the original target path for matching
			fullPath := filepath.Join(opts.TargetPath, path)
			relPath, err := filepath.Rel(opts.TargetPath, fullPath)
			if err != nil {
				// Should generally not happen if logic is correct
				fmt.Fprintf(os.Stderr, "Warning: could not make path relative: %v\n", err)
				relPath = path // Fallback
			}

			// Check against ignore patterns
			for _, pattern := range allIgnores {
				// Ensure patterns use forward slashes for consistency
				matchPattern := filepath.ToSlash(pattern)
				pathToMatch := filepath.ToSlash(relPath)

				// Doublestar expects patterns relative to the Walk root (".")
				// and the path being checked should also be relative to the Walk root.
				// If the pattern contains '/', it's treated as path-based.
				// If not, it matches the basename.

				matched, _ := doublestar.Match(matchPattern, pathToMatch)

				// Also match against the basename for patterns like '*.log'
				if !matched && !strings.Contains(matchPattern, "/") {
					base := filepath.Base(pathToMatch)
					matched, _ = doublestar.Match(matchPattern, base)
				}

				if matched {
					if d.IsDir() {
						// Skip the entire directory if the directory itself matches
						return fs.SkipDir
					} else {
						// Skip the file
						return nil
					}
				}
			}

			// Send the result (relative path)
			out <- Result{Path: relPath, IsDir: d.IsDir()}
			return nil
		})

		if err != nil {
			// Send the walk error itself
			out <- Result{Err: fmt.Errorf("error during directory walk: %w", err)}
		}
	}()

	return out
}
