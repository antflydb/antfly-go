# docsaf

A generic content traversal and processing library for building documentation from various sources including local filesystems and websites.

## Features

- **Multiple Content Sources**: Traverse local directories, crawl websites, or clone Git repositories
- **Pluggable Processors**: Markdown, HTML, PDF, OpenAPI, and custom processors
- **Web Crawling**: Full-featured web crawler with sitemap support via go-colly
- **Git Integration**: Clone and traverse Git repositories with branch/tag support
- **URL Normalization**: Consistent URL deduplication across crawls
- **Retry Logic**: Exponential backoff for transient failures
- **Advanced Caching**: HTTP-aware caching with disk persistence, ETag/Last-Modified support, and content deduplication
- **robots.txt Support**: Respect crawling directives

## Installation

```bash
go get github.com/antflydb/antfly-go/docsaf
```

## Quick Start

### Processing Local Files

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/antflydb/antfly-go/docsaf"
)

func main() {
    // Create a filesystem source
    source, err := docsaf.NewFilesystemSource(docsaf.FilesystemSourceConfig{
        BaseDir: "./docs",
        BaseURL: "https://example.com/docs",
        IncludePatterns: []string{"**/*.md", "**/*.html"},
    })
    if err != nil {
        log.Fatal(err)
    }

    // Create processor with default registry (Markdown, HTML, PDF, OpenAPI)
    processor := docsaf.NewProcessor(source, docsaf.DefaultRegistry())

    // Process all content
    sections, err := processor.Process(context.Background())
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Processed %d sections\n", len(sections))
}
```

### Crawling a Website

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/antflydb/antfly-go/docsaf"
)

func main() {
    // Create a web source
    source, err := docsaf.NewWebSource(docsaf.WebSourceConfig{
        StartURL:         "https://docs.example.com",
        IncludePatterns:  []string{"/docs/**", "/guides/**"},
        UseSitemap:       true,
        MaxPages:         100,
        Concurrency:      2,
        RequestDelay:     200 * time.Millisecond,
        RespectRobotsTxt: true,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Create processor
    processor := docsaf.NewProcessor(source, docsaf.DefaultRegistry())

    // Process with streaming callback
    err = processor.ProcessWithCallback(context.Background(), func(sections []docsaf.DocumentSection) error {
        for _, section := range sections {
            fmt.Printf("Section: %s - %s\n", section.Title, section.URL)
        }
        return nil
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

## Content Sources

### FilesystemSource

Traverses local directories and yields files matching specified patterns.

```go
source, err := docsaf.NewFilesystemSource(docsaf.FilesystemSourceConfig{
    // Required: Base directory to traverse
    BaseDir: "./docs",

    // Required: Base URL for generating document links
    BaseURL: "https://example.com/docs",

    // Optional: Glob patterns for files to include (default: all files)
    IncludePatterns: []string{"**/*.md", "**/*.html"},

    // Optional: Glob patterns for files to exclude
    ExcludePatterns: []string{"**/node_modules/**", "**/.git/**"},
})
```

### WebSource

Crawls websites using go-colly with support for sitemaps, rate limiting, and more.

```go
source, err := docsaf.NewWebSource(docsaf.WebSourceConfig{
    // Required: Starting URL for the crawl
    StartURL: "https://docs.example.com",

    // Optional: Base URL for document links (default: derived from StartURL)
    BaseURL: "https://docs.example.com",

    // Optional: Restrict crawling to these domains (default: StartURL domain)
    AllowedDomains: []string{"docs.example.com"},

    // Optional: URL path patterns to include
    IncludePatterns: []string{"/docs/**", "/api/**"},

    // Optional: URL path patterns to exclude (has defaults for static assets)
    ExcludePatterns: []string{"/blog/**"},

    // Optional: Maximum crawl depth (default: 0 = unlimited)
    MaxDepth: 5,

    // Optional: Maximum pages to crawl (default: 0 = unlimited)
    MaxPages: 1000,

    // Optional: Concurrent requests (default: 2)
    Concurrency: 4,

    // Optional: Delay between requests (default: 100ms)
    RequestDelay: 200 * time.Millisecond,

    // Optional: Custom User-Agent string
    UserAgent: "MyBot/1.0",

    // Optional: Enable sitemap-based URL discovery
    UseSitemap: true,

    // Optional: Custom sitemap URL (default: /sitemap.xml)
    SitemapURL: "https://docs.example.com/sitemap.xml",

    // Optional: Only crawl URLs from sitemap, disable link discovery
    SitemapOnly: false,

    // Optional: Respect robots.txt directives (default: true in colly)
    RespectRobotsTxt: true,

    // Optional: Number of retry attempts for failed requests (default: 3)
    MaxRetries: 3,

    // Optional: Base delay for exponential backoff (default: 1s)
    RetryDelay: 1 * time.Second,

    // Optional: Enable response caching with TTL (default: 0 = disabled)
    CacheTTL: 5 * time.Minute,

    // Optional: Maximum cached items (default: 1000)
    CacheMaxItems: 500,

    // Optional: Enable URL normalization (default: true)
    NormalizeURLs: true,
})
```

### GitSource

Clones Git repositories and traverses their contents. Supports GitHub/GitLab shorthand, authentication, and branch/tag selection.

```go
// Using GitHub shorthand
source, err := docsaf.NewGitSource(docsaf.GitSourceConfig{
    // GitHub shorthand - automatically expanded to https://github.com/owner/repo.git
    URL: "owner/repo",

    // Optional: Branch, tag, or commit to checkout (default: default branch)
    Ref: "main",
})

