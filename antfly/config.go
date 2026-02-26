/*
Copyright 2025 The Antfly Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package antfly

import (
	"fmt"

	"github.com/antflydb/antfly-go/antfly/oapi"
)

func NewEmbedderConfig(config any) (*EmbedderConfig, error) {
	var provider EmbedderProvider
	modelConfig := &EmbedderConfig{}
	switch v := config.(type) {
	case OllamaEmbedderConfig:
		provider = EmbedderProviderOllama
		modelConfig.FromOllamaEmbedderConfig(v)
	case OpenAIEmbedderConfig:
		provider = EmbedderProviderOpenai
		modelConfig.FromOpenAIEmbedderConfig(v)
	case GoogleEmbedderConfig:
		provider = EmbedderProviderGemini
		modelConfig.FromGoogleEmbedderConfig(v)
	case BedrockEmbedderConfig:
		provider = EmbedderProviderBedrock
		modelConfig.FromBedrockEmbedderConfig(v)
	case VertexEmbedderConfig:
		provider = EmbedderProviderVertex
		modelConfig.FromVertexEmbedderConfig(v)
	case TermiteEmbedderConfig:
		provider = EmbedderProviderTermite
		modelConfig.FromTermiteEmbedderConfig(v)
	default:
		return nil, fmt.Errorf("unknown model config type: %T", v)
	}

	modelConfig.Provider = provider
	return modelConfig, nil
}

func NewGeneratorConfig(config any) (*GeneratorConfig, error) {
	var provider GeneratorProvider
	modelConfig := &GeneratorConfig{}
	switch v := config.(type) {
	case oapi.OllamaGeneratorConfig:
		provider = oapi.GeneratorProviderOllama
		modelConfig.FromOllamaGeneratorConfig(v)
	case oapi.OpenAIGeneratorConfig:
		provider = oapi.GeneratorProviderOpenai
		modelConfig.FromOpenAIGeneratorConfig(v)
	case oapi.GoogleGeneratorConfig:
		provider = oapi.GeneratorProviderGemini
		modelConfig.FromGoogleGeneratorConfig(v)
	case oapi.BedrockGeneratorConfig:
		provider = oapi.GeneratorProviderBedrock
		modelConfig.FromBedrockGeneratorConfig(v)
	case oapi.VertexGeneratorConfig:
		provider = oapi.GeneratorProviderVertex
		modelConfig.FromVertexGeneratorConfig(v)
	case oapi.AnthropicGeneratorConfig:
		provider = oapi.GeneratorProviderAnthropic
		modelConfig.FromAnthropicGeneratorConfig(v)
	case oapi.TermiteGeneratorConfig:
		provider = oapi.GeneratorProviderTermite
		modelConfig.FromTermiteGeneratorConfig(v)
	default:
		return nil, fmt.Errorf("unknown model config type: %T", v)
	}

	modelConfig.Provider = provider
	return modelConfig, nil
}

func NewRerankerConfig(config any) (*RerankerConfig, error) {
	var provider RerankerProvider
	rerankerConfig := &RerankerConfig{}
	switch v := config.(type) {
	case OllamaRerankerConfig:
		provider = RerankerProviderOllama
		rerankerConfig.FromOllamaRerankerConfig(v)
	case TermiteRerankerConfig:
		provider = RerankerProviderTermite
		rerankerConfig.FromTermiteRerankerConfig(v)
	default:
		return nil, fmt.Errorf("unknown reranker config type: %T", v)
	}

	rerankerConfig.Provider = provider
	return rerankerConfig, nil
}

func NewIndexConfig(name string, config any) (*IndexConfig, error) {
	var t IndexType
	idxConfig := &IndexConfig{
		Name: name,
	}
	switch v := config.(type) {
	case EmbeddingsIndexConfig:
		t = IndexTypeEmbeddings
		idxConfig.FromEmbeddingsIndexConfig(v)
	case BleveIndexConfig:
		t = IndexTypeFullText
		idxConfig.FromBleveIndexConfig(v)
	case GraphIndexConfig:
		t = IndexTypeGraph
		idxConfig.FromGraphIndexConfig(v)
	default:
		return nil, fmt.Errorf("unsupported index config type: %T", config)
	}
	idxConfig.Type = t

	return idxConfig, nil
}
