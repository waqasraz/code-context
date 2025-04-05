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
	"**/.*/**", // Any folder starting with a dot (hidden folders)
	"**/node_modules/**",
	"**/.git/**",
	"**/.idea/**",
	"**/vendor/**",
	"**/dist/**",
	"**/build/**",
	"**/.venv/**",
	"**/venv/**",       // Python virtual environments
	"**/env/**",        // Python virtual environments
	"**/*.egg-info/**", // Python package metadata
	"**/__pycache__/**",
	"**/*.pyc",          // Python compiled files
	"**/*.pyo",          // Python optimized files
	"**/*.spec.ts",      // Test files
	"**/*.test.ts",      // Test files
	"**/*.spec.js",      // Test files
	"**/*.test.js",      // Test files
	"**/__mocks__/**",   // Mock directories
	"**/coverage/**",    // Coverage reports
	"**/*.json",         // Data files
	"**/*.yaml",         // Data files
	"**/*.yml",          // Data files
	"**/*.xml",          // Data files
	"**/*.csv",          // Data files
	"**/*.toml",         // Data files
	"**/*.ini",          // Data files
	"**/*.min.css",      // Minified CSS files
	"**/*.min.js",       // Minified JS files
	"**/*.css.map",      // CSS source maps
	"**/*.js.map",       // JS source maps
	"**/*.map",          // All source maps
	"**/wwwroot/lib/**", // Web libraries in ASP.NET projects
	"**/wwwroot/css/**", // CSS files in ASP.NET projects
	"**/wwwroot/js/**",  // JS files in ASP.NET projects
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
	"**/.env", // Environment files
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

		// Create a filesystem to walk
		fsys := os.DirFS(opts.TargetPath)

		// Walk the file system using filepath.WalkDir instead of doublestar.Walk
		err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// Send the error and continue walking
				out <- Result{Path: path, Err: err}
				return nil
			}

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

				// Use doublestar.Match for globbing
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
