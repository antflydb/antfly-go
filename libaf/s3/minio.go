package s3

import (
	"errors"
	"fmt"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Credentials holds S3 authentication credentials.
type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string // Optional, for temporary credentials
}

// NewMinioClientWithCredentials creates a Minio client with explicit credentials.
// This is the preferred method when credentials come from config with secret resolution.
func NewMinioClientWithCredentials(endpoint string, creds Credentials, useSSL bool) (*minio.Client, error) {
	if creds.AccessKeyID == "" {
		return nil, errors.New("access key ID is required")
	}
	if creds.SecretAccessKey == "" {
		return nil, errors.New("secret access key is required")
	}

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("creating S3 client for endpoint %s: %w", endpoint, err)
	}
	return minioClient, nil
}

// NewMinioClient creates a Minio client using credentials from environment variables.
// This is a convenience function for cases where credentials aren't in config.
// Prefer NewMinioClientWithCredentials when credentials are available from config.
func NewMinioClient(endpoint string, useSSL bool) (*minio.Client, error) {
	accessKeyID := os.Getenv("AWS_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	sessionToken := os.Getenv("AWS_SESSION_TOKEN")

	if accessKeyID == "" {
		return nil, errors.New("AWS_ACCESS_KEY_ID not set, please configure your credentials")
	}
	if secretAccessKey == "" {
		return nil, errors.New("AWS_SECRET_ACCESS_KEY not set, please configure your credentials")
	}

	return NewMinioClientWithCredentials(endpoint, Credentials{
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
		SessionToken:    sessionToken,
	}, useSSL)
}
