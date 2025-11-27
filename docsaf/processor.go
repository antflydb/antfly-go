package docsaf

import (
	"context"
	"log"
)

// SourceWithBaseURL is an interface for sources that have a base URL.
type SourceWithBaseURL interface {
	ContentSource
	BaseURL() string
}

// UnifiedProcessor processes content from any source using registered content processors.
// It abstracts the traversal mechanism, allowing the same processing logic
// to work with filesystem, web, and other content sources.
type UnifiedProcessor struct {
	source   ContentSource
	registry ContentProcessorRegistry
	baseURL  string
}

// NewUnifiedProcessor creates a new unified processor.
// The source provides content items, and the registry provides processors to handle them.
func NewUnifiedProcessor(source ContentSource, registry ContentProcessorRegistry) *UnifiedProcessor {
	baseURL := ""
	if s, ok := source.(SourceWithBaseURL); ok {
		baseURL = s.BaseURL()
	}

	return &UnifiedProcessor{
		source:   source,
		registry: registry,
		baseURL:  baseURL,
	}
}

// SetBaseURL sets the base URL for generated links.
// This overrides any base URL from the source.
func (up *UnifiedProcessor) SetBaseURL(baseURL string) {
	up.baseURL = baseURL
}

// Process traverses the source and processes all content items.
// Returns a slice of all extracted DocumentSections.
func (up *UnifiedProcessor) Process(ctx context.Context) ([]DocumentSection, error) {
	var allSections []DocumentSection

	items, errs := up.source.Traverse(ctx)

	for item := range items {
		processor := up.registry.GetProcessor(item.ContentType, item.Path)
		if processor == nil {
			continue
		}

		sections, err := processor.ProcessContent(
			item.Path,
			item.SourceURL,
			up.baseURL,
			item.Content,
		)
		if err != nil {
			log.Printf("Warning: Failed to process %s: %v", item.Path, err)
			continue
		}

		allSections = append(allSections, sections...)
	}

	// Check for errors from the traversal
	for err := range errs {
		if err != nil {
			return allSections, err
		}
	}

	return allSections, nil
}

// ProcessWithCallback traverses the source and calls the callback for each batch of sections.
// This is useful for streaming large amounts of content without holding everything in memory.
func (up *UnifiedProcessor) ProcessWithCallback(ctx context.Context, callback func([]DocumentSection) error) error {
	items, errs := up.source.Traverse(ctx)

	for item := range items {
		processor := up.registry.GetProcessor(item.ContentType, item.Path)
		if processor == nil {
			continue
		}

		sections, err := processor.ProcessContent(
			item.Path,
			item.SourceURL,
			up.baseURL,
			item.Content,
		)
		if err != nil {
			log.Printf("Warning: Failed to process %s: %v", item.Path, err)
			continue
		}

		if len(sections) > 0 {
			if err := callback(sections); err != nil {
				return err
			}
		}
	}

	// Check for errors from the traversal
	for err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}

// SourceType returns the type of the underlying content source.
func (up *UnifiedProcessor) SourceType() string {
	return up.source.Type()
}
