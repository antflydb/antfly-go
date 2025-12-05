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

//go:generate go tool oapi-codegen --config=cfg.yaml ../../openapi.yaml

package antfly

import (
	"fmt"
	"io"
	"net/http"

	"github.com/antflydb/antfly-go/antfly/oapi"
)

// AntflyClient is a client for interacting with the Antfly API
type AntflyClient struct {
	client *oapi.Client
}

// NewAntflyClient creates a new Antfly client
func NewAntflyClient(baseURL string, httpClient *http.Client) (*AntflyClient, error) {
	client, err := oapi.NewClient(baseURL, oapi.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}
	return &AntflyClient{
		client: client,
	}, err
}

func readErrorResponse(resp *http.Response) error {
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading http response: %w", err)
	}
	return fmt.Errorf("received status %d: %s", resp.StatusCode, string(respBody))
}
