package pipeline

import (
	"bufio"
	"context"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"clip-publisher/internal/config"
	"clip-publisher/internal/runner"
	"clip-publisher/internal/scriptmodel"
)

var st = config.Settings{
	VideoRepo:   "/r/v",
	VoiceRepo:   "/r/s",
	VideoPython: "/r/v/.venv/bin/python",
	VoiceBin:    "/r/s/.venv/bin/make-voice-free",
	ClaudeBin:   "/n/bin/claude",
}

func TestValidateAndPreviewCmd(t *testing.T) {
	c := ValidateCmd(st, "/w/s.json")
	if c.Name != st.VideoPython || c.Dir != "/r/v" ||
		!reflect.DeepEqual(c.Args, []string{"-m", "history_shorts", "validate", "--script", "/w/s.json"}) {
		t.Fatalf("%+v", c)
	}
	p := PreviewCmd(st, "/w/s.json", "/w/p/preview.mp4")
	if !reflect.DeepEqual(p.Args, []string{"-m", "history_shorts", "render", "--script", "/w/s.json", "--out", "/w/p/preview.mp4", "--preview"}) {
		t.Fatalf("%+v", p.Args)
	}
	if len(p.Env) == 0 {
		t.Fatal("env not set")
	}
}

func TestRenderCmd(t *testing.T) {
	c := RenderCmd(st, "/w/s.json", "/w/voice", "/w/r/v1.mp4", "/r/s/voices/narrator_mid.json", "a.mp3")
	want := []string{"tools/make_video.py", "--script", "/w/s.json", "--work", "/w/voice", "--out", "/w/r/v1.mp4",
		"--music", "/r/v/music/a.mp3",
		"--voice-cmd", "/r/s/.venv/bin/make-voice-free voice {text} {wav} --voice /r/s/voices/narrator_mid.json --allow-noncommercial"}
	if c.Name != st.VideoPython || c.Dir != "/r/v" || !reflect.DeepEqual(c.Args, want) {
		t.Fatalf("%+v", c.Args)
	}
	c = RenderCmd(st, "/w/s.json", "/w/voice", "/w/r/v1.mp4", "/p.json", "")
	for _, a := range c.Args {
		if a == "--music" {
			t.Fatal("music flag without music")
		}
	}
}

func shlexSplit(t *testing.T, s string) []string {
	t.Helper()
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 missing")
	}
	out, err := exec.Command(py, "-c", "import shlex,sys\nfor a in shlex.split(sys.argv[1]): print(a)", s).Output()
	if err != nil {
		t.Fatal(err)
	}
	var parts []string
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		parts = append(parts, sc.Text())
	}
	return parts
}

func TestVoiceCmdTemplateQuotes(t *testing.T) {
	bin := "/Users/x/My Code/mvf/.venv/bin/make-voice-free"
	got := VoiceCmdTemplate(bin, "/a/voices/narrator_mid.json")
	if got != "'/Users/x/My Code/mvf/.venv/bin/make-voice-free' voice {text} {wav} --voice /a/voices/narrator_mid.json --allow-noncommercial" {
		t.Fatalf("%s", got)
	}
	weird := "/Users/x/Tri's videos/p.json"
	parts := shlexSplit(t, VoiceCmdTemplate(bin, weird))
	want := []string{bin, "voice", "{text}", "{wav}", "--voice", weird, "--allow-noncommercial"}
	if !reflect.DeepEqual(parts, want) {
		t.Fatalf("%q", parts)
	}
}

func TestParseValidate(t *testing.T) {
	out := "WARN script.json › slides[1].subtitle: text auto-shrunk to 42 px\n" +
		"script.json › slides[2].scene.layers[4].x: must be 0–1080\n" +
		"ERROR cannot read theme\n" +
		"1 errors, 1 warnings\n"
	errs, warns := ParseValidate(out)
	if len(errs) != 2 || len(warns) != 1 || !strings.Contains(errs[0], "layers[4].x") {
		t.Fatalf("errs %v warns %v", errs, warns)
	}
	if !strings.HasPrefix(warns[0], "script.json › slides[1]") {
		t.Fatalf("warn prefix kept: %q", warns[0])
	}
}

