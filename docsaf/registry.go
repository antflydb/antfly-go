package docsaf

// registry is a simple implementation of ProcessorRegistry.
type registry struct {
	processors []FileProcessor
}

// NewRegistry creates a new empty processor registry.
// Use this to build a custom registry with only the processors you need.
func NewRegistry() ProcessorRegistry {
	return &registry{
		processors: make([]FileProcessor, 0),
	}
}

// DefaultRegistry creates a registry with all built-in processors registered.
// This includes MarkdownProcessor, OpenAPIProcessor, HTMLProcessor, and PDFProcessor.
func DefaultRegistry() ProcessorRegistry {
	r := &registry{
		processors: make([]FileProcessor, 0, 4),
	}
	r.Register(&MarkdownProcessor{})
	r.Register(&OpenAPIProcessor{})
	r.Register(&HTMLProcessor{})
	r.Register(&PDFProcessor{})
	return r
}

// NewWholeFileRegistry creates a registry with only the WholeFileProcessor.
// This processor reads entire files without any chunking or sectioning,
// allowing Antfly's internal chunking (e.g., Termite) to handle document
// segmentation during the embedding process.
func NewWholeFileRegistry() ProcessorRegistry {
	r := &registry{
		processors: make([]FileProcessor, 0, 1),
	}
	r.Register(&WholeFileProcessor{})
	return r
}

// Register adds a processor to the registry.
func (r *registry) Register(processor FileProcessor) {
	r.processors = append(r.processors, processor)
}

// GetProcessor returns the first processor that can handle the given file path.
// Returns nil if no processor can handle the file.
func (r *registry) GetProcessor(filePath string) FileProcessor {
	for _, processor := range r.processors {
		if processor.CanProcess(filePath) {
			return processor
		}
	}
	return nil
}

// Processors returns all registered processors.
func (r *registry) Processors() []FileProcessor {
	return r.processors
}

// contentRegistry is a simple implementation of ContentProcessorRegistry.
type contentRegistry struct {
	processors []ContentProcessor
}

// NewContentRegistry creates a new empty content processor registry.
// Use this to build a custom registry with only the processors you need.
func NewContentRegistry() ContentProcessorRegistry {
	return &contentRegistry{
		processors: make([]ContentProcessor, 0),
	}
}

// DefaultContentRegistry creates a registry with all built-in content processors registered.
// This includes MarkdownContentProcessor and HTMLContentProcessor.
// These processors work with raw content bytes, making them suitable for both
// filesystem and web sources.
func DefaultContentRegistry() ContentProcessorRegistry {
	r := &contentRegistry{
		processors: make([]ContentProcessor, 0, 2),
	}
	r.Register(&MarkdownContentProcessor{})
	r.Register(&HTMLContentProcessor{})
	return r
}

// Register adds a content processor to the registry.
func (r *contentRegistry) Register(processor ContentProcessor) {
	r.processors = append(r.processors, processor)
}

// GetProcessor returns the first content processor that can handle the given content.
// Returns nil if no processor can handle the content.
func (r *contentRegistry) GetProcessor(contentType, path string) ContentProcessor {
	for _, processor := range r.processors {
		if processor.CanProcess(contentType, path) {
			return processor
		}
	}
	return nil
}

// Processors returns all registered content processors.
func (r *contentRegistry) Processors() []ContentProcessor {
	return r.processors
}
