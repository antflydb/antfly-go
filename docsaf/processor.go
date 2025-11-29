package docsaf

import (
	"context"
	"log"
)

// Processor processes content from any source using registered processors.
// It abstracts the traversal mechanism, allowing the same processing logic
// to work with filesystem, web, and other content sources.
type Processor struct {
	source   ContentSource
	registry ProcessorRegistry
	baseURL  string
}

// NewProcessor creates a new processor.
// The source provides content items, and the registry provides processors to handle them.
func NewProcessor(source ContentSource, registry ProcessorRegistry) *Processor {
	return &Processor{
		source:   source,
		registry: registry,
		baseURL:  source.BaseURL(),
	}
}

// SetBaseURL sets the base URL for generated links.
// This overrides the base URL from the source.
func (p *Processor) SetBaseURL(baseURL string) {
	p.baseURL = baseURL
}

// Process traverses the source and processes all content items.
// Returns a slice of all extracted DocumentSections.
func (p *Processor) Process(ctx context.Context) ([]DocumentSection, error) {
	var allSections []DocumentSection

	items, errs := p.source.Traverse(ctx)

	for item := range items {
		processor := p.registry.GetProcessor(item.ContentType, item.Path)
		if processor == nil {
			continue
		}

		sections, err := processor.Process(
			item.Path,
			item.SourceURL,
			p.baseURL,
			item.Content,
		)
		if err != nil {
			log.Printf("Warning: Failed to process %s: %v", item.Path, err)
			continue
		}

		allSections = append(allSections, sections...)
	}

	for err := range errs {
		if err != nil {
			return allSections, err
		}
	}

	return allSections, nil
}

// ProcessWithCallback traverses the source and calls the callback for each batch of sections.
// This is useful for streaming large amounts of content without holding everything in memory.
func (p *Processor) ProcessWithCallback(ctx context.Context, callback func([]DocumentSection) error) error {
	items, errs := p.source.Traverse(ctx)

	for item := range items {
		processor := p.registry.GetProcessor(item.ContentType, item.Path)
		if processor == nil {
			continue
		}

		sections, err := processor.Process(
			item.Path,
			item.SourceURL,
			p.baseURL,
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

	for err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}

// SourceType returns the type of the underlying content source.
func (p *Processor) SourceType() string {
	return p.source.Type()
}
