package cacm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeRSS builds a minimal RSS 2.0 document containing the given raw <item> blobs.
func fakeRSS(items ...string) string {
	header := `<?xml version="1.0" encoding="UTF-8"?><rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/"><channel><title>CACM</title>`
	footer := `</channel></rss>`
	return header + strings.Join(items, "") + footer
}

// fakeAtom builds a minimal Atom 1.0 document containing the given raw <entry> blobs.
func fakeAtom(entries ...string) string {
	header := `<?xml version="1.0" encoding="UTF-8"?><feed xmlns="http://www.w3.org/2005/Atom">`
	footer := `</feed>`
	return header + strings.Join(entries, "") + footer
}

const sampleRSSItem = `<item>
<title>Test Article Title</title>
<link>https://cacm.acm.org/magazines/2026/6/test/</link>
<pubDate>Sat, 14 Jun 2026 10:00:00 +0000</pubDate>
<dc:creator>Jane Smith</dc:creator>
<description>Short description of the article.</description>
</item>`

const sampleAtomEntry = `<entry>
<title>Test Atom Article</title>
<link href="https://cacm.acm.org/magazines/2026/6/atom-test/" rel="alternate"/>
<published>2026-06-14T10:00:00Z</published>
<author><name>John Doe</name></author>
<summary>Atom summary text.</summary>
</entry>`

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 3
	return NewClient(cfg)
}

func TestFeedRSS(t *testing.T) {
	body := fakeRSS(sampleRSSItem)
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(body))
	})
	arts, err := c.Feed(context.Background(), "/feed/", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 {
		t.Fatalf("got %d articles, want 1", len(arts))
	}
	a := arts[0]
	if a.Title != "Test Article Title" {
		t.Errorf("title = %q", a.Title)
	}
	if a.Author != "Jane Smith" {
		t.Errorf("author = %q", a.Author)
	}
	if a.URL != "https://cacm.acm.org/magazines/2026/6/test/" {
		t.Errorf("url = %q", a.URL)
	}
	if a.Published != "2026-06-14 10:00" {
		t.Errorf("published = %q", a.Published)
	}
	if a.Description != "Short description of the article." {
		t.Errorf("description = %q", a.Description)
	}
	if a.Rank != 1 {
		t.Errorf("rank = %d, want 1", a.Rank)
	}
}

func TestFeedAtom(t *testing.T) {
	body := fakeAtom(sampleAtomEntry)
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(body))
	})
	arts, err := c.Feed(context.Background(), "/magazines/feed/", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 {
		t.Fatalf("got %d articles, want 1", len(arts))
	}
	a := arts[0]
	if a.Title != "Test Atom Article" {
		t.Errorf("title = %q", a.Title)
	}
	if a.Author != "John Doe" {
		t.Errorf("author = %q", a.Author)
	}
	if a.URL != "https://cacm.acm.org/magazines/2026/6/atom-test/" {
		t.Errorf("url = %q", a.URL)
	}
	if a.Published != "2026-06-14 10:00" {
		t.Errorf("published = %q", a.Published)
	}
}

func TestFeedLimit(t *testing.T) {
	body := fakeRSS(sampleRSSItem, sampleRSSItem, sampleRSSItem, sampleRSSItem, sampleRSSItem)
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	arts, err := c.Feed(context.Background(), "/feed/", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 2 {
		t.Fatalf("got %d articles with limit=2, want 2", len(arts))
	}
}

func TestFeedSendsUserAgent(t *testing.T) {
	var gotUA string
	body := fakeRSS(sampleRSSItem)
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(body))
	})
	_, err := c.Feed(context.Background(), "/feed/", 0)
	if err != nil {
		t.Fatal(err)
	}
	if gotUA == "" {
		t.Error("request carried no User-Agent")
	}
}

func TestFeedRetriesOn503(t *testing.T) {
	var hits int
	body := fakeRSS(sampleRSSItem)
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(body))
	})
	start := time.Now()
	arts, err := c.Feed(context.Background(), "/feed/", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) == 0 {
		t.Error("got no articles after retries")
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestFeedHTMLResponse(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>Blocked</title></head></html>`
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	})
	_, err := c.Feed(context.Background(), "/feed/", 0)
	if err == nil {
		t.Fatal("expected ErrHTMLResponse, got nil")
	}
	if err != ErrHTMLResponse {
		t.Errorf("got error %v, want ErrHTMLResponse", err)
	}
}

func TestFeedInvalidXML(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<?xml version='1.0'?><broken><unclosed>"))
	})
	_, err := c.Feed(context.Background(), "/feed/", 0)
	if err == nil {
		t.Error("expected error for invalid XML, got nil")
	}
}

func TestFeedEmpty(t *testing.T) {
	body := fakeRSS() // no items
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	arts, err := c.Feed(context.Background(), "/feed/", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 0 {
		t.Errorf("got %d articles from empty feed, want 0", len(arts))
	}
}

func TestKnownSections(t *testing.T) {
	secs := KnownSections()
	if len(secs) != 3 {
		t.Fatalf("got %d sections, want 3", len(secs))
	}
	for _, s := range secs {
		if s.Name == "" {
			t.Error("section has empty name")
		}
		if s.URL == "" {
			t.Errorf("section %q has empty URL", s.Name)
		}
		if s.Rank == 0 {
			t.Errorf("section %q has zero rank", s.Name)
		}
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Sat, 14 Jun 2026 10:00:00 +0000", "2026-06-14 10:00"},
		{"Mon, 01 Jan 2024 00:00:00 GMT", "2024-01-01 00:00"},
		{"2026-06-14T10:00:00Z", "2026-06-14 10:00"},
		{"", ""},
		{"not a date", "not a date"},
	}
	for _, tc := range cases {
		got := parseDate(tc.in)
		if got != tc.want {
			t.Errorf("parseDate(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
