package docsaf

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// Default export formats for Google Workspace MIME types.
var defaultExportFormats = map[string]string{
	"application/vnd.google-apps.document":     "text/html",
	"application/vnd.google-apps.spreadsheet":  "text/csv",
	"application/vnd.google-apps.presentation": "application/pdf",
	"application/vnd.google-apps.drawing":      "image/png",
}

// folderIDRegexp extracts the folder ID from various Google Drive URL formats.
var folderIDRegexp = regexp.MustCompile(`/folders/([a-zA-Z0-9_-]+)`)

// GoogleDriveSourceConfig holds configuration for a GoogleDriveSource.
type GoogleDriveSourceConfig struct {
	// CredentialsJSON is a service account key JSON string or file path.
	// Either CredentialsJSON or AccessToken must be provided.
	CredentialsJSON string

	// AccessToken is a pre-obtained OAuth2 access token.
	// Either CredentialsJSON or AccessToken must be provided.
	AccessToken string

	// FolderID is the Google Drive folder ID or full folder URL (required).
	// Supports URLs like https://drive.google.com/drive/folders/<ID> or
	// https://drive.google.com/drive/u/0/folders/<ID>.
	FolderID string

	// BaseURL is the base URL for generating document links (optional).
	// If empty, defaults to the Google Drive folder URL.
	BaseURL string

	// IncludePatterns is a list of glob patterns to include.
	// If empty, all files are included (subject to exclude patterns).
	// Supports ** wildcards for recursive matching.
	IncludePatterns []string

	// ExcludePatterns is a list of glob patterns to exclude.
	// Supports ** wildcards for recursive matching.
	ExcludePatterns []string

	// Concurrency controls how many parallel downloads run at once.
	// Default: 5
	Concurrency int

	// Recursive controls whether subfolders are traversed.
	// Default: true
	Recursive bool

	// IncludeSharedDrives enables listing files from shared/team drives.
	IncludeSharedDrives bool

	// ExportFormats overrides the default export MIME type for Google Workspace files.
	// Keys are Google Workspace MIME types, values are the export MIME types.
	ExportFormats map[string]string
}

// GoogleDriveSource traverses files in a Google Drive folder and yields content items.
type GoogleDriveSource struct {
	config    GoogleDriveSourceConfig
	service   *drive.Service
	semaphore chan struct{}
}

// NewGoogleDriveSource creates a new Google Drive content source.
func NewGoogleDriveSource(ctx context.Context, config GoogleDriveSourceConfig) (*GoogleDriveSource, error) {
	if config.CredentialsJSON == "" && config.AccessToken == "" {
		return nil, fmt.Errorf("either CredentialsJSON or AccessToken is required")
	}
	if config.FolderID == "" {
		return nil, fmt.Errorf("FolderID is required")
	}

	// Parse folder ID from URL if needed
	config.FolderID = parseFolderID(config.FolderID)

	// Set defaults
	if config.Concurrency <= 0 {
		config.Concurrency = 5
	}

	// Merge export formats with defaults
	exportFormats := make(map[string]string, len(defaultExportFormats))
	for k, v := range defaultExportFormats {
		exportFormats[k] = v
	}
	for k, v := range config.ExportFormats {
		exportFormats[k] = v
	}
	config.ExportFormats = exportFormats

	// Build Drive service
	var opts []option.ClientOption
	if config.AccessToken != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: config.AccessToken})
		opts = append(opts, option.WithTokenSource(ts))
	} else {
		// Try as file path first, fall back to inline JSON
		credJSON := []byte(config.CredentialsJSON)
		if data, err := os.ReadFile(config.CredentialsJSON); err == nil {
			credJSON = data
		}
		creds, err := google.CredentialsFromJSON(ctx, credJSON, drive.DriveReadonlyScope)
		if err != nil {
			return nil, fmt.Errorf("parsing credentials: %w", err)
		}
		opts = append(opts, option.WithCredentials(creds))
	}

	service, err := drive.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating Drive service: %w", err)
	}

	return &GoogleDriveSource{
		config:    config,
		service:   service,
		semaphore: make(chan struct{}, config.Concurrency),
	}, nil
}

// Type returns "google_drive" as the source type.
func (s *GoogleDriveSource) Type() string {
	return "google_drive"
}

// BaseURL returns the base URL for this source.
func (s *GoogleDriveSource) BaseURL() string {
	if s.config.BaseURL != "" {
		return s.config.BaseURL
	}
	return fmt.Sprintf("https://drive.google.com/drive/folders/%s", s.config.FolderID)
}

// Traverse lists files in the Google Drive folder and yields content items.
func (s *GoogleDriveSource) Traverse(ctx context.Context) (<-chan ContentItem, <-chan error) {
	items := make(chan ContentItem)
	errs := make(chan error, 1)

	go func() {
		defer close(items)
		defer close(errs)

		var wg sync.WaitGroup
		if err := s.traverseFolder(ctx, s.config.FolderID, "", items, &wg); err != nil {
			errs <- err
		}
		wg.Wait()
	}()

	return items, errs
}

