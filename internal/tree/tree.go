package tree

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// Node represents a node in the directory tree.
type Node struct {
	Name     string
	Path     string // Relative path from the root
	IsDir    bool
	Children []*Node
	Relevant bool // Flag to indicate if this file/dir contains relevant content
}

// Generate creates a string representation of the directory tree.
// It takes the base path, a list of all discovered files, and all discovered dirs.
// TODO: Adapt to accept a list of *relevant* files to mark them in the tree.
func Generate(basePath string, allFiles []string, allDirs []string) string {
	root := &Node{Name: filepath.Base(basePath), Path: ".", IsDir: true}
	nodes := make(map[string]*Node)
	nodes["."] = root

	// Add directory nodes
	for _, dirPath := range allDirs {
		parts := strings.Split(filepath.ToSlash(dirPath), "/")
		parent := root
		for i, part := range parts {
			relChildPath := filepath.Join(strings.Join(parts[:i+1], "/"))

			node, exists := nodes[relChildPath]
			if !exists {
				node = &Node{Name: part, Path: relChildPath, IsDir: true}
				nodes[relChildPath] = node
				parent.Children = append(parent.Children, node)
			}
			parent = node
		}
	}

	// Add file nodes
	for _, filePath := range allFiles {
		parts := strings.Split(filepath.ToSlash(filePath), "/")
		dirPath := "."
		if len(parts) > 1 {
			dirPath = filepath.Join(parts[:len(parts)-1]...)
		}
		parent, ok := nodes[dirPath]
		if !ok {
			// This might happen if a file is in the root or walker didn't report parent dir; create implicitly?
			// For now, let's assume parent dirs are always reported by walker.
			// If not found, attach to root as fallback.
			fmt.Printf("Warning: Parent directory node not found for file %s, attaching to root\n", filePath)
			parent = root
			dirPath = "."
		}
		fileName := parts[len(parts)-1]
		fileNode := &Node{Name: fileName, Path: filePath, IsDir: false}
		// TODO: Mark fileNode.Relevant based on actual relevance check
		nodes[filePath] = fileNode // Add file node to map as well
		parent.Children = append(parent.Children, fileNode)
	}

	// Sort children at each level
	for _, node := range nodes {
		sort.Slice(node.Children, func(i, j int) bool {
			if node.Children[i].IsDir != node.Children[j].IsDir {
				return node.Children[i].IsDir // Directories first
			}
			return node.Children[i].Name < node.Children[j].Name
		})
	}

	var builder strings.Builder
	builder.WriteString("```\n")
	buildTreeString(root, "", true, &builder)
	builder.WriteString("```\n")
	// builder.WriteString("(* Indicates files included in the summaries below)\n") // Add when relevance is implemented

	return builder.String()
}

// buildTreeString recursively builds the string representation.
func buildTreeString(node *Node, prefix string, isLast bool, builder *strings.Builder) {
	marker := "├── "
	if isLast {
		marker = "└── "
	}

	// Don't print the artificial root "." marker itself, start with its children
	if node.Path != "." {
		builder.WriteString(prefix)
		builder.WriteString(marker)
		builder.WriteString(node.Name)
		if node.IsDir {
			builder.WriteString("/")
		}
		// if node.Relevant && !node.IsDir { // Mark relevant files
		// 	builder.WriteString(" (*)")
		// }
		builder.WriteString("\n")
	}

	newPrefix := prefix
	if node.Path != "." { // Only add indentation if we printed the node
		if isLast {
			newPrefix += "    "
		} else {
			newPrefix += "│   "
		}
	}

	for i, child := range node.Children {
		buildTreeString(child, newPrefix, i == len(node.Children)-1, builder)
	}
}
