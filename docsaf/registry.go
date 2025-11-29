package docsaf

// registry is a simple implementation of ProcessorRegistry.
type registry struct {
	processors []ContentProcessor
}

// NewRegistry creates a new empty processor registry.
// Use this to build a custom registry with only the processors you need.
func NewRegistry() ProcessorRegistry {
	return &registry{
		processors: make([]ContentProcessor, 0),
	}
}

// DefaultRegistry creates a registry with all built-in processors registered.
// This includes MarkdownProcessor, OpenAPIProcessor, HTMLProcessor, and PDFProcessor.
func DefaultRegistry() ProcessorRegistry {
	r := &registry{
		processors: make([]ContentProcessor, 0, 4),
	}
	r.Register(&MarkdownProcessor{})
	r.Register(&OpenAPIProcessor{})
	r.Register(&HTMLProcessor{})
	r.Register(&PDFProcessor{})
	return r
}

// NewWholeFileRegistry creates a registry with only the WholeFileProcessor.
// This processor returns entire content without any chunking,
// allowing Antfly's internal chunking (e.g., Termite) to handle
// document segmentation during the embedding process.
func NewWholeFileRegistry() ProcessorRegistry {
	r := &registry{
		processors: make([]ContentProcessor, 0, 1),
	}
	r.Register(&WholeFileProcessor{})
	return r
}

// Register adds a processor to the registry.
func (r *registry) Register(processor ContentProcessor) {
	r.processors = append(r.processors, processor)
}

// GetProcessor returns the first processor that can handle the given content.
// Returns nil if no processor can handle the content.
func (r *registry) GetProcessor(contentType, path string) ContentProcessor {
	for _, processor := range r.processors {
		if processor.CanProcess(contentType, path) {
			return processor
		}
	}
	return nil
}

// Processors returns all registered processors.
func (r *registry) Processors() []ContentProcessor {
	return r.processors
}
