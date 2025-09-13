package antfly

import (
	"flag"
	"strings"
	"testing"

	"github.com/antflydb/antfly/antfly-go/antfly"
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core"
	"github.com/firebase/genkit/go/genkit"
	"github.com/stretchr/testify/require"
)

var (
	testURL   = flag.String("test-antfly-url", "", "Antfly url to use for tests")
	testTable = flag.String("test-antfly-table", "", "Antfly table to use for tests")
	testIndex = flag.String("test-antfly-index", "", "Antfly index to use for tests")
)

func TestGenkit(t *testing.T) {
	if *testURL == "" {
		t.Skip("skipping test because -test-antfly-url flag not used")
	}
	if *testTable == "" {
		t.Skip("skipping test because -test-antfly-table flag not used")
	}
	if *testIndex == "" {
		t.Skip("skipping test because -test-antfly-index flag not used")
	}

	ctx := t.Context()
	af := &Antfly{
		BaseURL: *testURL,
	}

	// actions := af.Init(ctx)
	// assert.Empty(t, actions)

	g := genkit.Init(ctx, genkit.WithPlugins(af))

	err := af.client.DropTable(ctx, *testTable)
	require.NoError(t, err)

	var modelConfig = antfly.ModelConfig{
		Provider: antfly.Ollama,
	}
	// TODO (ajr) Maybe we want a mock embedder for tests in antfly?
	err = modelConfig.FromOllamaConfig(antfly.OllamaConfig{
		Model: "all-minilm",
	})
	require.NoError(t, err)
	var idxConfig = antfly.IndexConfig{
		Type: antfly.VectorV2,
	}
	idxConfig.FromEmbeddingIndexConfig(
		antfly.EmbeddingIndexConfig{
			Field:          textKey,
			EmbedderConfig: modelConfig,
		})
	err = af.client.CreateTable(ctx, *testTable, antfly.CreateTableRequest{
		Indexes: map[string]antfly.IndexConfig{
			*testIndex: idxConfig,
		},
	})
	require.NoError(t, err)

	d1 := ai.DocumentFromText("hello1", map[string]any{"name": "hello1"})
	d2 := ai.DocumentFromText("hello2", map[string]any{"name": "hello2"})
	d3 := ai.DocumentFromText("goodbye", map[string]any{"name": "goodbye"})

	classCfg := IndexConfig{
		TableName: *testTable,
		IndexName: *testIndex,
	}
	retOpts := &ai.RetrieverOptions{
		ConfigSchema: core.InferSchemaMap(RetrieverOptions{}),
		Label:        provider,
		Supports: &ai.RetrieverSupports{
			Media: false,
		},
	}
	ds, retriever, err := DefineRetriever(ctx, g, classCfg, retOpts)
	if err != nil {
		t.Fatal(err)
	}

	err = Index(ctx, []*ai.Document{d1, d2, d3}, ds)
	if err != nil {
		t.Fatalf("Index operation failed: %v", err)
	}

	retrieverOptions := &RetrieverOptions{
		Count:        2,
		MetadataKeys: []string{"name"},
	}
	retrieverResp, err := genkit.Retrieve(ctx, g,
		ai.WithRetriever(retriever),
		ai.WithDocs(d1),
		ai.WithConfig(retrieverOptions))
	if err != nil {
		t.Fatalf("Retrieve operation failed: %v", err)
	}

	docs := retrieverResp.Documents
	if len(docs) != 2 {
		t.Errorf("got %d results, expected 2", len(docs))
	}
	for _, d := range docs {
		text := d.Content[0].Text
		if !strings.HasPrefix(text, "hello") {
			t.Errorf("returned doc text %q does not start with %q", text, "hello")
		}
		name, ok := d.Metadata["name"]
		if !ok {
			t.Errorf("missing metadata entry for name: %v", d.Metadata)
		} else if !strings.HasPrefix(name.(string), "hello") {
			t.Errorf("metadata name entry %q does not start with %q", name, "hello")
		}
	}
}
