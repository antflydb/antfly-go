package docsaf

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/gocolly/colly/v2"
)

// WebSourceConfig holds configuration for a WebSource.
type WebSourceConfig struct {
	// StartURL is the starting URL to crawl (required)
	StartURL string

	// BaseURL is the base URL for generating document links.
	// If empty, it will be derived from StartURL.
	BaseURL string

	// AllowedDomains restricts crawling to these domains.
	// If empty, only the domain from StartURL is allowed.
	AllowedDomains []string

	// IncludePatterns is a list of glob patterns for URL paths to include.
	// If empty, all paths are included (subject to exclude patterns).
	// Patterns match against the URL path (e.g., "/docs/**", "/guides/*")
	IncludePatterns []string

	// ExcludePatterns is a list of glob patterns for URL paths to exclude.
	// Default excludes common non-content paths.
	ExcludePatterns []string

	// MaxDepth is the maximum crawl depth (0 = unlimited).
	MaxDepth int

	// MaxPages is the maximum number of pages to crawl (0 = unlimited).
	MaxPages int

	// Concurrency is the number of concurrent requests (default: 2).
	Concurrency int

	// RequestDelay is the delay between requests (default: 100ms).
	RequestDelay time.Duration

	// UserAgent is the User-Agent string to use for requests.
	UserAgent string

	// UseSitemap enables sitemap-based crawling.
	// When enabled, the crawler will first fetch and parse the sitemap
	// to discover URLs before following links.
	UseSitemap bool

	// SitemapURL is the URL of the sitemap (optional).
	// If empty and UseSitemap is true, it will try /sitemap.xml
	SitemapURL string

	// SitemapOnly restricts crawling to URLs found in the sitemap only.
	// When true, link discovery is disabled.
	SitemapOnly bool

	// RespectRobotsTxt enables robots.txt parsing (default: true).
	RespectRobotsTxt bool
}

// WebSource crawls websites and yields content items.
type WebSource struct {
	config WebSourceConfig
}

// NewWebSource creates a new web content source.
func NewWebSource(config WebSourceConfig) (*WebSource, error) {
	if config.StartURL == "" {
		return nil, fmt.Errorf("StartURL is required")
	}

	// Parse start URL to extract domain
	parsedURL, err := url.Parse(config.StartURL)
	if err != nil {
		return nil, fmt.Errorf("invalid StartURL: %w", err)
	}

	// Set defaults
	if config.BaseURL == "" {
		config.BaseURL = parsedURL.Scheme + "://" + parsedURL.Host
	}

	if len(config.AllowedDomains) == 0 {
		config.AllowedDomains = []string{parsedURL.Host}
	}

	if config.Concurrency == 0 {
		config.Concurrency = 2
	}

	if config.RequestDelay == 0 {
		config.RequestDelay = 100 * time.Millisecond
	}

	if config.UserAgent == "" {
		config.UserAgent = "Antfly-Docsaf/1.0 (+https://antfly.co)"
	}

	// Default exclude patterns for common non-content paths
	if len(config.ExcludePatterns) == 0 {
		config.ExcludePatterns = []string{
			"/api/**",
			"/search/**",
			"/**/*.css",
			"/**/*.js",
			"/**/*.png",
			"/**/*.jpg",
			"/**/*.jpeg",
			"/**/*.gif",
			"/**/*.svg",
			"/**/*.ico",
			"/**/*.woff",
			"/**/*.woff2",
			"/**/*.ttf",
			"/**/*.eot",
		}
	}

	return &WebSource{config: config}, nil
}

// Type returns "web" as the source type.
func (ws *WebSource) Type() string {
	return "web"
}

// BaseURL returns the base URL for this source.
func (ws *WebSource) BaseURL() string {
	return ws.config.BaseURL
}

