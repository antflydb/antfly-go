package docsaf

import (
	"context"
)

// ContentItem represents a single piece of content from any source (filesystem, web, etc.)
type ContentItem struct {
	// Path is the relative path or URL path for the content
	Path string

	// SourceURL is the full URL for web sources (empty for filesystem sources)
	SourceURL string

	// Content is the raw content bytes
	Content []byte

	// ContentType is the MIME type (e.g., "text/html", "application/pdf")
	// For filesystem sources, this is detected from the file extension
	ContentType string

	// Metadata contains source-specific metadata (HTTP headers, file info, etc.)
	Metadata map[string]any
}

// ContentSource represents a source of documents that can be traversed.
// Implementations include filesystem directories and web crawlers.
type ContentSource interface {
	// Type returns the source type identifier (e.g., "filesystem", "web")
	Type() string

	// Traverse iterates over all content items from the source.
	// It returns a channel of ContentItems and a channel for errors.
	// The implementation should close both channels when done.
	// The context can be used to cancel the traversal.
	Traverse(ctx context.Context) (<-chan ContentItem, <-chan error)
}

// ContentProcessor processes content bytes into document sections.
// Unlike FileProcessor which reads from disk, ContentProcessor works
// with raw bytes, making it suitable for both filesystem and web sources.
type ContentProcessor interface {
	// CanProcess returns true if this processor can handle the given content.
	// contentType is the MIME type (may be empty for filesystem sources)
	// path is the file path or URL path
	CanProcess(contentType, path string) bool

	// ProcessContent processes content bytes and returns document sections.
	// path: relative path or URL path for the content
	// sourceURL: the original URL (for web) or empty (for filesystem)
	// baseURL: the base URL for generating links
	// content: raw bytes to process
	ProcessContent(path, sourceURL, baseURL string, content []byte) ([]DocumentSection, error)
}

// ContentProcessorRegistry manages a collection of ContentProcessors.
type ContentProcessorRegistry interface {
	// Register adds a processor to the registry.
	Register(processor ContentProcessor)

	// GetProcessor returns the first processor that can handle the given content.
	// Returns nil if no processor can handle the content.
	GetProcessor(contentType, path string) ContentProcessor

	// Processors returns all registered processors.
	Processors() []ContentProcessor
}
