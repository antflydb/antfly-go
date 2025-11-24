package docsaf

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// TraverserConfig holds configuration for a Traverser.
type TraverserConfig struct {
	// BaseDir is the base directory to traverse
	BaseDir string

	// BaseURL is the base URL for generating document links (optional).
	// If set, URLs will be generated as: BaseURL + "/" + relPath + "#" + slug
	// Example: "https://docs.example.com" -> "https://docs.example.com/guide.md#section"
	BaseURL string

	// IncludePatterns is a list of glob patterns to include.
	// Files matching any include pattern will be processed.
	// If empty, all files are included (subject to exclude patterns).
	// Supports ** wildcards for recursive matching.
	IncludePatterns []string

	// ExcludePatterns is a list of glob patterns to exclude.
	// Files matching any exclude pattern will be skipped.
	// Default excludes are: .git/**
	// Supports ** wildcards for recursive matching.
	ExcludePatterns []string

	// Registry is the processor registry to use for finding file processors.
	// If nil, DefaultRegistry() will be used.
	Registry ProcessorRegistry
}

// Traverser walks a directory tree and processes files using registered processors.
// It supports include/exclude patterns with wildcard matching.
type Traverser struct {
	config TraverserConfig
}

// NewTraverser creates a new Traverser with the given configuration.
func NewTraverser(config TraverserConfig) *Traverser {
	// Use default registry if none provided
	if config.Registry == nil {
		config.Registry = DefaultRegistry()
	}

	// Add default excludes if not already present
	defaultExcludes := []string{".git/**"}
	config.ExcludePatterns = append(defaultExcludes, config.ExcludePatterns...)

	return &Traverser{config: config}
}

// Traverse walks the directory tree and processes all matching files.
// Returns a slice of all extracted DocumentSections.
func (t *Traverser) Traverse() ([]DocumentSection, error) {
	var allSections []DocumentSection

	err := filepath.Walk(t.config.BaseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path for pattern matching
		relPath, err := filepath.Rel(t.config.BaseDir, path)
		if err != nil {
			relPath = path
		}

		// Check exclude patterns first
		for _, pattern := range t.config.ExcludePatterns {
			matched, err := doublestar.Match(pattern, relPath)
			if err != nil {
				log.Printf("Warning: Invalid exclude pattern %s: %v", pattern, err)
				continue
			}
			if matched {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// If include patterns are specified, check them
		if len(t.config.IncludePatterns) > 0 {
			included := false
			for _, pattern := range t.config.IncludePatterns {
				matched, err := doublestar.Match(pattern, relPath)
				if err != nil {
					log.Printf("Warning: Invalid include pattern %s: %v", pattern, err)
					continue
				}
				if matched {
					included = true
					break
				}
			}
			if !included {
				if info.IsDir() {
					// Check if this directory could contain matching files
					// If the pattern is like "**/content/**", we need to traverse into subdirs
					couldContainMatches := false
					for _, pattern := range t.config.IncludePatterns {
						if strings.Contains(pattern, "**") {
							couldContainMatches = true
							break
						}
					}
					if !couldContainMatches {
						return filepath.SkipDir
					}
				}
				return nil
			}
		}

		if info.IsDir() {
			return nil
		}

		// Find appropriate processor and process file
		processor := t.config.Registry.GetProcessor(path)
		if processor != nil {
			sections, err := processor.ProcessFile(path, t.config.BaseDir, t.config.BaseURL)
			if err != nil {
				log.Printf("Warning: Failed to process %s: %v", path, err)
				return nil // Continue with other files
			}
			allSections = append(allSections, sections...)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to traverse directory: %w", err)
	}

	return allSections, nil
}

// BaseDir returns the base directory being traversed.
func (t *Traverser) BaseDir() string {
	return t.config.BaseDir
}

// IncludePatterns returns the include patterns being used.
func (t *Traverser) IncludePatterns() []string {
	return t.config.IncludePatterns
}

// ExcludePatterns returns the exclude patterns being used.
func (t *Traverser) ExcludePatterns() []string {
	return t.config.ExcludePatterns
}
