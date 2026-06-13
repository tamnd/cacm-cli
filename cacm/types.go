package cacm

import (
	"encoding/xml"
	"strings"
	"time"
)

// Article is the record emitted for a CACM article or blog post.
type Article struct {
	Rank        int    `json:"rank"`
	Title       string `json:"title"`
	Author      string `json:"author"`
	Published   string `json:"published"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

// Section is the record emitted by the sections command.
type Section struct {
	Rank int    `json:"rank"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// ─── RSS 2.0 wire types ───────────────────────────────────────────────────────

// rssItem maps a single <item> in RSS 2.0.
// dc:creator is the Dublin Core author element used by CACM WordPress feeds.
type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	Creator     string `xml:"creator"` // dc:creator
	Author      string `xml:"author"`  // plain <author>
	Description string `xml:"description"`
	GUID        string `xml:"guid"`
}

// rssFeed maps the top-level <rss> element.
type rssFeed struct {
	XMLName xml.Name  `xml:"rss"`
	Items   []rssItem `xml:"channel>item"`
}

// ─── Atom 1.0 wire types ──────────────────────────────────────────────────────

// atomEntry maps a single <entry> in Atom 1.0.
type atomEntry struct {
	Title     string     `xml:"title"`
	Links     []atomLink `xml:"link"`
	Published string     `xml:"published"`
	Updated   string     `xml:"updated"`
	Author    atomAuthor `xml:"author"`
	Summary   string     `xml:"summary"`
	Content   string     `xml:"content"`
	ID        string     `xml:"id"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

type atomAuthor struct {
	Name string `xml:"name"`
}

// atomFeed maps the top-level <feed> element (Atom 1.0).
type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Entries []atomEntry `xml:"entry"`
}

// ─── converters ──────────────────────────────────────────────────────────────

func rssItemToArticle(it rssItem, rank int) Article {
	u := strings.TrimSpace(it.Link)
	if u == "" {
		u = strings.TrimSpace(it.GUID)
	}
	author := strings.TrimSpace(it.Creator)
	if author == "" {
		author = strings.TrimSpace(it.Author)
	}
	return Article{
		Rank:        rank,
		Title:       strings.TrimSpace(it.Title),
		Author:      author,
		Published:   parseDate(it.PubDate),
		Description: stripTags(strings.TrimSpace(it.Description)),
		URL:         u,
	}
}

func atomEntryToArticle(e atomEntry, rank int) Article {
	u := ""
	// Prefer rel=alternate, fall back to first href.
	for _, l := range e.Links {
		if l.Rel == "alternate" || l.Rel == "" {
			u = l.Href
			break
		}
	}
	if u == "" && len(e.Links) > 0 {
		u = e.Links[0].Href
	}
	if u == "" {
		u = strings.TrimSpace(e.ID)
	}
	summary := strings.TrimSpace(e.Summary)
	if summary == "" {
		summary = strings.TrimSpace(e.Content)
	}
	pub := e.Published
	if pub == "" {
		pub = e.Updated
	}
	return Article{
		Rank:        rank,
		Title:       strings.TrimSpace(e.Title),
		Author:      strings.TrimSpace(e.Author.Name),
		Published:   parseDate(pub),
		Description: stripTags(summary),
		URL:         u,
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// parseDate normalizes date strings from both RSS (RFC1123Z/RFC1123) and Atom
// (RFC3339) into "2006-01-02 15:04" UTC. Falls back to the raw string on failure.
func parseDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	formats := []string{
		time.RFC1123Z, // "Mon, 02 Jan 2006 15:04:05 -0700"
		time.RFC1123,  // "Mon, 02 Jan 2006 15:04:05 MST"
		time.RFC3339,  // "2006-01-02T15:04:05Z07:00"
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05 -0700",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC().Format("2006-01-02 15:04")
		}
	}
	return s
}

// stripTags removes HTML tags from s and unescapes common HTML entities.
func stripTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	out := b.String()
	out = strings.ReplaceAll(out, "&amp;", "&")
	out = strings.ReplaceAll(out, "&lt;", "<")
	out = strings.ReplaceAll(out, "&gt;", ">")
	out = strings.ReplaceAll(out, "&quot;", `"`)
	out = strings.ReplaceAll(out, "&#39;", "'")
	out = strings.ReplaceAll(out, "&apos;", "'")
	out = strings.ReplaceAll(out, "&#8217;", "'")
	out = strings.ReplaceAll(out, "&#8220;", `"`)
	out = strings.ReplaceAll(out, "&#8221;", `"`)
	out = strings.TrimSpace(out)
	return out
}

// knownSections is the canonical list of CACM feed sections.
var knownSections = []Section{
	{Rank: 1, Name: "top", URL: "https://cacm.acm.org/feed/"},
	{Rank: 2, Name: "blogs", URL: "https://cacm.acm.org/blogs/feed/"},
	{Rank: 3, Name: "magazine", URL: "https://cacm.acm.org/magazines/feed/"},
}
