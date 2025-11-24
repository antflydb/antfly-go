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
// This includes MarkdownProcessor and OpenAPIProcessor.
func DefaultRegistry() ProcessorRegistry {
	r := &registry{
		processors: make([]FileProcessor, 0, 2),
	}
	r.Register(&MarkdownProcessor{})
	r.Register(&OpenAPIProcessor{})
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