// Full configuration
source, err := docsaf.NewGitSource(docsaf.GitSourceConfig{
    // Required: Git repository URL
    // Supports: https://, git://, git@, ssh://, or GitHub shorthand (owner/repo)
    URL: "https://github.com/owner/repo.git",

    // Optional: Branch, tag, or commit to checkout
    Ref: "v1.0.0",

    // Optional: Base URL for document links (auto-derived for GitHub/GitLab)
    BaseURL: "https://github.com/owner/repo/blob/v1.0.0",

    // Optional: Subdirectory to traverse (useful for monorepos)
    SubPath: "docs",

    // Optional: Glob patterns for files to include
    IncludePatterns: []string{"**/*.md", "**/*.html"},

    // Optional: Glob patterns for files to exclude (has defaults)
    ExcludePatterns: []string{".git/**", "node_modules/**"},

    // Optional: Use shallow clone with depth 1 (default: true)
    ShallowClone: true,

    // Optional: Directory to clone into (default: temp directory)
    CloneDir: "/path/to/clone",

    // Optional: Keep clone directory after traversal (default: false)
    KeepClone: false,

    // Optional: Authentication for private repositories
    Auth: &docsaf.GitAuth{
        Username: "user",
        Password: "token-or-password",
        // Or use SSH key:
        // SSHKeyPath: "/path/to/id_rsa",
    },
})
```

#### Processing a Git Repository

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/antflydb/antfly-go/docsaf"
)

func main() {
    // Clone and process a GitHub repository
    source, err := docsaf.NewGitSource(docsaf.GitSourceConfig{
        URL:             "owner/repo",
        Ref:             "main",
        SubPath:         "docs",
        IncludePatterns: []string{"**/*.md"},
    })
    if err != nil {
        log.Fatal(err)
    }

    processor := docsaf.NewProcessor(source, docsaf.DefaultRegistry())

    sections, err := processor.Process(context.Background())
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Processed %d sections from repository\n", len(sections))
}
```

## Content Processors

Processors extract structured content from raw bytes. Each processor implements:

```go
type ContentProcessor interface {
    CanProcess(contentType, path string) bool
    Process(path, sourceURL, baseURL string, content []byte) ([]DocumentSection, error)
}
```

### Built-in Processors

| Processor | File Extensions | Content Types |
|-----------|----------------|---------------|
| MarkdownProcessor | `.md`, `.markdown` | `text/markdown` |
| HTMLProcessor | `.html`, `.htm` | `text/html` |
| PDFProcessor | `.pdf` | `application/pdf` |
| OpenAPIProcessor | `.yaml`, `.yml`, `.json` (with OpenAPI content) | - |
| WholeFileProcessor | Any | Any (fallback) |

### Registries

```go
// Default registry: Markdown, OpenAPI, HTML, PDF
registry := docsaf.DefaultRegistry()

// Whole file registry: treats entire files as single sections
registry := docsaf.NewWholeFileRegistry()

// Custom registry
registry := docsaf.NewRegistry()
registry.Register(&docsaf.MarkdownProcessor{})
registry.Register(&MyCustomProcessor{})
```

### Custom Processors

```go
type MyProcessor struct{}

func (p *MyProcessor) CanProcess(contentType, path string) bool {
    return strings.HasSuffix(path, ".custom")
}

func (p *MyProcessor) Process(path, sourceURL, baseURL string, content []byte) ([]docsaf.DocumentSection, error) {
    // Parse content and return sections
    return []docsaf.DocumentSection{
        {
            Title:   "My Section",
            Content: string(content),
            URL:     baseURL + path,
        },
    }, nil
}
```

