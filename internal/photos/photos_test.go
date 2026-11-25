package photos

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const commonsBody = `{"query":{"pages":{
 "1":{"index":1,"title":"File:A.jpg","imageinfo":[{"url":"FULL_A","thumburl":"THUMB_A","descriptionurl":"PAGE_A","width":800,"height":600,"mime":"image/jpeg","extmetadata":{"LicenseShortName":{"value":"CC BY-SA 4.0"},"Artist":{"value":"<a href=\"x\">Tác giả A</a>"},"LicenseUrl":{"value":"https://creativecommons.org/licenses/by-sa/4.0"}}}]},
 "2":{"index":2,"title":"File:B.jpg","imageinfo":[{"url":"FULL_B","thumburl":"THUMB_B","descriptionurl":"PAGE_B","width":800,"height":600,"mime":"image/jpeg","extmetadata":{"LicenseShortName":{"value":"CC BY-NC 2.0"},"Artist":{"value":"B"},"LicenseUrl":{"value":""}}}]},
 "3":{"index":3,"title":"File:C.jpg","imageinfo":[{"url":"FULL_C","thumburl":"THUMB_C","descriptionurl":"PAGE_C","width":800,"height":600,"mime":"image/jpeg","extmetadata":{"LicenseShortName":{"value":"Public domain"},"Artist":{"value":"C"},"LicenseUrl":{"value":""}}}]}
}}}`

func commonsServer(t *testing.T, body string) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &Client{HTTP: srv.Client(), BaseURL: srv.URL}
}

func TestSearchFiltersLicenses(t *testing.T) {
	_, c := commonsServer(t, commonsBody)
	got, err := c.Search(context.Background(), "bronze drum", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 allowed results, got %d: %+v", len(got), got)
	}
	for _, cand := range got {
		if strings.Contains(cand.License, "NC") || strings.Contains(cand.License, "ND") {
			t.Fatalf("NC/ND license leaked: %q", cand.License)
		}
	}
	if got[0].Title != "File:A.jpg" || got[0].Author != "Tác giả A" {
		t.Fatalf("first result: %+v", got[0])
	}
}

func TestSearchEmptyIsNotError(t *testing.T) {
	_, c := commonsServer(t, `{"batchcomplete":""}`)
	got, err := c.Search(context.Background(), "không có", 5)
	if err != nil {
		t.Fatalf("empty result must not be an error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want 0 results, got %d", len(got))
	}
}

func TestSearchHTTPErrorIsVietnamese(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	_, err := c.Search(context.Background(), "x", 5)
	if err == nil || !strings.Contains(err.Error(), "Không kết nối được Wikimedia Commons") {
		t.Fatalf("want Vietnamese connection error, got %v", err)
	}
}

func TestStripHTMLArtist(t *testing.T) {
	if got := StripHTML(`<a href="x">Tác giả</a>`); got != "Tác giả" {
		t.Fatalf("StripHTML = %q", got)
	}
}

func jpegBytes() []byte {
	return append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{1}, 64)...)
}

func imageServer(t *testing.T, contentType string, body []byte) (*httptest.Server, Candidate) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, Candidate{Title: "File:A.jpg", Author: "Tác giả A", License: "CC BY-SA 4.0",
		PageURL: "https://commons.wikimedia.org/wiki/File:A.jpg", FullURL: srv.URL + "/a.jpg"}
}

func TestDownloadRejectsLargeAndNonImage(t *testing.T) {
	c := &Client{HTTP: http.DefaultClient}
	cases := map[string]struct {
		ct   string
		body []byte
	}{
		"svg":   {"image/svg+xml", []byte("<svg/>")},
		"large": {"image/jpeg", make([]byte, maxBytes+1)},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			_, cand := imageServer(t, tc.ct, tc.body)
			if _, err := c.Download(context.Background(), cand, dir); err == nil {
				t.Fatal("want error")
			}
			if entries, _ := os.ReadDir(filepath.Join(dir, "photos")); len(entries) != 0 {
				t.Fatalf("file written despite error: %v", entries)
			}
		})
	}
}

func TestDownloadWritesSHAName(t *testing.T) {
	dir := t.TempDir()
	body := jpegBytes()
	_, cand := imageServer(t, "image/jpeg", body)
	f, err := (&Client{HTTP: http.DefaultClient}).Download(context.Background(), cand, dir)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	want := hex.EncodeToString(sum[:])
	if f.SHA256 != want || f.Rel != "photos/"+want+".jpg" {
		t.Fatalf("file = %+v, want sha %s", f, want)
	}
	if _, err := os.Stat(filepath.Join(dir, f.Rel)); err != nil {
		t.Fatal(err)
	}
}

func TestAddRefIsIdempotentPerSHA(t *testing.T) {
	dir := t.TempDir()
	ref := Ref{SHA256: "abc", File: "photos/abc.jpg", Credit: "Ảnh: X", PageURL: "p"}
	for i := 0; i < 2; i++ {
		if err := AddRef(dir, "hook", ref); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := Index(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx["hook"]) != 1 {
		t.Fatalf("want 1 ref, got %+v", idx["hook"])
	}
}
