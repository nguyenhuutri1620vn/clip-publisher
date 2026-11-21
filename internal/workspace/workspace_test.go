package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Nỏ thần Cổ Loa":      "no-than-co-loa",
		"Đền Thượng — 2004!":  "den-thuong-2004",
		"   ":                 "video",
		"Hai Bà Trưng: 3 năm": "hai-ba-trung-3-nam",
		"ỨNG Ở ỄNH ỷ":         "ung-o-enh-y",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
	long := Slugify(strings.Repeat("Cổ Loa ", 12))
	if len(long) > 48 || strings.HasSuffix(long, "-") {
		t.Errorf("long slug %q: len %d", long, len(long))
	}
}

func TestNewVideoDirUnique(t *testing.T) {
	w, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s1, d1, err := w.NewVideoDir("Cổ Loa")
	if err != nil {
		t.Fatal(err)
	}
	s2, d2, err := w.NewVideoDir("Cổ Loa")
	if err != nil {
		t.Fatal(err)
	}
	if s1 != "co-loa" || s2 != "co-loa-2" {
		t.Fatalf("slugs %q %q", s1, s2)
	}
	for _, d := range []string{d1, d2} {
		if st, err := os.Stat(d); err != nil || !st.IsDir() {
			t.Fatalf("dir %s missing", d)
		}
	}
	if w.VideoDir(s1) != d1 {
		t.Fatalf("VideoDir mismatch")
	}
}

func TestNewCreatesLayout(t *testing.T) {
	root := t.TempDir()
	if _, err := New(root); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"videos", "logs", "thumbs"} {
		if _, err := os.Stat(filepath.Join(root, d)); err != nil {
			t.Errorf("%s not created", d)
		}
	}
}

func TestPaths(t *testing.T) {
	d := "/w/videos/x"
	checks := map[string]string{
		ScriptPath(d, 2):     "/w/videos/x/script.v2.json",
		NotesPath(d, 2):      "/w/videos/x/notes.v2.json",
		RenderMP4Path(d, 3):  "/w/videos/x/renders/v3.mp4",
		RenderLogPath(d, 3):  "/w/videos/x/renders/render.v3.log",
		PreviewMP4Path(d, 1): "/w/videos/x/renders/preview.v1/preview.mp4",
		VoiceDir(d):          "/w/videos/x/voice",
		InputPath(d):         "/w/videos/x/input.md",
	}
	for got, want := range checks {
		if got != want {
			t.Errorf("got %s want %s", got, want)
		}
	}
}

func TestWriteFileAtomicNoPartial(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a", "b", "c.json")
	if err := WriteFileAtomic(p, []byte(`{"x":1}`)); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil || string(b) != `{"x":1}` {
		t.Fatalf("content %q err %v", b, err)
	}
	entries, _ := os.ReadDir(filepath.Dir(p))
	if len(entries) != 1 {
		t.Fatalf("leftover files: %v", entries)
	}
}

func TestChatRoundTrip(t *testing.T) {
	dir := t.TempDir()
	got, err := ReadChat(dir)
	if err != nil || len(got) != 0 {
		t.Fatalf("empty read: %v %v", got, err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	msgs := []ChatMessage{{Role: "user", Text: "cảnh 2 ngắn lại", At: now}, {Role: "assistant", Text: "Đã rút gọn", At: now}}
	for _, m := range msgs {
		if err := AppendChat(dir, m); err != nil {
			t.Fatal(err)
		}
	}
	got, err = ReadChat(dir)
	if err != nil || len(got) != 2 || got[0].Text != msgs[0].Text || got[1].Role != "assistant" || !got[0].At.Equal(now) {
		t.Fatalf("got %+v err %v", got, err)
	}
}