// Traverse crawls the website and yields content items.
func (ws *WebSource) Traverse(ctx context.Context) (<-chan ContentItem, <-chan error) {
	items := make(chan ContentItem)
	errs := make(chan error, 1)

	go func() {
		defer close(items)
		defer close(errs)

		visited := &sync.Map{}
		pageCount := 0
		var mu sync.Mutex
		done := false

		// Create collector
		c := colly.NewCollector(
			colly.AllowedDomains(ws.config.AllowedDomains...),
			colly.Async(true),
			colly.MaxDepth(ws.config.MaxDepth),
		)

		// Set limits
		c.Limit(&colly.LimitRule{
			DomainGlob:  "*",
			Parallelism: ws.config.Concurrency,
			Delay:       ws.config.RequestDelay,
		})

		// Set user agent
		c.UserAgent = ws.config.UserAgent

		// Handle HTML responses
		c.OnResponse(func(r *colly.Response) {
			// Check for cancellation
			select {
			case <-ctx.Done():
				return
			default:
			}

			mu.Lock()
			if done || (ws.config.MaxPages > 0 && pageCount >= ws.config.MaxPages) {
				mu.Unlock()
				return
			}
			pageCount++
			mu.Unlock()

			urlStr := r.Request.URL.String()
			path := r.Request.URL.Path
			if path == "" {
				path = "/"
			}

			// Check if we've already processed this URL
			if _, loaded := visited.LoadOrStore(urlStr, true); loaded {
				return
			}

			// Check include/exclude patterns
			if !ws.shouldIncludePath(path) {
				return
			}

			contentType := r.Headers.Get("Content-Type")

			// Only process HTML content
			if !strings.Contains(contentType, "text/html") {
				return
			}

			select {
			case items <- ContentItem{
				Path:        path,
				SourceURL:   urlStr,
				Content:     r.Body,
				ContentType: contentType,
				Metadata: map[string]any{
					"source_type":  "web",
					"status_code":  r.StatusCode,
					"content_type": contentType,
					"depth":        r.Request.Depth,
				},
			}:
			case <-ctx.Done():
				mu.Lock()
				done = true
				mu.Unlock()
				return
			}
		})

		// Follow links (unless sitemap-only mode)
		if !ws.config.SitemapOnly {
			c.OnHTML("a[href]", func(e *colly.HTMLElement) {
				select {
				case <-ctx.Done():
					return
				default:
				}

				mu.Lock()
				if done || (ws.config.MaxPages > 0 && pageCount >= ws.config.MaxPages) {
					mu.Unlock()
					return
				}
				mu.Unlock()

				link := e.Attr("href")
				if link == "" {
					return
				}

				// Skip fragment-only links, javascript, mailto, tel
				if strings.HasPrefix(link, "#") ||
					strings.HasPrefix(link, "javascript:") ||
					strings.HasPrefix(link, "mailto:") ||
					strings.HasPrefix(link, "tel:") {
					return
				}

				// Resolve relative URLs
				absURL := e.Request.AbsoluteURL(link)
				if absURL == "" {
					return
				}

				// Parse URL to check path
				parsedURL, err := url.Parse(absURL)
				if err != nil {
					return
				}

				// Check include/exclude patterns
				if !ws.shouldIncludePath(parsedURL.Path) {
					return
				}

				// Remove fragment for deduplication
				parsedURL.Fragment = ""
				cleanURL := parsedURL.String()

				// Check if already visited
				if _, loaded := visited.Load(cleanURL); loaded {
					return
				}

				e.Request.Visit(cleanURL)
			})
		}

		// Handle errors
		c.OnError(func(r *colly.Response, err error) {
			log.Printf("Warning: Failed to fetch %s: %v", r.Request.URL, err)
		})

		// If using sitemap, fetch and process it first
		if ws.config.UseSitemap {
			sitemapURLs, err := ws.fetchSitemap(ctx)
			if err != nil {
				log.Printf("Warning: Failed to fetch sitemap: %v", err)
			} else {
				for _, u := range sitemapURLs {
					select {
					case <-ctx.Done():
						errs <- ctx.Err()
						return
					default:
					}

					mu.Lock()
					if done || (ws.config.MaxPages > 0 && pageCount >= ws.config.MaxPages) {
						mu.Unlock()
						break
					}
					mu.Unlock()

					c.Visit(u)
				}
			}
		}

		// Start crawling from start URL (if not sitemap-only or sitemap failed)
		if !ws.config.SitemapOnly {
			c.Visit(ws.config.StartURL)
		}

		// Wait for all requests to complete
		c.Wait()

		// Check for context cancellation
		if ctx.Err() != nil {
			errs <- ctx.Err()
		}
	}()

	return items, errs
}