// traverseFolder recursively lists files in a folder and sends them to the items channel.
func (s *GoogleDriveSource) traverseFolder(ctx context.Context, folderID, pathPrefix string, items chan<- ContentItem, wg *sync.WaitGroup) error {
	var pageToken string
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		query := fmt.Sprintf("'%s' in parents and trashed = false", folderID)
		call := s.service.Files.List().
			Q(query).
			Fields("nextPageToken, files(id, name, mimeType, size, modifiedTime, webViewLink)").
			PageSize(1000)

		if s.config.IncludeSharedDrives {
			call = call.SupportsAllDrives(true).IncludeItemsFromAllDrives(true)
		}

		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		fileList, err := call.Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("listing folder %s: %w", folderID, err)
		}

		for _, file := range fileList.Files {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Build relative path
			relPath := file.Name
			if pathPrefix != "" {
				relPath = pathPrefix + "/" + file.Name
			}

			// Handle folders
			if file.MimeType == "application/vnd.google-apps.folder" {
				if s.config.Recursive {
					if err := s.traverseFolder(ctx, file.Id, relPath, items, wg); err != nil {
						return err
					}
				}
				continue
			}

			// Skip Google Apps types that aren't exportable
			if strings.HasPrefix(file.MimeType, "application/vnd.google-apps.") {
				if _, ok := s.config.ExportFormats[file.MimeType]; !ok {
					continue
				}
			}

			// Check exclude/include patterns
			if s.shouldExclude(relPath) {
				continue
			}
			if !s.shouldInclude(relPath) {
				continue
			}

			// Download with concurrency control
			wg.Add(1)
			s.semaphore <- struct{}{}

			go func(f *drive.File, relPath string) {
				defer wg.Done()
				defer func() { <-s.semaphore }()

				content, contentType, err := s.downloadFile(ctx, f)
				if err != nil {
					log.Printf("Warning: Failed to download %s: %v", relPath, err)
					return
				}

				driveURL := f.WebViewLink
				if driveURL == "" {
					driveURL = fmt.Sprintf("https://drive.google.com/file/d/%s/view", f.Id)
				}

				select {
				case items <- ContentItem{
					Path:        relPath,
					SourceURL:   driveURL,
					Content:     content,
					ContentType: contentType,
					Metadata: map[string]any{
						"source_type": "google_drive",
						"file_id":     f.Id,
						"drive_url":   driveURL,
						"mod_time":    f.ModifiedTime,
						"size":        f.Size,
					},
				}:
				case <-ctx.Done():
					return
				}
			}(file, relPath)
		}

		pageToken = fileList.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return nil
}

// downloadFile downloads a file from Google Drive, handling Google Workspace exports.
func (s *GoogleDriveSource) downloadFile(ctx context.Context, file *drive.File) ([]byte, string, error) {
	// Check if this is an exportable Google Workspace file
	if exportMIME, ok := s.config.ExportFormats[file.MimeType]; ok {
		resp, err := s.service.Files.Export(file.Id, exportMIME).Context(ctx).Download()
		if err != nil {
			return nil, "", fmt.Errorf("exporting file %s: %w", file.Name, err)
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, "", fmt.Errorf("reading exported file %s: %w", file.Name, err)
		}
		return data, exportMIME, nil
	}

	// Regular file download
	resp, err := s.service.Files.Get(file.Id).Context(ctx).Download()
	if err != nil {
		return nil, "", fmt.Errorf("downloading file %s: %w", file.Name, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("reading file %s: %w", file.Name, err)
	}

	contentType := DetectContentType(file.Name, data)
	return data, contentType, nil
}

// parseFolderID extracts a folder ID from a Google Drive URL or returns the input as-is.
func parseFolderID(input string) string {
	if matches := folderIDRegexp.FindStringSubmatch(input); len(matches) == 2 {
		return matches[1]
	}
	return input
}

// shouldExclude checks if a path matches any exclude pattern.
func (s *GoogleDriveSource) shouldExclude(path string) bool {
	for _, pattern := range s.config.ExcludePatterns {
		matched, err := doublestar.Match(pattern, path)
		if err != nil {
			log.Printf("Warning: Invalid exclude pattern %s: %v", pattern, err)
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

// shouldInclude checks if a path matches include patterns.
// If no include patterns are configured, returns true.
func (s *GoogleDriveSource) shouldInclude(path string) bool {
	if len(s.config.IncludePatterns) == 0 {
		return true
	}
	for _, pattern := range s.config.IncludePatterns {
		matched, err := doublestar.Match(pattern, path)
		if err != nil {
			log.Printf("Warning: Invalid include pattern %s: %v", pattern, err)
			continue
		}
		if matched {
			return true
		}
	}
	return false
}
