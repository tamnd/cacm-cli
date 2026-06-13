// Package cacm is the library behind the cacm command: the HTTP client,
// RSS/Atom parser, and typed data models for Communications of the ACM feeds.
//
// All data comes from the public CACM RSS and Atom feeds at cacm.acm.org.
// No API key or authentication is required.
package cacm

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DefaultUserAgent identifies the client to CACM servers.
const DefaultUserAgent = "cacm/dev (+https://github.com/tamnd/cacm-cli)"

// ErrHTMLResponse is returned when the server responds with HTML instead of
// XML. This typically means a Cloudflare challenge page is blocking the request.
var ErrHTMLResponse = errors.New("cacm: response is HTML (Cloudflare challenge?), try a different network")

// Config holds constructor parameters for Client.
type Config struct {
	// BaseURL is the scheme+host prefix for all feed URLs.
	// Default: "https://cacm.acm.org"
	BaseURL   string
	UserAgent string
	// Rate is the minimum gap between requests. Zero means no pacing.
	Rate    time.Duration
	Retries int
	Timeout time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://cacm.acm.org",
		UserAgent: DefaultUserAgent,
		Rate:      500 * time.Millisecond,
		Retries:   3,
		Timeout:   30 * time.Second,
	}
}

// Client talks to the CACM RSS/Atom feeds.
type Client struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
	rate       time.Duration
	retries    int
	mu         sync.Mutex
	last       time.Time
}

// NewClient returns a Client with the given config.
func NewClient(cfg Config) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: cfg.Timeout},
		baseURL:    cfg.BaseURL,
		userAgent:  cfg.UserAgent,
		rate:       cfg.Rate,
		retries:    cfg.Retries,
	}
}

// Feed fetches the RSS/Atom feed at path (e.g. "/feed/") relative to BaseURL
// and returns up to limit Article records. limit=0 returns all items.
func (c *Client) Feed(ctx context.Context, path string, limit int) ([]Article, error) {
	u := c.baseURL + path
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	return parseArticles(body, limit)
}

// ─── parsing ─────────────────────────────────────────────────────────────────

func parseArticles(body []byte, limit int) ([]Article, error) {
	// Detect HTML (Cloudflare block or other non-XML response).
	trimmed := strings.TrimSpace(string(body))
	lower := strings.ToLower(trimmed)
	if len(lower) > 100 {
		lower = lower[:100]
	}
	if strings.HasPrefix(lower, "<!doctype") || strings.HasPrefix(lower, "<html") {
		return nil, ErrHTMLResponse
	}

	// Try RSS 2.0 first.
	var rss rssFeed
	if err := xml.Unmarshal(body, &rss); err == nil && len(rss.Items) > 0 {
		return rssToArticles(rss.Items, limit), nil
	}

	// Try Atom 1.0.
	var atom atomFeed
	if err := xml.Unmarshal(body, &atom); err == nil && len(atom.Entries) > 0 {
		return atomToArticles(atom.Entries, limit), nil
	}

	// Valid empty RSS feed.
	var rssCheck rssFeed
	if err := xml.Unmarshal(body, &rssCheck); err == nil {
		return nil, nil
	}

	// Valid empty Atom feed.
	var atomCheck atomFeed
	if err := xml.Unmarshal(body, &atomCheck); err == nil {
		return nil, nil
	}

	return nil, fmt.Errorf("cacm: unrecognized feed format")
}

func rssToArticles(items []rssItem, limit int) []Article {
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	out := make([]Article, len(items))
	for i, it := range items {
		out[i] = rssItemToArticle(it, i+1)
	}
	return out
}

func atomToArticles(entries []atomEntry, limit int) []Article {
	if limit > 0 && limit < len(entries) {
		entries = entries[:limit]
	}
	out := make([]Article, len(entries))
	for i, e := range entries {
		out[i] = atomEntryToArticle(e, i+1)
	}
	return out
}

// KnownSections returns all built-in Section records in a stable order.
func KnownSections() []Section {
	out := make([]Section, len(knownSections))
	copy(out, knownSections)
	return out
}

// ─── HTTP internals ───────────────────────────────────────────────────────────

func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/xml, application/rss+xml, application/atom+xml, text/xml")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rate <= 0 {
		return
	}
	if wait := c.rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
