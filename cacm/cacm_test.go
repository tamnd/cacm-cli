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
	if len(secs) != 4 {
		t.Fatalf("got %d sections, want 4", len(secs))
	}
	for i, s := range secs {
		if s.Name == "" {
			t.Errorf("sections[%d] has empty Name", i)
		}
		if s.Slug == "" {
			t.Errorf("sections[%d] %q has empty Slug", i, s.Name)
		}
		if s.URL == "" {
			t.Errorf("sections[%d] %q has empty URL", i, s.Name)
		}
		if s.Rank != i+1 {
			t.Errorf("sections[%d] Rank=%d, want %d", i, s.Rank, i+1)
		}
	}
	// technews uses a different base domain
	techNews := secs[3]
	if techNews.Slug != "technews" {
		t.Errorf("secs[3].Slug=%q, want technews", techNews.Slug)
	}
	if !strings.Contains(techNews.URL, "technews.acm.org") {
		t.Errorf("technews URL=%q, want technews.acm.org", techNews.URL)
	}
}

func TestFeedLimitZero(t *testing.T) {
	// limit=0 must return all items in the feed.
	body := fakeRSS(sampleRSSItem, sampleRSSItem, sampleRSSItem)
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	arts, err := c.Feed(context.Background(), "/feed/", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 3 {
		t.Errorf("got %d articles with limit=0, want 3", len(arts))
	}
}

func TestFeedURL(t *testing.T) {
	// FeedURL uses an absolute URL, ignoring BaseURL.
	body := fakeRSS(sampleRSSItem)
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	cfg := DefaultConfig()
	cfg.BaseURL = "http://should-not-be-used.invalid"
	cfg.Rate = 0
	cfg.Retries = 0
	c := NewClient(cfg)
	arts, err := c.FeedURL(context.Background(), srv.URL+"/technews/feed/", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 {
		t.Fatalf("got %d articles, want 1", len(arts))
	}
	if gotPath != "/technews/feed/" {
		t.Errorf("server saw path %q, want /technews/feed/", gotPath)
	}
}

func TestFeedRSSAuthorFallback(t *testing.T) {
	// When dc:creator is absent, <author> should be used.
	item := `<item>
<title>Fallback Author Article</title>
<link>https://cacm.acm.org/test/</link>
<pubDate>Sat, 14 Jun 2026 10:00:00 +0000</pubDate>
<author>fallback@example.com (Fallback Author)</author>
<description>Test description.</description>
</item>`
	body := fakeRSS(item)
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	arts, err := c.Feed(context.Background(), "/feed/", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 {
		t.Fatalf("got %d articles, want 1", len(arts))
	}
	if arts[0].Author == "" {
		t.Error("expected non-empty author from <author> fallback")
	}
}

func TestFeedHTMLEntities(t *testing.T) {
	// HTML entities in title and description should be decoded.
	item := `<item>
<title>S&amp;P 500 &lt;Rises&gt;</title>
<link>https://cacm.acm.org/test/</link>
<pubDate>Sat, 14 Jun 2026 10:00:00 +0000</pubDate>
<dc:creator>Jane Smith</dc:creator>
<description>Index rose &quot;quickly&quot; says &#39;source&#39;.</description>
</item>`
	body := fakeRSS(item)
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	arts, err := c.Feed(context.Background(), "/feed/", 0)
	if err != nil {
		t.Fatal(err)
	}
	if arts[0].Title != "S&P 500 <Rises>" {
		t.Errorf("title = %q", arts[0].Title)
	}
}

func TestAtomFallbackURL(t *testing.T) {
	// Atom entry with no <link> should use <id> as URL.
	entry := `<entry>
<title>No Link Entry</title>
<id>https://cacm.acm.org/fallback-id/</id>
<published>2026-06-14T10:00:00Z</published>
<author><name>Test Author</name></author>
<summary>Summary text.</summary>
</entry>`
	body := fakeAtom(entry)
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	arts, err := c.Feed(context.Background(), "/feed/", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 {
		t.Fatalf("got %d articles, want 1", len(arts))
	}
	if !strings.Contains(arts[0].URL, "fallback-id") {
		t.Errorf("URL fallback to <id> not used: %q", arts[0].URL)
	}
}

func TestAtomUpdatedFallback(t *testing.T) {
	// When <published> is absent, <updated> should be used for date.
	entry := `<entry>
<title>Updated Only Entry</title>
<link href="https://cacm.acm.org/updated-only/" rel="alternate"/>
<updated>2026-06-14T10:00:00Z</updated>
<author><name>Test Author</name></author>
<summary>Summary text.</summary>
</entry>`
	body := fakeAtom(entry)
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	arts, err := c.Feed(context.Background(), "/feed/", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 {
		t.Fatalf("got %d articles, want 1", len(arts))
	}
	if arts[0].Published != "2026-06-14 10:00" {
		t.Errorf("published = %q, want 2026-06-14 10:00", arts[0].Published)
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
