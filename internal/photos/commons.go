// Package photos finds Wikimedia Commons images for a scene, keeps only licences the channel may
// use, and stores the chosen file with its credit inside the video folder.
package photos

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://commons.wikimedia.org/w/api.php"
	userAgent      = "clip-publisher/1.0 (desktop; no-contact)"
)

// Candidate is one Commons image offered to the owner.
type Candidate struct {
	Title      string `json:"title"`
	Author     string `json:"author"`
	License    string `json:"license"`
	LicenseURL string `json:"licenseUrl"`
	PageURL    string `json:"pageUrl"`
	ThumbURL   string `json:"thumbUrl"`
	FullURL    string `json:"fullUrl"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

// Client talks to the Commons API. BaseURL is replaceable in tests.
type Client struct {
	HTTP    *http.Client
	BaseURL string
}

func (c *Client) base() string {
	if c.BaseURL == "" {
		return defaultBaseURL
	}
	return c.BaseURL
}

func (c *Client) client() *http.Client {
	if c.HTTP == nil {
		return &http.Client{Timeout: 15 * time.Second}
	}
	return c.HTTP
}

type commonsImage struct {
	URL            string `json:"url"`
	ThumbURL       string `json:"thumburl"`
	DescriptionURL string `json:"descriptionurl"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	Mime           string `json:"mime"`
	ExtMetadata    map[string]struct {
		Value string `json:"value"`
	} `json:"extmetadata"`
}

type commonsPage struct {
	Index     int            `json:"index"`
	Title     string         `json:"title"`
	ImageInfo []commonsImage `json:"imageinfo"`
}

type commonsResp struct {
	Query struct {
		Pages map[string]commonsPage `json:"pages"`
	} `json:"query"`
}

// Search returns up to limit images for query whose licence and format are allowed. An empty
// result is not an error.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Candidate, error) {
	q := url.Values{}
	q.Set("action", "query")
	q.Set("generator", "search")
	q.Set("gsrnamespace", "6")
	q.Set("gsrlimit", strconv.Itoa(limit))
	q.Set("gsrsearch", query)
	q.Set("prop", "imageinfo")
	q.Set("iiprop", "url|size|mime|extmetadata")
	q.Set("iiurlwidth", "640")
	q.Set("format", "json")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("Không kết nối được Wikimedia Commons: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Không kết nối được Wikimedia Commons (HTTP %d)", resp.StatusCode)
	}
	var body commonsResp
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&body); err != nil {
		return nil, fmt.Errorf("Phản hồi của Wikimedia Commons không đọc được: %w", err)
	}

	pages := make([]commonsPage, 0, len(body.Query.Pages))
	for _, p := range body.Query.Pages {
		pages = append(pages, p)
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Index < pages[j].Index })

	out := []Candidate{}
	for _, p := range pages {
		if len(p.ImageInfo) == 0 {
			continue
		}
		ii := p.ImageInfo[0]
		if ii.Mime != "image/jpeg" && ii.Mime != "image/png" {
			continue
		}
		lic := ii.ExtMetadata["LicenseShortName"].Value
		if !AllowedLicense(lic) {
			continue
		}
		out = append(out, Candidate{
			Title:      p.Title,
			Author:     StripHTML(ii.ExtMetadata["Artist"].Value),
			License:    lic,
			LicenseURL: ii.ExtMetadata["LicenseUrl"].Value,
			PageURL:    ii.DescriptionURL,
			ThumbURL:   ii.ThumbURL,
			FullURL:    ii.URL,
			Width:      ii.Width,
			Height:     ii.Height,
		})
	}
	return out, nil
}

// AllowedLicense reports whether a Commons licence short name is one the channel may use:
// CC0, public domain, CC BY, CC BY-SA. Anything with NC or ND is refused.
func AllowedLicense(short string) bool {
	if strings.Contains(short, "NC") || strings.Contains(short, "ND") {
		return false
	}
	return short == "Public domain" || strings.HasPrefix(short, "CC0") ||
		strings.HasPrefix(short, "CC BY ") || strings.HasPrefix(short, "CC BY-SA ")
}

var tagRe = regexp.MustCompile(`<[^>]*>`)

// StripHTML removes tags and unescapes entities from Commons metadata such as the artist field.
func StripHTML(s string) string {
	return strings.TrimSpace(html.UnescapeString(tagRe.ReplaceAllString(s, "")))
}

// Credit is the line written into script sources for a chosen image.
func Credit(c Candidate) string {
	author := c.Author
	if author == "" {
		author = "không rõ tác giả"
	}
	return fmt.Sprintf("Ảnh: %s, %s, %s", author, c.License, c.PageURL)
}
