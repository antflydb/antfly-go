package s3

//go:generate go tool oapi-codegen --config=cfg.yaml ./openapi.yaml

import (
	"errors"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// NewMinioClient creates a Minio client from a Credentials struct.
// This is the preferred method when using the full Credentials configuration.
func (creds *Credentials) NewMinioClient() (*minio.Client, error) {
	if creds.Endpoint == "" {
		return nil, errors.New("endpoint is required")
	}
	if creds.AccessKeyId == "" {
		return nil, errors.New("access key ID is required")
	}
	if creds.SecretAccessKey == "" {
		return nil, errors.New("secret access key is required")
	}

	minioClient, err := minio.New(creds.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(creds.AccessKeyId, creds.SecretAccessKey, creds.SessionToken),
		Secure: creds.UseSsl,
	})
	if err != nil {
		return nil, fmt.Errorf("creating S3 client for endpoint %s: %w", creds.Endpoint, err)
	}
	return minioClient, nil
}
