package service

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"clip-publisher/internal/config"
	"clip-publisher/internal/runner"
	"clip-publisher/internal/store"
	"clip-publisher/internal/workspace"
)

func fakes(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "fakes", name)
}

const validScript = `{"schema_version":"2.2","title":"Nỏ thần","sources":["Lĩnh Nam chích quái [CẦN KIỂM TRA: trang]"],
"slides":[{"id":"a","title":"A","subtitle":"sa","narration":"Lời A"},{"id":"b","title":"B","subtitle":"sb","narration":"Lời B","claim":true,"source":0}]}`

type env struct {
	svc     *Service
	st      *store.Store
	ws      *workspace.Workspace
	dir     string
	mu      sync.Mutex
	events  []string
	notifs  []string
	trashed []string
}

func (e *env) eventCount(name string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, ev := range e.events {
		if ev == name {
			n++
		}
	}
	return n
}

func reply(t *testing.T, dir, name, script, summary string) string {
	t.Helper()
	inner, _ := json.Marshal(map[string]any{
		"script":  json.RawMessage(script),
		"notes":   map[string]any{"slides": map[string]any{"a": map[string]string{"note": "cần ảnh thật", "time_hint": "0:00 – 0:04"}}},
		"summary": summary,
	})
	outer, _ := json.Marshal(map[string]any{"type": "result", "is_error": false, "result": string(inner)})
	p := filepath.Join(dir, name)
	os.WriteFile(p, outer, 0o644)
	return p
}

