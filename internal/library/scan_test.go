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
