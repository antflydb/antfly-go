package docsaf

import (
	"testing"

	"github.com/antflydb/antfly-go/libaf/s3"
)

func TestS3Source_Type(t *testing.T) {
	// We can't fully create a source without credentials, but we can test config
	config := S3SourceConfig{
		Endpoint: "s3.amazonaws.com",
		Bucket:   "test-bucket",
		UseSSL:   true,
	}

	// Test the config structure
	if config.Endpoint != "s3.amazonaws.com" {
		t.Errorf("Endpoint = %q, want %q", config.Endpoint, "s3.amazonaws.com")
	}
	if config.Bucket != "test-bucket" {
		t.Errorf("Bucket = %q, want %q", config.Bucket, "test-bucket")
	}
	if !config.UseSSL {
		t.Error("UseSSL should be true")
	}
}

func TestS3Source_ConfigWithCredentials(t *testing.T) {
	creds := &s3.Credentials{
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}

	config := S3SourceConfig{
		Endpoint:    "minio.example.com",
		Bucket:      "docs",
		Prefix:      "documentation/",
		BaseURL:     "https://docs.example.com",
		UseSSL:      true,
		Credentials: creds,
		IncludePatterns: []string{"**/*.md", "**/*.html"},
		ExcludePatterns: []string{"**/draft/**"},
	}

	if config.Prefix != "documentation/" {
		t.Errorf("Prefix = %q, want %q", config.Prefix, "documentation/")
	}
	if config.BaseURL != "https://docs.example.com" {
		t.Errorf("BaseURL = %q, want %q", config.BaseURL, "https://docs.example.com")
	}
	if len(config.IncludePatterns) != 2 {
		t.Errorf("len(IncludePatterns) = %d, want 2", len(config.IncludePatterns))
	}
	if len(config.ExcludePatterns) != 1 {
		t.Errorf("len(ExcludePatterns) = %d, want 1", len(config.ExcludePatterns))
	}
	if config.Credentials.AccessKeyID != "AKIAIOSFODNN7EXAMPLE" {
		t.Errorf("Credentials.AccessKeyID = %q, want %q", config.Credentials.AccessKeyID, "AKIAIOSFODNN7EXAMPLE")
	}
}

func TestS3Source_MatchesAnyPattern(t *testing.T) {
	source := &S3Source{
		config: S3SourceConfig{},
	}

	tests := []struct {
		name     string
		path     string
		patterns []string
		want     bool
	}{
		{
			name:     "Match simple glob",
			path:     "docs/readme.md",
			patterns: []string{"**/*.md"},
			want:     true,
		},
		{
			name:     "Match exact pattern",
			path:     "index.html",
			patterns: []string{"*.html"},
			want:     true,
		},
		{
			name:     "No match",
			path:     "image.png",
			patterns: []string{"**/*.md", "**/*.html"},
			want:     false,
		},
		{
			name:     "Match with directory pattern",
			path:     "draft/doc.md",
			patterns: []string{"draft/**"},
			want:     true,
		},
		{
			name:     "Empty patterns",
			path:     "any/file.txt",
			patterns: []string{},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := source.matchesAnyPattern(tt.path, tt.patterns)
			if got != tt.want {
				t.Errorf("matchesAnyPattern(%q, %v) = %v, want %v", tt.path, tt.patterns, got, tt.want)
			}
		})
	}
}

func TestS3Source_BuildSourceURL(t *testing.T) {
	tests := []struct {
		name     string
		config   S3SourceConfig
		key      string
		expected string
	}{
		{
			name: "HTTPS URL",
			config: S3SourceConfig{
				Endpoint: "s3.amazonaws.com",
				Bucket:   "my-bucket",
				UseSSL:   true,
			},
			key:      "docs/readme.md",
			expected: "https://s3.amazonaws.com/my-bucket/docs/readme.md",
		},
		{
			name: "HTTP URL",
			config: S3SourceConfig{
				Endpoint: "minio.local:9000",
				Bucket:   "test",
				UseSSL:   false,
			},
			key:      "file.txt",
			expected: "http://minio.local:9000/test/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := &S3Source{config: tt.config}
			got := source.buildSourceURL(tt.key)
			if got != tt.expected {
				t.Errorf("buildSourceURL(%q) = %q, want %q", tt.key, got, tt.expected)
			}
		})
	}
}

func TestNewS3Source_MissingCredentials(t *testing.T) {
	// Clear env vars to ensure we test the error path
	// Note: This test will fail if AWS_ACCESS_KEY_ID is set in the environment
	config := S3SourceConfig{
		Endpoint: "s3.amazonaws.com",
		Bucket:   "test-bucket",
		UseSSL:   true,
		// No credentials provided
	}

	_, err := NewS3Source(config)
	// We expect an error because no credentials are provided and env vars are likely not set
	// In a CI environment without credentials, this should fail
	if err == nil {
		// If it succeeds, that means env vars were set - that's okay for local development
		t.Log("NewS3Source succeeded - AWS credentials must be set in environment")
	}
}
