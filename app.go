package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"clip-publisher/internal/config"
	"clip-publisher/internal/media"
	"clip-publisher/internal/notify"
	"clip-publisher/internal/photos"
	"clip-publisher/internal/pipeline"
	"clip-publisher/internal/runner"
	"clip-publisher/internal/service"
	"clip-publisher/internal/store"
	"clip-publisher/internal/workspace"
)

// App is bound to the frontend; every method delegates to the service.
type App struct {
	ctx      context.Context
	mu       sync.RWMutex
	settings config.Settings
	defaults config.Settings
	st       *store.Store
	svc      *service.Service
	startErr error
}

// NewApp creates the app.
func NewApp() *App { return &App{} }

func (a *App) getSettings() config.Settings {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.settings
}

// mediaMiddleware serves /media/* (local videos, PNGs, thumbnails) and passes everything else on.
func (a *App) mediaMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/media/") {
			next.ServeHTTP(w, r)
			return
		}
		if a.svc == nil {
			http.NotFound(w, r)
			return
		}
		media.Handler(a.svc).ServeHTTP(w, r)
	})
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	home, _ := os.UserHomeDir()
	a.defaults = config.Defaults(home, exec.LookPath)
	// The database lives at the default workspace so settings can be read before anything else.
	ws, err := workspace.New(a.defaults.Workspace)
	if err != nil {
		a.startErr = err
		return
	}
	st, err := store.Open(filepath.Join(ws.Root, "app.db"))
	if err != nil {
		a.startErr = err
		return
	}
	a.st = st
	s, err := config.Load(st, a.defaults)
	if err != nil {
		a.startErr = err
		return
	}
	s.Workspace = a.defaults.Workspace
	a.settings = s
	a.svc = service.New(service.Deps{
		Store: st, WS: ws, Settings: a.getSettings, Runner: runner.ExecRunner{},
		Emit:   func(name string, data any) { runtime.EventsEmit(ctx, name, data) },
		Notify: func(title, body string) { _ = notify.Send(title, body) },
		Trash:  notify.MoveToTrash,
	})
	if err := a.svc.Recover(); err != nil {
		a.startErr = err
		return
	}
	go func() {
		if _, err := a.svc.Import(); err != nil {
			runtime.LogErrorf(ctx, "import: %v", err)
		}
	}()
}

func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	if a.svc == nil || !a.svc.Busy() {
		return false
	}
	answer, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type: runtime.QuestionDialog, Title: "Đang render",
		Message: "Đang có việc chạy (render hoặc Claude). Thoát sẽ huỷ việc đó. Thoát?",
		Buttons: []string{"Thoát", "Ở lại"}, DefaultButton: "Ở lại", CancelButton: "Ở lại",
	})
	if err != nil || answer != "Thoát" {
		return true
	}
	a.svc.CancelAll()
	done := make(chan struct{})
	go func() { a.svc.WaitIdle(); close(done) }()
	select {
	case <-done:
	case <-time.After(8 * time.Second):
	}
	return false
}

func (a *App) shutdown(context.Context) {
	if a.svc != nil {
		a.svc.CancelAll()
	}
	if a.st != nil {
		a.st.Close()
	}
}

func (a *App) ready() error {
	if a.startErr != nil {
		return a.startErr
	}
	if a.svc == nil {
		return errors.New("App chưa khởi động xong")
	}
	return nil
}

// StartupError returns the error that stopped the app from starting ("" if none).
func (a *App) StartupError() string {
	if a.startErr != nil {
		return a.startErr.Error()
	}
	return ""
}

func (a *App) ListVideos() ([]service.VideoSummary, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.List()
}

func (a *App) GetVideo(id int64) (service.VideoDetail, error) {
	if err := a.ready(); err != nil {
		return service.VideoDetail{}, err
	}
	return a.svc.Detail(id)
}

func (a *App) CreateVideo(input, voice, music string) (int64, error) {
	if err := a.ready(); err != nil {
		return 0, err
	}
	return a.svc.CreateFromText(input, voice, music)
}

func (a *App) Chat(id int64, message string) error {
	return a.call(func() error { return a.svc.Chat(id, message) })
}

func (a *App) EditSlideText(id int64, slideID, title, subtitle, narration string) error {
	return a.call(func() error { return a.svc.EditSlideText(id, slideID, title, subtitle, narration) })
}

func (a *App) EditSlideRaw(id int64, slideID, raw string) error {
	return a.call(func() error { return a.svc.EditSlideRaw(id, slideID, raw) })
}

func (a *App) RestoreVersion(id int64, n int) error {
	return a.call(func() error { return a.svc.RestoreVersion(id, n) })
}

// FindPhotos searches Wikimedia Commons for a slide's photo (licences checked in internal/photos).
func (a *App) FindPhotos(id int64, query string) ([]photos.Candidate, error) {
	var out []photos.Candidate
	err := a.call(func() error {
		var e error
		out, e = a.svc.FindPhotos(id, query)
		return e
	})
	return out, err
}

// PickPhoto downloads the chosen photo for a slide and cites it in the script.
func (a *App) PickPhoto(id int64, slideID string, cand photos.Candidate) error {
	return a.call(func() error { return a.svc.PickPhoto(id, slideID, cand) })
}

func (a *App) Preview(id int64) error { return a.call(func() error { return a.svc.Preview(id) }) }
func (a *App) Render(id int64) error  { return a.call(func() error { return a.svc.Render(id) }) }
func (a *App) Retry(id int64) error   { return a.call(func() error { return a.svc.Retry(id) }) }
func (a *App) Cancel(id int64) error  { return a.call(func() error { return a.svc.Cancel(id) }) }
func (a *App) Hide(id int64) error    { return a.call(func() error { return a.svc.Hide(id) }) }
func (a *App) Delete(id int64) error  { return a.call(func() error { return a.svc.Delete(id) }) }

func (a *App) ImportExisting() (int, error) {
	if err := a.ready(); err != nil {
		return 0, err
	}
	return a.svc.Import()
}

func (a *App) OpenFolder(id int64) error {
	return a.call(func() error {
		dir, err := a.svc.VideoDir(id)
		if err != nil {
			return err
		}
		return exec.Command("/usr/bin/open", dir).Run()
	})
}

func (a *App) GetSettings() config.Settings { return a.getSettings() }

func (a *App) SaveSettings(s config.Settings) error {
	if err := a.ready(); err != nil {
		return err
	}
	s.Workspace = a.defaults.Workspace // fixed in the MVP (the database lives there)
	if err := config.Save(a.st, s); err != nil {
		return err
	}
	a.mu.Lock()
	a.settings = s
	a.mu.Unlock()
	return nil
}

func (a *App) ListVoices() ([]string, error) { return config.ListVoices(a.getSettings()) }
func (a *App) ListMusic() ([]string, error)  { return config.ListMusic(a.getSettings()) }

func (a *App) RunDoctor() []pipeline.Check {
	return pipeline.Doctor(a.ctx, a.getSettings(), runner.ExecRunner{})
}

func (a *App) call(fn func() error) error {
	if err := a.ready(); err != nil {
		return err
	}
	return fn()
}
