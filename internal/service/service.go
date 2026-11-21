// Package service orchestrates videos: script generation, edits, versions, renders and imports.
// It is the only package the Wails bindings talk to.
package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"clip-publisher/internal/config"
	"clip-publisher/internal/jobs"
	"clip-publisher/internal/pipeline"
	"clip-publisher/internal/runner"
	"clip-publisher/internal/scriptmodel"
	"clip-publisher/internal/store"
	"clip-publisher/internal/workspace"
)

// ErrBusy is returned when a video is being generated or rendered.
var ErrBusy = errors.New("Video đang được xử lý, hãy đợi hoặc huỷ")

// Deps are the service's collaborators.
type Deps struct {
	Store         *store.Store
	WS            *workspace.Workspace
	Settings      func() config.Settings
	Runner        runner.Runner
	ClaudeTimeout time.Duration // default 5 min
	Emit          func(event string, data any)
	Notify        func(title, body string)
	Trash         func(path string) error
}

// Service is the orchestrator.
type Service struct {
	d        Deps
	renderQ  *jobs.Queue
	claudeQ  *jobs.Queue
	mu       sync.Mutex
	progress map[int64]*ProgressDTO
	logTail  map[int64][]string
	warnings map[string][]string // validator warnings by script path
}

const (
	claudeTimeoutDefault = 5 * time.Minute
	maxRepairs           = 3
	liveLogLines         = 200
)

// New creates the service.
func New(d Deps) *Service {
	if d.ClaudeTimeout == 0 {
		d.ClaudeTimeout = claudeTimeoutDefault
	}
	if d.Emit == nil {
		d.Emit = func(string, any) {}
	}
	if d.Notify == nil {
		d.Notify = func(string, string) {}
	}
	return &Service{
		d: d, renderQ: jobs.NewQueue(), claudeQ: jobs.NewQueue(),
		progress: map[int64]*ProgressDTO{}, logTail: map[int64][]string{}, warnings: map[string][]string{},
	}
}

// WaitIdle blocks until no job is queued or running (tests, shutdown).
func (s *Service) WaitIdle() {
	s.claudeQ.Wait()
	s.renderQ.Wait()
}

// Busy reports whether any render or Claude job is running.
func (s *Service) Busy() bool { return s.renderQ.Busy() || s.claudeQ.Busy() }

// CancelAll stops all jobs (app quit).
func (s *Service) CancelAll() {
	s.claudeQ.CancelAll()
	s.renderQ.CancelAll()
}

func (s *Service) changed(id int64) {
	s.d.Emit("video:changed", map[string]int64{"id": id})
}

func renderKey(id int64) string { return fmt.Sprintf("render:%d", id) }
func claudeKey(id int64) string { return fmt.Sprintf("claude:%d", id) }

func isBusy(v store.Video) bool {
	return v.Status == store.StatusRendering || v.Status == store.StatusGeneratingScript
}

func (s *Service) videoDir(v store.Video) string { return s.d.WS.VideoDir(v.Slug) }

// current loads the current script version of a video.
func (s *Service) current(v store.Video) ([]byte, *scriptmodel.Script, scriptmodel.Notes, error) {
	if v.CurrentVersion == 0 {
		return nil, nil, scriptmodel.Notes{}, errors.New("video chưa có script")
	}
	ver, err := s.d.Store.GetVersion(v.ID, v.CurrentVersion)
	if err != nil {
		return nil, nil, scriptmodel.Notes{}, err
	}
	raw, err := os.ReadFile(ver.ScriptPath)
	if err != nil {
		return nil, nil, scriptmodel.Notes{}, err
	}
	sc, err := scriptmodel.Parse(raw)
	if err != nil {
		return raw, nil, scriptmodel.Notes{}, err
	}
	nb, _ := os.ReadFile(ver.NotesPath)
	notes, _ := scriptmodel.ParseNotes(nb)
	return raw, sc, notes, nil
}

// validate runs `history_shorts validate` on script bytes (written next to the video's scripts so
// relative sfx paths resolve) and caches the warnings under cacheKey.
func (s *Service) validate(ctx context.Context, dir string, script []byte) (errs, warns []string, err error) {
	tmp := filepath.Join(dir, ".validate.json")
	if err := workspace.WriteFileAtomic(tmp, script); err != nil {
		return nil, nil, err
	}
	defer os.Remove(tmp)
	res, err := s.d.Runner.Run(ctx, pipeline.ValidateCmd(s.d.Settings(), tmp), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("không chạy được history_shorts validate: %w", err)
	}
	errs, rawWarns := pipeline.ParseValidate(res.Output)
	for _, w := range rawWarns {
		// The app validates without --audio on purpose; that warning is always true and only noise.
		if strings.Contains(w, "no --audio") {
			continue
		}
		warns = append(warns, strings.ReplaceAll(w, ".validate.json", "script.json"))
	}
	for i, e := range errs {
		errs[i] = strings.ReplaceAll(e, ".validate.json", "script.json")
	}
	if res.ExitCode != 0 && len(errs) == 0 {
		errs = []string{"validate thoát với mã " + fmt.Sprint(res.ExitCode) + ": " + strings.Join(pipeline.Tail(res.Output, 3), " / ")}
	}
	return errs, warns, nil
}

// saveVersion writes script+notes as version N+1 and makes it current.
func (s *Service) saveVersion(v *store.Video, script []byte, notes scriptmodel.Notes, author, summary string, warns []string) error {
	vs, err := s.d.Store.ListVersions(v.ID)
	if err != nil {
		return err
	}
	n := 1
	if len(vs) > 0 {
		n = vs[len(vs)-1].N + 1
	}
	dir := s.videoDir(*v)
	sp, np := workspace.ScriptPath(dir, n), workspace.NotesPath(dir, n)
	if err := workspace.WriteFileAtomic(sp, script); err != nil {
		return err
	}
	nb, _ := jsonIndent(notes)
	if err := workspace.WriteFileAtomic(np, nb); err != nil {
		return err
	}
	if err := s.d.Store.AddVersion(store.Version{VideoID: v.ID, N: n, ScriptPath: sp, NotesPath: np, Author: author, Summary: summary}); err != nil {
		return err
	}
	s.mu.Lock()
	s.warnings[sp] = warns
	s.mu.Unlock()
	v.CurrentVersion = n
	if sc, err := scriptmodel.Parse(script); err == nil && sc.Title != "" {
		v.Title = sc.Title
	}
	return s.d.Store.UpdateVideo(*v)
}
