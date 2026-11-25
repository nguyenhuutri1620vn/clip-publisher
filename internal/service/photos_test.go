package service

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"clip-publisher/internal/photos"
)

// photoServer plays Wikimedia Commons: one allowed JPEG, one NC-licensed image, and the file itself.
func photoServer(t *testing.T, imgMime string) (*httptest.Server, *photos.Client) {
	t.Helper()
	var base string
	mux := http.NewServeMux()
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"query":{"pages":{
		"1":{"index":1,"title":"File:A.jpg","imageinfo":[{"url":"%[1]s/img.jpg","descriptionurl":"https://commons.example/File:A.jpg","width":1,"height":1,"mime":"image/jpeg","extmetadata":{"LicenseShortName":{"value":"CC BY-SA 4.0"},"Artist":{"value":"<a>Tác giả X</a>"}}}]},
		"2":{"index":2,"title":"File:B.jpg","imageinfo":[{"url":"%[1]s/b.jpg","descriptionurl":"https://commons.example/File:B.jpg","width":1,"height":1,"mime":"image/jpeg","extmetadata":{"LicenseShortName":{"value":"CC BY-NC 2.0"},"Artist":{"value":"B"}}}]}
		}}}`, base)
	})
	mux.HandleFunc("/img.jpg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", imgMime)
		w.Write(append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{7}, 32)...))
	})
	srv := httptest.NewServer(mux)
	base = srv.URL
	t.Cleanup(srv.Close)
	return srv, &photos.Client{HTTP: srv.Client(), BaseURL: srv.URL + "/api"}
}

func TestFindPhotosReturnsOnlyAllowed(t *testing.T) {
	e := setup(t)
	id := e.create(t)
	_, cl := photoServer(t, "image/jpeg")
	e.svc.d.Photos = cl
	got, err := e.svc.FindPhotos(id, "bronze drum")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "File:A.jpg" {
		t.Fatalf("want only the CC BY-SA image, got %+v", got)
	}
}

func TestPickPhotoAddsSourceAndVersion(t *testing.T) {
	e := setup(t)
	id := e.create(t)
	_, cl := photoServer(t, "image/jpeg")
	e.svc.d.Photos = cl
	got, _ := e.svc.FindPhotos(id, "bronze drum")
	if err := e.svc.PickPhoto(id, "a", got[0]); err != nil {
		t.Fatal(err)
	}
	d, _ := e.svc.Detail(id)
	if d.CurrentVersion != 2 {
		t.Fatalf("want new version 2, got %d", d.CurrentVersion)
	}
	credit := photos.Credit(got[0])
	found := false
	for _, s := range d.Sources {
		if s.Text == credit {
			found = true
		}
	}
	if !found {
		t.Fatalf("credit %q not in sources %+v", credit, d.Sources)
	}
	dir, _ := e.svc.VideoDir(id)
	idx, _ := photos.Index(dir)
	if len(idx["a"]) != 1 {
		t.Fatalf("photos/index.json: %+v", idx)
	}
	if !strings.HasPrefix(idx["a"][0].File, "photos/") {
		t.Fatalf("file not under photos/: %+v", idx["a"][0])
	}
}

func TestPickPhotoTwiceNoDuplicateSource(t *testing.T) {
	e := setup(t)
	id := e.create(t)
	_, cl := photoServer(t, "image/jpeg")
	e.svc.d.Photos = cl
	got, _ := e.svc.FindPhotos(id, "bronze drum")
	if err := e.svc.PickPhoto(id, "a", got[0]); err != nil {
		t.Fatal(err)
	}
	before, _ := e.svc.Detail(id)
	if err := e.svc.PickPhoto(id, "a", got[0]); err != nil {
		t.Fatal(err)
	}
	after, _ := e.svc.Detail(id)
	if len(after.Sources) != len(before.Sources) {
		t.Fatalf("duplicate source: %d → %d", len(before.Sources), len(after.Sources))
	}
	if after.CurrentVersion != before.CurrentVersion {
		t.Fatalf("no-op pick created version %d", after.CurrentVersion)
	}
}

func TestPickPhotoRejectsNonImage(t *testing.T) {
	e := setup(t)
	id := e.create(t)
	_, cl := photoServer(t, "image/svg+xml")
	e.svc.d.Photos = cl
	got, _ := e.svc.FindPhotos(id, "bronze drum")
	err := e.svc.PickPhoto(id, "a", got[0])
	if err == nil || !strings.Contains(err.Error(), "Ảnh không hợp lệ") {
		t.Fatalf("want Vietnamese non-image error, got %v", err)
	}
	if d, _ := e.svc.Detail(id); d.CurrentVersion != 1 {
		t.Fatalf("version created despite error: %d", d.CurrentVersion)
	}
}

func TestDetailShowsPhotoQueries(t *testing.T) {
	e := setup(t)
	id := e.create(t)
	v, _ := e.st.GetVideo(id)
	ver, _ := e.st.GetVersion(id, v.CurrentVersion)
	if err := os.WriteFile(ver.NotesPath, []byte(`{"slides":{"a":{"note":"cần ảnh thật","photo_queries":["Ngoc Lu bronze drum"]}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	d, _ := e.svc.Detail(id)
	if q := d.Slides[0].PhotoQueries; len(q) != 1 || q[0] != "Ngoc Lu bronze drum" {
		t.Fatalf("photoQueries = %q", q)
	}
}