func setup(t *testing.T) *env {
	t.Helper()
	root := t.TempDir()
	video := filepath.Join(root, "mvf")
	voice := filepath.Join(root, "mvo")
	os.MkdirAll(filepath.Join(video, "docs"), 0o755)
	os.WriteFile(filepath.Join(video, "docs", "raw_requirement.md"),
		[]byte("## 6. Data Contracts\nC\n## 7. CLI\n## Appendix A — x\nA\n## Appendix B — y\n"), 0o644)
	os.MkdirAll(filepath.Join(video, "examples", "hai_ba_trung"), 0o755)
	os.WriteFile(filepath.Join(video, "examples", "hai_ba_trung", "script.json"), []byte(validScript), 0o644)
	os.MkdirAll(filepath.Join(voice, "voices"), 0o755)
	os.WriteFile(filepath.Join(voice, "voices", "narrator_mid.json"), []byte("{}"), 0o644)

	ws, err := workspace.New(filepath.Join(root, "ws"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(ws.Root, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	settings := config.Settings{
		VideoRepo: video, VoiceRepo: voice, VideoPython: fakes("fake_python.sh"), VoiceBin: "/bin/echo",
		ClaudeBin: fakes("fake_claude.sh"), Workspace: ws.Root, DefaultVoice: "narrator_mid",
	}
	e := &env{st: st, ws: ws, dir: root}
	t.Setenv("FAKE_CLAUDE_REPLY", reply(t, root, "ok.json", validScript, "Đã tạo script"))
	e.svc = New(Deps{
		Store: st, WS: ws, Settings: func() config.Settings { return settings }, Runner: runner.ExecRunner{},
		ClaudeTimeout: 10 * time.Second,
		Emit: func(name string, _ any) {
			e.mu.Lock()
			e.events = append(e.events, name)
			e.mu.Unlock()
		},
		Notify: func(title, body string) {
			e.mu.Lock()
			e.notifs = append(e.notifs, title+"|"+body)
			e.mu.Unlock()
		},
		Trash: func(p string) error {
			e.trashed = append(e.trashed, p)
			return os.RemoveAll(p)
		},
	})
	return e
}

func (e *env) create(t *testing.T) int64 {
	t.Helper()
	id, err := e.svc.CreateFromText("CẢNH 1\nChữ trên hình: Nỏ thần Cổ Loa\nLời đọc: Lời A", "", "")
	if err != nil {
		t.Fatal(err)
	}
	e.svc.WaitIdle()
	return id
}

func TestCreateFromTextHappyPath(t *testing.T) {
	e := setup(t)
	id := e.create(t)
	v, _ := e.st.GetVideo(id)
	if v.Status != store.StatusScriptReview || v.Title != "Nỏ thần" || v.CurrentVersion != 1 || v.Slug != "no-than-co-loa" ||
		v.VoiceProfile != "narrator_mid" {
		t.Fatalf("%+v", v)
	}
	vs, _ := e.st.ListVersions(id)
	if len(vs) != 1 || vs[0].Author != "claude" || vs[0].Summary != "Đã tạo script" {
		t.Fatalf("%+v", vs)
	}
	in, _ := os.ReadFile(workspace.InputPath(e.ws.VideoDir(v.Slug)))
	if !strings.HasPrefix(string(in), "CẢNH 1") {
		t.Fatalf("input.md %q", in)
	}
	d, err := e.svc.Detail(id)
	if err != nil || len(d.Slides) != 2 || d.Slides[0].Note != "cần ảnh thật" || d.Slides[0].TimeHint != "0:00 – 0:04" ||
		!d.Sources[0].NeedsCheck || !d.Slides[1].Claim {
		t.Fatalf("%+v %v", d, err)
	}
	if e.eventCount("video:changed") == 0 {
		t.Fatal("no events")
	}
}

func TestCreateFromTextInvalidAfterRepairs(t *testing.T) {
	e := setup(t)
	t.Setenv("FAKE_VALIDATE_OUT", "script.json › slides[0].id: bad")
	t.Setenv("FAKE_VALIDATE_EXIT", "3")
	id := e.create(t)
	v, _ := e.st.GetVideo(id)
	if v.Status != store.StatusFailed || !strings.HasPrefix(v.StatusDetail, "Tạo script:") {
		t.Fatalf("%+v", v)
	}
	if v.CurrentVersion != 1 {
		t.Fatalf("last invalid script not kept for manual fix: %d", v.CurrentVersion)
	}
	j, _ := e.st.LastJob(id)
	if j.Kind != "script" || j.State != "FAILED" {
		t.Fatalf("%+v", j)
	}
}

func TestRenderHappyPath(t *testing.T) {
	e := setup(t)
	id := e.create(t)
	if err := e.svc.Render(id); err != nil {
		t.Fatal(err)
	}
	e.svc.WaitIdle()
	v, _ := e.st.GetVideo(id)
	if v.Status != store.StatusReady {
		t.Fatalf("%+v", v)
	}
	r, _ := e.st.LatestRender(id, "render")
	if r.State != "DONE" || r.Version != 1 {
		t.Fatalf("%+v", r)
	}
	logb, _ := os.ReadFile(r.LogPath)
	if !strings.Contains(string(logb), "$ python -m history_shorts render") {
		t.Fatalf("log %q", logb)
	}
	if len(e.notifs) != 1 || !strings.Contains(e.notifs[0], "Nỏ thần") {
		t.Fatalf("notifs %v", e.notifs)
	}
	if e.eventCount("job:progress") < 5 {
		t.Fatal("too few progress events")
	}
	d, _ := e.svc.Detail(id)
	if len(d.Renders) != 1 || d.Renders[0].URL == "" || d.Slides[1].StartSec != 4.0 {
		t.Fatalf("%+v", d.Renders)
	}
}

func TestRenderFailure(t *testing.T) {
	e := setup(t)
	id := e.create(t)
	t.Setenv("FAKE_MV_EXIT", "5")
	e.svc.Render(id)
	e.svc.WaitIdle()
	v, _ := e.st.GetVideo(id)
	if v.Status != store.StatusFailed || !strings.HasPrefix(v.StatusDetail, "Render (render):") ||
		!strings.Contains(v.StatusDetail, "boom") {
		t.Fatalf("%+v", v)
	}
	d, _ := e.svc.Detail(id)
	if len(d.LogTail) == 0 {
		t.Fatal("no log tail")
	}
}

func TestEditBlockedWhileRendering(t *testing.T) {
	e := setup(t)
	id := e.create(t)
	t.Setenv("FAKE_MV_SLEEP", "2")
	e.svc.Render(id)
	time.Sleep(300 * time.Millisecond)
	if err := e.svc.EditSlideText(id, "a", "X", "", "Lời"); !errors.Is(err, ErrBusy) {
		t.Fatalf("edit: %v", err)
	}
	if err := e.svc.Chat(id, "ngắn lại"); !errors.Is(err, ErrBusy) {
		t.Fatalf("chat: %v", err)
	}
	if err := e.svc.Render(id); !errors.Is(err, ErrBusy) {
		t.Fatalf("render twice: %v", err)
	}
	e.svc.WaitIdle()
	vs, _ := e.st.ListVersions(id)
	if len(vs) != 1 {
		t.Fatalf("versions %d", len(vs))
	}
}

func TestCancelRender(t *testing.T) {
	e := setup(t)
	id := e.create(t)
	t.Setenv("FAKE_MV_SLEEP", "5")
	e.svc.Render(id)
	time.Sleep(300 * time.Millisecond)
	if err := e.svc.Cancel(id); err != nil {
		t.Fatal(err)
	}
	e.svc.WaitIdle()
	v, _ := e.st.GetVideo(id)
	r, _ := e.st.LatestRender(id, "render")
	if v.Status != store.StatusScriptReview || r.State != "CANCELED" {
		t.Fatalf("%+v %+v", v, r)
	}
}

func TestCancelQueuedRender(t *testing.T) {
	e := setup(t)
	a := e.create(t)
	b := e.create(t)
	t.Setenv("FAKE_MV_SLEEP", "2")
	e.svc.Render(a)
	e.svc.Render(b)
	d, _ := e.svc.Detail(b)
	if d.Video.QueuePos != 1 {
		t.Fatalf("queue pos %d", d.Video.QueuePos)
	}
	e.svc.Cancel(b)
	e.svc.WaitIdle()
	vb, _ := e.st.GetVideo(b)
	rb, _ := e.st.LatestRender(b, "render")
	if vb.Status != store.StatusScriptReview || rb.State != "CANCELED" {
		t.Fatalf("%+v %+v", vb, rb)
	}
}

func TestRecoverMarksInterrupted(t *testing.T) {
	e := setup(t)
	id := e.create(t)
	e.st.SetStatus(id, store.StatusRendering, "")
	j := store.Job{VideoID: id, Kind: "render", State: "RUNNING"}
	e.st.AddJob(&j)
	if err := e.svc.Recover(); err != nil {
		t.Fatal(err)
	}
	v, _ := e.st.GetVideo(id)
	if v.Status != store.StatusFailed || v.StatusDetail != "Bị gián đoạn (app đã tắt khi đang chạy)" {
		t.Fatalf("%+v", v)
	}
	if err := e.svc.Retry(id); err != nil {
		t.Fatal(err)
	}
	e.svc.WaitIdle()
	v, _ = e.st.GetVideo(id)
	if v.Status != store.StatusReady {
		t.Fatalf("after retry %+v", v)
	}
}

func TestEditCreatesVersionAndDiff(t *testing.T) {
	e := setup(t)
	id := e.create(t)
	if err := e.svc.EditSlideText(id, "a", "A", "sa", "Lời mới"); err != nil {
		t.Fatal(err)
	}
	d, _ := e.svc.Detail(id)
	if d.CurrentVersion != 2 || d.Versions[1].Author != "manual" || d.Slides[0].Note != "cần ảnh thật" {
		t.Fatalf("%+v", d)
	}
	if len(d.Changes) != 1 || d.Changes[0].ID != "a" || d.Changes[0].Fields[0] != "narration" {
		t.Fatalf("%+v", d.Changes)
	}
	if err := e.svc.EditSlideRaw(id, "b", `{"id":"b","title":"B2"}`); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_VALIDATE_OUT", "script.json › slides[1].narration: required")
	if err := e.svc.EditSlideText(id, "a", "A", "", ""); err == nil || !strings.Contains(err.Error(), "narration: required") {
		t.Fatalf("invalid edit: %v", err)
	}
	if err := e.svc.RestoreVersion(id, 1); err != nil {
		t.Fatal(err)
	}
	d, _ = e.svc.Detail(id)
	if d.CurrentVersion != 4 || d.Versions[3].Author != "restore" || d.Slides[0].Narration != "Lời A" {
		t.Fatalf("%+v", d.Versions)
	}
}

func TestChatCreatesVersion(t *testing.T) {
	e := setup(t)
	id := e.create(t)
	edited := strings.Replace(validScript, "Lời B", "Ngắn", 1)
	t.Setenv("FAKE_CLAUDE_REPLY", reply(t, e.dir, "edit.json", edited, "Rút gọn cảnh b"))
	if err := e.svc.Chat(id, "cảnh b ngắn lại"); err != nil {
		t.Fatal(err)
	}
	e.svc.WaitIdle()
	d, _ := e.svc.Detail(id)
	if d.CurrentVersion != 2 || len(d.Chat) != 2 || d.Chat[1].Text != "Rút gọn cảnh b" || d.Video.Status != "SCRIPT_REVIEW" {
		t.Fatalf("%+v", d)
	}
}

func TestPreview(t *testing.T) {
	e := setup(t)
	id := e.create(t)
	if err := e.svc.Preview(id); err != nil {
		t.Fatal(err)
	}
	e.svc.WaitIdle()
	d, _ := e.svc.Detail(id)
	if d.Slides[0].PreviewURL == "" || !strings.Contains(d.Slides[1].PreviewURL, "02.png") {
		t.Fatalf("%+v", d.Slides)
	}
	if d.Video.Status != "SCRIPT_REVIEW" {
		t.Fatal("preview changed status")
	}
}

func TestImportIdempotent(t *testing.T) {
	e := setup(t)
	out := filepath.Join(e.svc.d.Settings().VideoRepo, "output")
	os.MkdirAll(out, 0o755)
	os.WriteFile(filepath.Join(out, "x.mp4"), []byte("v"), 0o644)
	os.WriteFile(filepath.Join(out, "x.report.json"), []byte(`{"script_sha256":"none","total_duration":9}`), 0o644)
	n, err := e.svc.Import()
	if err != nil || n != 1 {
		t.Fatalf("%d %v", n, err)
	}
	n, _ = e.svc.Import()
	if n != 0 {
		t.Fatalf("second import %d", n)
	}
	list, _ := e.svc.List()
	if len(list) != 1 || list[0].Editable || list[0].Status != "READY" || list[0].Duration != 9 {
		t.Fatalf("%+v", list)
	}
	if err := e.svc.Delete(list[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "x.mp4")); err != nil {
		t.Fatal("import delete touched the original file")
	}
	if list, _ := e.svc.List(); len(list) != 0 {
		t.Fatal("import not hidden")
	}
}

func TestDeleteAppVideoTrashesFolder(t *testing.T) {
	e := setup(t)
	id := e.create(t)
	v, _ := e.st.GetVideo(id)
	if err := e.svc.Delete(id); err != nil {
		t.Fatal(err)
	}
	if len(e.trashed) != 1 || e.trashed[0] != e.ws.VideoDir(v.Slug) {
		t.Fatalf("%v", e.trashed)
	}
	if _, err := e.st.GetVideo(id); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("row not deleted")
	}
}

func TestValidatorWarningsCleaned(t *testing.T) {
	e := setup(t)
	t.Setenv("FAKE_VALIDATE_OUT", "WARN script.json › slides: narration present but no --audio: slides use auto timing and no voice\n"+
		"WARN .validate.json › slides[1].subtitle: text auto-shrunk to 42 px")
	id := e.create(t)
	d, _ := e.svc.Detail(id)
	var msgs []string
	for _, w := range d.Warnings {
		msgs = append(msgs, w.Message)
	}
	joined := strings.Join(msgs, "|")
	if strings.Contains(joined, "no --audio") || strings.Contains(joined, ".validate.json") ||
		!strings.Contains(joined, "script.json › slides[1].subtitle") {
		t.Fatalf("%q", joined)
	}
}
