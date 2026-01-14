package s3

//go:generate go tool oapi-codegen --config=cfg.yaml ./openapi.yaml

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

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

// DownloadObjectOptions configures how an S3 object is downloaded.
type DownloadObjectOptions struct {
	// ProgressFn is called for each part downloaded (partNumber, partSize, totalParts).
	// If nil, no progress tracking is performed.
	ProgressFn func(partNumber, partSize, totalParts int)
}

// DownloadObject downloads an object from S3 to a local file.
// For multipart objects, it downloads parts individually for progress tracking.
// The download is atomic - a temp file is used and renamed on success.
func (creds *Credentials) DownloadObject(
	ctx context.Context,
	bucketName, objectKey, destPath string,
	opts *DownloadObjectOptions,
) error {
	client, err := creds.NewMinioClient()
	if err != nil {
		return fmt.Errorf("creating S3 client: %w", err)
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), os.ModePerm); err != nil {
		return fmt.Errorf("creating directory %s: %w", filepath.Dir(destPath), err)
	}

	// Try to get object attributes for multipart download with progress
	attrs, err := client.GetObjectAttributes(ctx, bucketName, objectKey, minio.ObjectAttributesOptions{})
	if err == nil && attrs.ObjectParts.PartsCount > 1 && opts != nil && opts.ProgressFn != nil {
		// Multipart object - download parts individually for progress tracking
		return creds.downloadMultipart(ctx, client, bucketName, objectKey, destPath, attrs, opts.ProgressFn)
	}

	// Single-part object or GetObjectAttributes not supported - use simple download
	if err := client.FGetObject(ctx, bucketName, objectKey, destPath, minio.GetObjectOptions{}); err != nil {
		return fmt.Errorf("downloading object %s from bucket %s: %w", objectKey, bucketName, err)
	}

	return nil
}

// downloadMultipart downloads a multipart object part-by-part with progress tracking.
func (creds *Credentials) downloadMultipart(
	ctx context.Context,
	client *minio.Client,
	bucketName, objectKey, destPath string,
	attrs *minio.ObjectAttributes,
	progressFn func(partNumber, partSize, totalParts int),
) error {
	filePartPath := destPath + ".part.minio"
	f, err := os.OpenFile(filePartPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("creating temp file %s: %w", filePartPath, err)
	}

	cleanupOnError := true
	defer func() {
		if cleanupOnError {
			_ = f.Close()
			_ = os.Remove(filePartPath)
		}
	}()

	for _, part := range attrs.ObjectParts.Parts {
		if progressFn != nil {
			progressFn(part.PartNumber, part.Size, attrs.ObjectParts.PartsCount)
		}

		obj, err := client.GetObject(ctx, bucketName, objectKey, minio.GetObjectOptions{
			VersionID:  attrs.VersionID,
			PartNumber: part.PartNumber,
		})
		if err != nil {
			return fmt.Errorf("getting part %d: %w", part.PartNumber, err)
		}

		_, copyErr := io.Copy(f, obj)
		obj.Close()
		if copyErr != nil {
			return fmt.Errorf("downloading part %d: %w", part.PartNumber, copyErr)
		}
	}

	// Close file before rename (required for Windows)
	cleanupOnError = false
	if err = f.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	// Atomically move to final destination
	if err = os.Rename(filePartPath, destPath); err != nil {
		return fmt.Errorf("renaming %s to %s: %w", filePartPath, destPath, err)
	}

	return nil
}