func TestProgressFromLog(t *testing.T) {
	f, err := os.Open("testdata/make_video.log")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var p Progress
	var steps []Step
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		if p.Feed(sc.Text()) && (len(steps) == 0 || steps[len(steps)-1] != p.Step) {
			steps = append(steps, p.Step)
		}
	}
	want := []Step{StepValidate, StepExport, StepVoice, StepAudio, StepRender}
	if !reflect.DeepEqual(steps, want) {
		t.Fatalf("steps %v", steps)
	}
	if p.VoiceDone != 3 || p.VoiceID != "that-thu" {
		t.Fatalf("voice %d %q", p.VoiceDone, p.VoiceID)
	}
	if (&Progress{}).Feed("Paired 15 voices") {
		t.Fatal("non-command line changed progress")
	}
}

func TestPendingVoices(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("Lời A\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "a.wav"), nil, 0o644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("Lời cũ\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.wav"), nil, 0o644)
	os.WriteFile(filepath.Join(dir, "d.txt"), []byte("Lời D\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "d.flac"), nil, 0o644)
	s := &scriptmodel.Script{Slides: []scriptmodel.Slide{
		{ID: "a", Narration: "Lời A"},
		{ID: "b", Narration: "Lời mới"},
		{ID: "c", Narration: "Lời C"},
		{ID: "d", Narration: "Lời D"},
		{ID: "e"},
	}}
	if n := PendingVoices(dir, s); n != 2 {
		t.Fatalf("pending %d", n)
	}
}

func TestReportAndStarts(t *testing.T) {
	r, err := ReadReport("testdata/report.json")
	if err != nil {
		t.Fatal(err)
	}
	starts := SlideStarts(r)
	for i, w := range []float64{0, 4.867, 9.8} {
		if math.Abs(starts[i]-w) > 1e-9 {
			t.Fatalf("starts %v", starts[:3])
		}
	}
	if ReportPath("/a/v1.mp4") != "/a/v1.report.json" {
		t.Fatal("ReportPath")
	}
	s := &scriptmodel.Script{Slides: []scriptmodel.Slide{{ID: "one"}, {ID: "two"}}}
	n := scriptmodel.Notes{Slides: map[string]scriptmodel.SlideNote{
		"one": {TimeHint: "0:00 – 0:08"},
		"two": {TimeHint: "0:08 – 0:13"},
	}}
	w := TimingWarnings(s, n, r)
	if len(w) != 1 || w[0].SlideID != "one" || !strings.Contains(w[0].Message, "4.9s") {
		t.Fatalf("%+v", w)
	}
}

func TestTail(t *testing.T) {
	if got := Tail("a\nb\nc\n", 2); !reflect.DeepEqual(got, []string{"b", "c"}) {
		t.Fatalf("%v", got)
	}
	if got := Tail("", 3); len(got) != 0 {
		t.Fatalf("%v", got)
	}
}

type failClaude struct{}

func (failClaude) Run(_ context.Context, c runner.Cmd, _ func(string)) (runner.Result, error) {
	if c.Name == "/n/bin/claude" {
		return runner.Result{}, errors.New("no such file")
	}
	return runner.Result{Output: "ok 1.0\n"}, nil
}

func TestDoctorReportsFailure(t *testing.T) {
	checks := Doctor(context.Background(), st, failClaude{})
	if len(checks) != 4 {
		t.Fatalf("%d checks", len(checks))
	}
	for _, c := range checks {
		isClaude := strings.Contains(strings.ToLower(c.Name), "claude")
		if isClaude && (c.OK || !strings.HasPrefix(c.Detail, "Không chạy được")) {
			t.Fatalf("claude check %+v", c)
		}
		if !isClaude && (!c.OK || c.Detail != "ok 1.0") {
			t.Fatalf("check %+v", c)
		}
	}
}