// shouldIncludePath checks if a URL path should be included based on patterns.
func (ws *WebSource) shouldIncludePath(path string) bool {
	if path == "" {
		path = "/"
	}

	// Check exclude patterns first
	for _, pattern := range ws.config.ExcludePatterns {
		matched, err := doublestar.Match(pattern, path)
		if err != nil {
			continue
		}
		if matched {
			return false
		}
	}

	// If no include patterns, include everything (that wasn't excluded)
	if len(ws.config.IncludePatterns) == 0 {
		return true
	}

	// Check include patterns
	for _, pattern := range ws.config.IncludePatterns {
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

// Sitemap types for XML parsing
type sitemapIndex struct {
	XMLName  xml.Name         `xml:"sitemapindex"`
	Sitemaps []sitemapLocator `xml:"sitemap"`
}

type sitemapLocator struct {
	Loc string `xml:"loc"`
}

type urlset struct {
	XMLName xml.Name     `xml:"urlset"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod"`
	ChangeFreq string `xml:"changefreq"`
	Priority   string `xml:"priority"`
}

// fetchSitemap fetches and parses the sitemap, returning all URLs.
func (ws *WebSource) fetchSitemap(ctx context.Context) ([]string, error) {
	sitemapURL := ws.config.SitemapURL
	if sitemapURL == "" {
		// Try default sitemap location
		parsedBase, _ := url.Parse(ws.config.BaseURL)
		parsedBase.Path = "/sitemap.xml"
		sitemapURL = parsedBase.String()
	}

	return ws.fetchSitemapRecursive(ctx, sitemapURL, make(map[string]bool))
}

// fetchSitemapRecursive fetches a sitemap and any nested sitemaps.
func (ws *WebSource) fetchSitemapRecursive(ctx context.Context, sitemapURL string, visited map[string]bool) ([]string, error) {
	if visited[sitemapURL] {
		return nil, nil
	}
	visited[sitemapURL] = true

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", sitemapURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ws.config.UserAgent)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sitemap returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var urls []string

	// Try parsing as sitemap index first
	var index sitemapIndex
	if err := xml.Unmarshal(body, &index); err == nil && len(index.Sitemaps) > 0 {
		// This is a sitemap index, fetch each nested sitemap
		for _, sm := range index.Sitemaps {
			nestedURLs, err := ws.fetchSitemapRecursive(ctx, sm.Loc, visited)
			if err != nil {
				log.Printf("Warning: Failed to fetch nested sitemap %s: %v", sm.Loc, err)
				continue
			}
			urls = append(urls, nestedURLs...)
		}
		return urls, nil
	}

	// Try parsing as urlset
	var urlSet urlset
	if err := xml.Unmarshal(body, &urlSet); err != nil {
		return nil, fmt.Errorf("failed to parse sitemap: %w", err)
	}

	// Filter URLs based on include/exclude patterns
	for _, u := range urlSet.URLs {
		parsedURL, err := url.Parse(u.Loc)
		if err != nil {
			continue
		}

		if ws.shouldIncludePath(parsedURL.Path) {
			urls = append(urls, u.Loc)
		}
	}

	return urls, nil
}
