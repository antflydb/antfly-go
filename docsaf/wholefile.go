package docsaf

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WholeFileProcessor processes files by reading their entire content without
// any chunking or sectioning. This is useful when you want Antfly's internal
// chunking (e.g., Termite) to handle document segmentation rather than
// pre-chunking documents before indexing.
type WholeFileProcessor struct{}

// CanProcess returns true for common text-based file types.
// Supports: .md, .mdx, .txt, .yaml, .yml, .json, .rst, .adoc
func (wfp *WholeFileProcessor) CanProcess(filePath string) bool {
	lower := strings.ToLower(filePath)
	supportedExtensions := []string{
		".md", ".mdx", ".txt", ".yaml", ".yml",
		".json", ".rst", ".adoc",
	}
	for _, ext := range supportedExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// ProcessFile reads an entire file and returns a single DocumentSection
// containing the complete file content without any processing or chunking.
func (wfp *WholeFileProcessor) ProcessFile(filePath, baseDir, baseURL string) ([]DocumentSection, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Convert paths to absolute for correct relative path calculation
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		absBaseDir = baseDir
	}

	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		absFilePath = filePath
	}

	relPath, _ := filepath.Rel(absBaseDir, absFilePath)

	// Use filename as title
	title := filepath.Base(filePath)

	// Generate URL if baseURL is provided
	url := ""
	if baseURL != "" {
		// Remove file extension for cleaner URLs
		cleanPath := strings.TrimSuffix(relPath, filepath.Ext(relPath))
		url = baseURL + "/" + cleanPath
	}

	// Create metadata with file extension info
	ext := strings.TrimPrefix(filepath.Ext(filePath), ".")
	metadata := map[string]any{
		"file_extension": ext,
		"whole_file":     true,
	}

	// Generate unique ID based on file path
	docID := wfpGenerateID(relPath)

	section := DocumentSection{
		ID:       docID,
		FilePath: relPath,
		Title:    title,
		Content:  string(content),
		Type:     "file",
		URL:      url,
		Metadata: metadata,
	}

	return []DocumentSection{section}, nil
}

// wfpGenerateID creates a unique ID for a file using SHA-256 hash.
// The ID format is: doc_<hash(filePath)[:16]>
func wfpGenerateID(filePath string) string {
	hasher := sha256.New()
	hasher.Write([]byte(filePath))
	hash := hex.EncodeToString(hasher.Sum(nil))
	return "doc_" + hash[:16]
}
