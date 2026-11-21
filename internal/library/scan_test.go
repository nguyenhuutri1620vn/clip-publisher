package library

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func write(t *testing.T, p, content string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScan(t *testing.T) {
	repo := t.TempDir()
	script := `{"title":"Nỏ thần Cổ Loa","slides":[]}`
	write(t, filepath.Join(repo, "examples", "x", "script.json"), script)
	sum := sha256.Sum256([]byte(script))
	write(t, filepath.Join(repo, "output", "a.mp4"), "v")
	write(t, filepath.Join(repo, "output", "a.report.json"),
		fmt.Sprintf(`{"script_sha256":"%s","total_duration":54.8,"slides":[]}`, hex.EncodeToString(sum[:])))
	write(t, filepath.Join(repo, "output", "hai_ba-trung.mp4"), "v")
	write(t, filepath.Join(repo, "output", "hai_ba-trung.report.json"), `{"script_sha256":"ff","total_duration":10}`)
	write(t, filepath.Join(repo, "output", "c.mp4"), "v")
	old := time.Now().Add(-time.Hour)
	os.Chtimes(filepath.Join(repo, "output", "hai_ba-trung.mp4"), old, old)

	got, err := Scan(repo)
	if err != nil {
		t.Fatal(err)
	}
	want := []Candidate{
		{MP4: filepath.Join(repo, "output", "a.mp4"), Report: filepath.Join(repo, "output", "a.report.json"),
			Title: "Nỏ thần Cổ Loa", ScriptPath: filepath.Join(repo, "examples", "x", "script.json"), Duration: 54.8},
		{MP4: filepath.Join(repo, "output", "hai_ba-trung.mp4"), Report: filepath.Join(repo, "output", "hai_ba-trung.report.json"),
			Title: "hai ba trung", Duration: 10},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestScanMissingOutput(t *testing.T) {
	got, err := Scan(t.TempDir())
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestThumbCmd(t *testing.T) {
	c := ThumbCmd("/ff/ffmpeg", "/a/v.mp4", "/t/1.jpg")
	want := []string{"-y", "-ss", "0.5", "-i", "/a/v.mp4", "-frames:v", "1", "-vf", "scale=270:-1", "/t/1.jpg"}
	if c.Name != "/ff/ffmpeg" || !reflect.DeepEqual(c.Args, want) {
		t.Fatalf("%+v", c)
	}
}

func TestScanFindsVideosInSubfolders(t *testing.T) {
	repo := t.TempDir()
	write(t, filepath.Join(repo, "output", "khuc_thua_du", "khuc_thua_du.mp4"), "v")
	write(t, filepath.Join(repo, "output", "khuc_thua_du", "khuc_thua_du.report.json"), `{"script_sha256":"x","total_duration":5}`)
	write(t, filepath.Join(repo, "output", "voice", "a", "a.mp4"), "v") // no report → ignored
	got, err := Scan(repo)
	if err != nil || len(got) != 1 || got[0].MP4 != filepath.Join(repo, "output", "khuc_thua_du", "khuc_thua_du.mp4") {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestScanIncludesWorktrees(t *testing.T) {
	repo := t.TempDir()
	wt := filepath.Join(repo, ".claude", "worktrees", "co-loa-arrow-script-2f38f6")
	script := `{"title":"Nỏ thần Cổ Loa","slides":[]}`
	write(t, filepath.Join(wt, "examples", "co_loa", "script.json"), script)
	sum := sha256.Sum256([]byte(script))
	write(t, filepath.Join(wt, "output", "co_loa.mp4"), "v")
	write(t, filepath.Join(wt, "output", "co_loa.report.json"),
		fmt.Sprintf(`{"script_sha256":"%s","total_duration":60}`, hex.EncodeToString(sum[:])))
	write(t, filepath.Join(wt, "output", "trong_dong", "trong_dong.mp4"), "v")
	write(t, filepath.Join(wt, "output", "trong_dong", "trong_dong.report.json"), `{"script_sha256":"x","total_duration":5}`)
	got, err := Scan(repo)
	if err != nil || len(got) != 2 {
		t.Fatalf("%+v %v", got, err)
	}
	var co Candidate
	for _, c := range got {
		if filepath.Base(c.MP4) == "co_loa.mp4" {
			co = c
		}
	}
	if co.Title != "Nỏ thần Cổ Loa" || co.ScriptPath != filepath.Join(wt, "examples", "co_loa", "script.json") {
		t.Fatalf("co_loa: %+v", co)
	}
}
