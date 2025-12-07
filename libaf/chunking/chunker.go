//go:generate go tool oapi-codegen --config=cfg.yaml ./openapi.yaml
package chunking

import (
	"context"
)

// Fixed chunker model names
const (
	// ModelFixedBert uses BERT's WordPiece tokenization (~30k vocab).
	// Good for general-purpose text and multilingual content.
	ModelFixedBert = "fixed-bert-tokenizer"

	// ModelFixedBPE uses OpenAI's tiktoken BPE tokenization (cl100k_base, ~100k vocab).
	// Good for GPT-style models and code.
	ModelFixedBPE = "fixed-bpe-tokenizer"
)

// Chunker splits text into semantically meaningful chunks.
// ChunkOptions is generated from openapi.yaml - see openapi.gen.go
type Chunker interface {
	// Chunk splits text using the provided per-request options.
	// Options that are nil use the chunker's default values.
	Chunk(ctx context.Context, text string, opts ChunkOptions) ([]Chunk, error)
	Close() error
}
