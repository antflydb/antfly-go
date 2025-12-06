package docsaf

import (
	"context"
)

// DocumentSection represents a generic document section extracted from content.
// It contains the content, metadata, and type information needed to index
// the section in Antfly.
type DocumentSection struct {
	ID          string         // Unique ID for the section (generated from path + identifier)
	FilePath    string         // Source path (relative path or URL path)
	Title       string         // Section title (from heading or frontmatter)
	Content     string         // Section content (markdown/text)
	Type        string         // Document type (markdown_section, mdx_section, openapi_path, etc.)
	URL         string         // URL to the document section (base URL + path + anchor)
	SectionPath []string       // Heading hierarchy path (e.g., ["Getting Started", "Installation", "Prerequisites"])
	Metadata    map[string]any // Additional type-specific metadata
}

// ToDocument converts a DocumentSection to a document map suitable for
// storage in Antfly.
func (ds *DocumentSection) ToDocument() map[string]any {
	doc := map[string]any{
		"id":        ds.ID,
		"file_path": ds.FilePath,
		"title":     ds.Title,
		"content":   ds.Content,
		"_type":     ds.Type,
		"metadata":  ds.Metadata,
	}
	if ds.URL != "" {
		doc["url"] = ds.URL
	}
	if len(ds.SectionPath) > 0 {
		doc["section_path"] = ds.SectionPath
	}
	return doc
}

// ContentItem represents a single piece of content from any source (filesystem, web, etc.)
type ContentItem struct {
	// Path is the relative path or URL path for the content
	Path string

	// SourceURL is the full URL for web sources (empty for filesystem sources)
	SourceURL string

	// Content is the raw content bytes
	Content []byte

	// ContentType is the MIME type (e.g., "text/html", "application/pdf")
	ContentType string

	// Metadata contains source-specific metadata (HTTP headers, file info, etc.)
	Metadata map[string]any
}

// ContentSource represents a source of documents that can be traversed.
// Implementations include filesystem directories and web crawlers.
type ContentSource interface {
	// Type returns the source type identifier (e.g., "filesystem", "web")
	Type() string

	// BaseURL returns the base URL for generating document links
	BaseURL() string

	// Traverse iterates over all content items from the source.
	// It returns a channel of ContentItems and a channel for errors.
	// The implementation should close both channels when done.
	Traverse(ctx context.Context) (<-chan ContentItem, <-chan error)
}

// ContentProcessor processes content bytes into document sections.
// It works with raw bytes, making it suitable for both filesystem and web sources.
type ContentProcessor interface {
	// CanProcess returns true if this processor can handle the given content.
	// contentType is the MIME type (may be empty)
	// path is the file path or URL path
	CanProcess(contentType, path string) bool

	// Process processes content bytes and returns document sections.
	// path: relative path or URL path for the content
	// sourceURL: the original URL (for web) or empty (for filesystem)
	// baseURL: the base URL for generating links
	// content: raw bytes to process
	Process(path, sourceURL, baseURL string, content []byte) ([]DocumentSection, error)
}

// ProcessorRegistry manages a collection of ContentProcessors.
type ProcessorRegistry interface {
	// Register adds a processor to the registry.
	Register(processor ContentProcessor)

	// GetProcessor returns the first processor that can handle the given content.
	// Returns nil if no processor can handle the content.
	GetProcessor(contentType, path string) ContentProcessor

	// Processors returns all registered processors.
	Processors() []ContentProcessor
}
