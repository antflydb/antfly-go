package docsaf

import "time"

// DocumentSection represents a generic document section extracted from a file.
// It contains the content, metadata, and type information needed to index
// the section in Antfly.
type DocumentSection struct {
	ID        string         // Unique ID for the section (generated from file path + identifier)
	FilePath  string         // Source file path (relative to base directory)
	Title     string         // Section title (from heading or frontmatter)
	Content   string         // Section content (markdown/text)
	Type      string         // Document type (markdown_section, mdx_section, openapi_path, etc.)
	Metadata  map[string]any // Additional type-specific metadata
	CreatedAt time.Time      // Timestamp when section was created
}

// ToDocument converts a DocumentSection to a document map suitable for
// storage in Antfly.
func (ds *DocumentSection) ToDocument() map[string]any {
	return map[string]any{
		"id":         ds.ID,
		"file_path":  ds.FilePath,
		"title":      ds.Title,
		"content":    ds.Content,
		"_type":      ds.Type,
		"metadata":   ds.Metadata,
		"created_at": ds.CreatedAt.Format(time.RFC3339),
	}
}

// FileProcessor is the interface that all file processors must implement.
// A FileProcessor is responsible for determining if it can process a file
// and extracting DocumentSections from it.
type FileProcessor interface {
	// CanProcess returns true if this processor can handle the given file path.
	CanProcess(filePath string) bool

	// ProcessFile processes a file and returns a slice of DocumentSections.
	// The baseDir parameter is the base directory used for generating relative paths.
	ProcessFile(filePath, baseDir string) ([]DocumentSection, error)
}

// ProcessorRegistry manages a collection of FileProcessors.
// It allows registering processors and finding the appropriate processor
// for a given file.
type ProcessorRegistry interface {
	// Register adds a processor to the registry.
	Register(processor FileProcessor)

	// GetProcessor returns the first processor that can handle the given file path.
	// Returns nil if no processor can handle the file.
	GetProcessor(filePath string) FileProcessor

	// Processors returns all registered processors.
	Processors() []FileProcessor
}
