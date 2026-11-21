package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func openTemp(t *testing.T) (*Store, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "app.db")
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, p
}

func newVideo(t *testing.T, s *Store, slug string) Video {
	t.Helper()
	v := Video{Slug: slug, Title: slug, Origin: "app", Status: StatusDraft, VoiceProfile: "narrator_mid"}
	if err := s.CreateVideo(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestVideoCRUD(t *testing.T) {
	s, _ := openTemp(t)
	a := newVideo(t, s, "a")
	time.Sleep(5 * time.Millisecond)
	b := newVideo(t, s, "b")
	if a.ID == 0 || b.ID == 0 {
		t.Fatal("ids not set")
	}
	got, err := s.GetVideo(a.ID)
	if err != nil || got.Slug != "a" || got.Status != StatusDraft || got.VoiceProfile != "narrator_mid" {
		t.Fatalf("get: %+v %v", got, err)
	}
	time.Sleep(5 * time.Millisecond)
	if err := s.SetStatus(a.ID, StatusFailed, "boom"); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListVideos(false)
	if err != nil || len(list) != 2 || list[0].ID != a.ID || list[0].StatusDetail != "boom" {
		t.Fatalf("list order: %+v %v", list, err)
	}
	b.Hidden = true
	if err := s.UpdateVideo(b); err != nil {
		t.Fatal(err)
	}
	list, _ = s.ListVideos(false)
	if len(list) != 1 {
		t.Fatalf("hidden not excluded: %d", len(list))
	}
	list, _ = s.ListVideos(true)
	if len(list) != 2 {
		t.Fatalf("includeHidden: %d", len(list))
	}
	if _, err := s.GetVideo(999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestFindVideoBySource(t *testing.T) {
	s, _ := openTemp(t)
	v := Video{Slug: "imp", Title: "imp", Origin: "import", SourcePath: "/o/x.mp4", Status: StatusReady}
	if err := s.CreateVideo(&v); err != nil {
		t.Fatal(err)
	}
	got, err := s.FindVideoBySource("/o/x.mp4")
	if err != nil || got.ID != v.ID {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := s.FindVideoBySource("/nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestVersionsOrdered(t *testing.T) {
	s, _ := openTemp(t)
	v := newVideo(t, s, "v")
	for _, n := range []int{2, 1} {
		if err := s.AddVersion(Version{VideoID: v.ID, N: n, ScriptPath: "s", Author: "claude"}); err != nil {
			t.Fatal(err)
		}
	}
	list, err := s.ListVersions(v.ID)
	if err != nil || len(list) != 2 || list[0].N != 1 || list[1].N != 2 {
		t.Fatalf("%+v %v", list, err)
	}
	if err := s.AddVersion(Version{VideoID: v.ID, N: 1, ScriptPath: "s", Author: "manual"}); err == nil {
		t.Fatal("duplicate version accepted")
	}
	got, err := s.GetVersion(v.ID, 2)
	if err != nil || got.Author != "claude" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestRenders(t *testing.T) {
	s, _ := openTemp(t)
	v := newVideo(t, s, "r")
	r1 := Render{VideoID: v.ID, Version: 1, Kind: "render", MP4Path: "a.mp4", State: "RUNNING"}
	if err := s.AddRender(&r1); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	r2 := Render{VideoID: v.ID, Version: 1, Kind: "preview", MP4Path: "p.mp4", State: "RUNNING"}
	if err := s.AddRender(&r2); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishRender(r1.ID, "DONE", "", 42.5); err != nil {
		t.Fatal(err)
	}
	got, err := s.LatestRender(v.ID, "render")
	if err != nil || got.ID != r1.ID || got.State != "DONE" || got.Seconds != 42.5 || got.FinishedAt == nil {
		t.Fatalf("%+v %v", got, err)
	}
	list, _ := s.ListRenders(v.ID)
	if len(list) != 2 || list[0].ID != r2.ID {
		t.Fatalf("order: %+v", list)
	}
	r, err := s.GetRender(r1.ID)
	if err != nil || r.MP4Path != "a.mp4" {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestMarkInterrupted(t *testing.T) {
	s, _ := openTemp(t)
	v := newVideo(t, s, "m")
	states := []string{"RUNNING", "QUEUED", "DONE"}
	for _, st := range states {
		j := Job{VideoID: v.ID, Kind: "render", State: st}
		if err := s.AddJob(&j); err != nil {
			t.Fatal(err)
		}
	}
	r := Render{VideoID: v.ID, Version: 1, Kind: "render", State: "RUNNING"}
	s.AddRender(&r)
	jobs, err := s.MarkInterrupted()
	if err != nil || len(jobs) != 2 {
		t.Fatalf("%+v %v", jobs, err)
	}
	for _, j := range jobs {
		if j.State != "FAILED" || j.Error != "interrupted" {
			t.Fatalf("job %+v", j)
		}
	}
	last, _ := s.LastJob(v.ID)
	if last.State != "DONE" {
		t.Fatalf("DONE job touched or wrong last: %+v", last)
	}
	rr, _ := s.GetRender(r.ID)
	if rr.State != "FAILED" {
		t.Fatalf("render state %s", rr.State)
	}
}

func TestDeleteCascades(t *testing.T) {
	s, _ := openTemp(t)
	v := newVideo(t, s, "d")
	s.AddVersion(Version{VideoID: v.ID, N: 1, ScriptPath: "s", Author: "claude"})
	r := Render{VideoID: v.ID, Version: 1, Kind: "render", State: "DONE"}
	s.AddRender(&r)
	j := Job{VideoID: v.ID, Kind: "render", State: "DONE"}
	s.AddJob(&j)
	if err := s.DeleteVideo(v.ID); err != nil {
		t.Fatal(err)
	}
	vs, _ := s.ListVersions(v.ID)
	rs, _ := s.ListRenders(v.ID)
	if len(vs) != 0 || len(rs) != 0 {
		t.Fatal("not cascaded")
	}
	if _, err := s.LastJob(v.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("job not cascaded")
	}
}

func TestSettingsAndReopen(t *testing.T) {
	s, p := openTemp(t)
	if _, ok, err := s.GetSetting("x"); ok || err != nil {
		t.Fatal("unexpected setting")
	}
	s.SetSetting("x", "1")
	s.SetSetting("x", "2")
	newVideo(t, s, "keep")
	s.Close()
	s2, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if v, ok, _ := s2.GetSetting("x"); !ok || v != "2" {
		t.Fatalf("setting %q", v)
	}
	if list, _ := s2.ListVideos(true); len(list) != 1 {
		t.Fatal("data lost on reopen")
	}
}
