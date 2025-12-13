package docsaf

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/antflydb/antfly-go/libaf/s3"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/minio/minio-go/v7"
)

// S3SourceConfig holds configuration for an S3Source.
type S3SourceConfig struct {
	// Endpoint is the S3 endpoint (e.g., "s3.amazonaws.com", "minio.example.com")
	Endpoint string

	// Bucket is the S3 bucket name
	Bucket string

	// Prefix is the key prefix to filter objects (optional)
	Prefix string

	// BaseURL is the base URL for generating document links (optional)
	BaseURL string

	// UseSSL enables HTTPS connections to the S3 endpoint
	UseSSL bool

	// Credentials holds S3 authentication credentials.
	// If nil, credentials are read from environment variables.
	Credentials *s3.Credentials

	// IncludePatterns is a list of glob patterns to include.
	// Files matching any include pattern will be processed.
	// If empty, all files are included (subject to exclude patterns).
	// Supports ** wildcards for recursive matching.
	IncludePatterns []string

	// ExcludePatterns is a list of glob patterns to exclude.
	// Files matching any exclude pattern will be skipped.
	// Supports ** wildcards for recursive matching.
	ExcludePatterns []string
}

// S3Source traverses an S3 bucket and yields content items.
type S3Source struct {
	config S3SourceConfig
	client *minio.Client
}

// NewS3Source creates a new S3 content source.
func NewS3Source(config S3SourceConfig) (*S3Source, error) {
	var client *minio.Client
	var err error

	if config.Credentials != nil {
		client, err = s3.NewMinioClientWithCredentials(config.Endpoint, *config.Credentials, config.UseSSL)
	} else {
		client, err = s3.NewMinioClient(config.Endpoint, config.UseSSL)
	}
	if err != nil {
		return nil, fmt.Errorf("creating S3 client: %w", err)
	}

	return &S3Source{
		config: config,
		client: client,
	}, nil
}

// Type returns "s3" as the source type.
func (s *S3Source) Type() string {
	return "s3"
}

// BaseURL returns the base URL for this source.
func (s *S3Source) BaseURL() string {
	return s.config.BaseURL
}

// Traverse lists all objects in the bucket and yields content items for matching files.
// It returns a channel of ContentItems and a channel for errors.
func (s *S3Source) Traverse(ctx context.Context) (<-chan ContentItem, <-chan error) {
	items := make(chan ContentItem)
	errs := make(chan error, 1)

	go func() {
		defer close(items)
		defer close(errs)

		opts := minio.ListObjectsOptions{
			Prefix:    s.config.Prefix,
			Recursive: true,
		}

		for object := range s.client.ListObjects(ctx, s.config.Bucket, opts) {
			// Check for cancellation
			select {
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			default:
			}

			if object.Err != nil {
				errs <- fmt.Errorf("listing objects: %w", object.Err)
				return
			}

			// Skip directories (keys ending with /)
			if strings.HasSuffix(object.Key, "/") {
				continue
			}

			// Get relative path for pattern matching
			relPath := object.Key
			if s.config.Prefix != "" {
				relPath = strings.TrimPrefix(object.Key, s.config.Prefix)
				relPath = strings.TrimPrefix(relPath, "/")
			}

			// Check exclude patterns first
			if s.matchesAnyPattern(relPath, s.config.ExcludePatterns) {
				continue
			}

			// If include patterns are specified, check them
			if len(s.config.IncludePatterns) > 0 {
				if !s.matchesAnyPattern(relPath, s.config.IncludePatterns) {
					continue
				}
			}

			// Download object content
			obj, err := s.client.GetObject(ctx, s.config.Bucket, object.Key, minio.GetObjectOptions{})
			if err != nil {
				errs <- fmt.Errorf("getting object %s: %w", object.Key, err)
				return
			}

			content, err := io.ReadAll(obj)
			obj.Close()
			if err != nil {
				errs <- fmt.Errorf("reading object %s: %w", object.Key, err)
				return
			}

			// Get content type from object metadata or detect from path
			contentType := object.ContentType
			if contentType == "" || contentType == "application/octet-stream" {
				contentType = DetectContentType(object.Key, content)
			}

			// Send content item
			select {
			case items <- ContentItem{
				Path:        relPath,
				SourceURL:   s.buildSourceURL(object.Key),
				Content:     content,
				ContentType: contentType,
				Metadata: map[string]any{
					"source_type": "s3",
					"bucket":      s.config.Bucket,
					"key":         object.Key,
					"size":        object.Size,
					"last_modified": object.LastModified,
					"etag":        object.ETag,
				},
			}:
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			}
		}
	}()

	return items, errs
}

// matchesAnyPattern checks if a path matches any of the given glob patterns.
func (s *S3Source) matchesAnyPattern(path string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := doublestar.Match(pattern, path)
		if err != nil {
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

// buildSourceURL constructs a URL for the S3 object.
func (s *S3Source) buildSourceURL(key string) string {
	scheme := "http"
	if s.config.UseSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s", scheme, s.config.Endpoint, s.config.Bucket, key)
}

