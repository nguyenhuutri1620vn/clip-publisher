package library

import (
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

// publish lays out final/<name>/ the way make-video-free/tools/publish_final.py does.
func publish(t *testing.T, repo, name, title string, dur float64) string {
	t.Helper()
	d := filepath.Join(repo, "final", name)
	write(t, filepath.Join(d, name+".mp4"), "v")
	write(t, filepath.Join(d, name+".report.json"), fmt.Sprintf(`{"script_sha256":"x","total_duration":%v}`, dur))
	write(t, filepath.Join(d, "script.json"), `{"title":"`+title+`","slides":[]}`)
	write(t, filepath.Join(d, "final.json"), `{"name":"`+name+`","title":"`+title+`","video":"`+name+`.mp4"}`)
	return d
}

func TestScanReadsFinalOnly(t *testing.T) {
	repo := t.TempDir()
	co := publish(t, repo, "co_loa", "Nỏ thần Cổ Loa", 60)
	old := time.Now().Add(-time.Hour)
	td := publish(t, repo, "trong_dong", "Trống đồng", 5)
	os.Chtimes(filepath.Join(td, "trong_dong.mp4"), old, old)
	// Drafts are ignored: main output/, output subfolders and worktree outputs.
	write(t, filepath.Join(repo, "output", "draft.mp4"), "v")
	write(t, filepath.Join(repo, "output", "draft.report.json"), `{"total_duration":1}`)
	write(t, filepath.Join(repo, ".claude", "worktrees", "x", "output", "w.mp4"), "v")
	write(t, filepath.Join(repo, ".claude", "worktrees", "x", "output", "w.report.json"), `{"total_duration":1}`)
	// A final folder without its video is skipped.
	write(t, filepath.Join(repo, "final", "broken", "final.json"), `{"name":"broken","video":"broken.mp4"}`)

	got, err := Scan(repo)
	if err != nil {
		t.Fatal(err)
	}
	want := []Candidate{
		{MP4: filepath.Join(co, "co_loa.mp4"), Report: filepath.Join(co, "co_loa.report.json"),
			Title: "Nỏ thần Cổ Loa", ScriptPath: filepath.Join(co, "script.json"), Duration: 60},
		{MP4: filepath.Join(td, "trong_dong.mp4"), Report: filepath.Join(td, "trong_dong.report.json"),
			Title: "Trống đồng", ScriptPath: filepath.Join(td, "script.json"), Duration: 5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestScanNoFinalFolder(t *testing.T) {
	got, err := Scan(t.TempDir())
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestFinalDir(t *testing.T) {
	if FinalDir("/r/mvf") != "/r/mvf/final" {
		t.Fatal(FinalDir("/r/mvf"))
	}
}

func TestThumbCmd(t *testing.T) {
	c := ThumbCmd("/ff/ffmpeg", "/a/v.mp4", "/t/1.jpg")
	want := []string{"-y", "-ss", "0.5", "-i", "/a/v.mp4", "-frames:v", "1", "-vf", "scale=270:-1", "/t/1.jpg"}
	if c.Name != "/ff/ffmpeg" || !reflect.DeepEqual(c.Args, want) {
		t.Fatalf("%+v", c)
	}
}