## DocumentSection

The output of processing is a slice of `DocumentSection`:

```go
type DocumentSection struct {
    Title    string         // Section title (e.g., heading text)
    Content  string         // Section content as text
    URL      string         // Full URL to the section (with anchor if applicable)
    Metadata map[string]any // Additional metadata (source_type, page_number, etc.)
}
```

## URL Normalization

When `NormalizeURLs` is enabled (default), URLs are canonicalized for consistent deduplication:

- Lowercase scheme and host: `HTTPS://Example.COM` → `https://example.com`
- Remove default ports: `https://example.com:443` → `https://example.com`
- Remove trailing slashes: `https://example.com/path/` → `https://example.com/path`
- Remove fragments: `https://example.com/path#section` → `https://example.com/path`
- Normalize empty paths: `https://example.com` → `https://example.com/`

## Retry Logic

Failed requests are automatically retried with exponential backoff:

- Network errors and 5xx status codes trigger retries
- Default: 3 retries with 1s base delay (1s, 2s, 4s)
- Maximum delay capped at 30 seconds
- Respects context cancellation

## Caching

### Simple In-Memory Cache

Enable basic in-memory response caching:

```go
source, _ := docsaf.NewWebSource(docsaf.WebSourceConfig{
    StartURL:      "https://docs.example.com",
    CacheTTL:      10 * time.Minute,  // Cache responses for 10 minutes
    CacheMaxItems: 500,               // Keep at most 500 responses
})
```

### Advanced Caching Features

Enable HTTP-aware caching with disk persistence and content deduplication:

```go
source, _ := docsaf.NewWebSource(docsaf.WebSourceConfig{
    StartURL: "https://docs.example.com",

    // Basic cache settings
    CacheTTL:      1 * time.Hour,
    CacheMaxItems: 1000,

    // Persistent disk cache
    CacheDir: "/tmp/docsaf-cache",

    // Respect HTTP cache headers (Cache-Control, ETag, Last-Modified)
    CacheRespectHeaders: true,

    // Deduplicate identical content across URLs
    CacheDeduplication: true,
})
```

### Cache Features

| Feature | Description |
|---------|-------------|
| **HTTP Headers** | Respects `Cache-Control`, `max-age`, `ETag`, `Last-Modified` |
| **Conditional Requests** | Sends `If-None-Match` and `If-Modified-Since` for revalidation |
| **Disk Persistence** | Cache survives restarts when `CacheDir` is set |
| **Content Deduplication** | Identical content stored once via SHA-256 hashing |
| **LRU Eviction** | Oldest entries removed when cache is full |

### Cache Management

```go
// Check cache statistics
stats := source.CacheStats()
if stats != nil {
    fmt.Printf("Cached entries: %d\n", stats.MemoryEntries)
    fmt.Printf("Unique contents: %d\n", stats.UniqueContents)
    fmt.Printf("Total size: %d bytes\n", stats.TotalSizeBytes)
}

// Clear the cache
source.ClearCache()

// Check if a URL is cached
if source.IsCached("https://docs.example.com/page") {
    fmt.Println("Page is cached")
}
```

## Pattern Matching

Include and exclude patterns use [doublestar](https://github.com/bmatcuk/doublestar) glob syntax:

| Pattern | Matches |
|---------|---------|
| `*.md` | Markdown files in current directory |
| `**/*.md` | Markdown files in any subdirectory |
| `/docs/**` | Everything under /docs path |
| `/api/*` | Direct children of /api (not nested) |
| `/**/*.{css,js}` | All CSS and JS files |

## Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌───────────────────┐
│  ContentSource  │────▶│    Processor     │────▶│ DocumentSection[] │
│                 │     │                  │     │                   │
│ - Filesystem    │     │ Uses registry to │     │ - Title           │
│ - Web           │     │ find appropriate │     │ - Content         │
│ - Git           │     │ ContentProcessor │     │ - URL             │
└─────────────────┘     └──────────────────┘     └───────────────────┘
                               │
                               ▼
                    ┌──────────────────────┐
                    │  ProcessorRegistry   │
                    │                      │
                    │ - MarkdownProcessor  │
                    │ - HTMLProcessor      │
                    │ - PDFProcessor       │
                    │ - OpenAPIProcessor   │
                    └──────────────────────┘
```

## License

See repository root for license information.
