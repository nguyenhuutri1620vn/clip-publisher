package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"clip-publisher/internal/library"
	"clip-publisher/internal/scriptmodel"
	"clip-publisher/internal/store"
	"clip-publisher/internal/workspace"
)

const interruptedDetail = "Bị gián đoạn (app đã tắt khi đang chạy)"

// Recover marks jobs left running by a previous run as failed (PRD F5.6) and stops their
// leftover processes, so a Retry never runs alongside a pipeline from before the crash.
func (s *Service) Recover() error {
	jobs, err := s.d.Store.MarkInterrupted()
	if err != nil {
		return err
	}
	for _, j := range jobs {
		if j.PID > 0 && s.ownedProcess(j.PID) {
			killGroup(j.PID)
		}
	}
	vs, err := s.d.Store.ListVideos(true)
	if err != nil {
		return err
	}
	for _, v := range vs {
		if isBusy(v) {
			if err := s.d.Store.SetStatus(v.ID, store.StatusFailed, interruptedDetail); err != nil {
				return err
			}
		}
	}
	return nil
}

// Import syncs the library with make-video-free/final (PRD F2.2): adds new finals, picks up the
// new script of a re-published final, and hides imports that are not in final/ (drafts from
// output/ or worktrees, or finals that were removed). Videos made in the app are never touched.
func (s *Service) Import() (int, error) {
	cands, err := library.Scan(s.d.Settings().VideoRepo)
	if err != nil {
		return 0, err
	}
	inFinal := map[string]bool{}
	for _, c := range cands {
		inFinal[c.MP4] = true
	}
	if err := s.hideNonFinal(inFinal); err != nil {
		return 0, err
	}
	added := 0
	for _, c := range cands {
		v, err := s.d.Store.FindVideoBySource(c.MP4)
		if err == nil {
			if !v.Hidden { // a final the owner hid stays hidden
				if err := s.refreshFromFinal(v, c); err != nil {
					return added, err
				}
			}
			continue
		} else if !errors.Is(err, store.ErrNotFound) {
			return added, err
		}
		if err := s.addFinal(c); err != nil {
			return added, err
		}
		added++
	}
	return added, nil
}

func (s *Service) hideNonFinal(inFinal map[string]bool) error {
	vs, err := s.d.Store.ListVideos(false)
	if err != nil {
		return err
	}
	for _, v := range vs {
		if v.Origin != "import" || inFinal[v.SourcePath] {
			continue
		}
		v.Hidden = true
		if err := s.d.Store.UpdateVideo(v); err != nil {
			return err
		}
		s.changed(v.ID)
	}
	return nil
}

func (s *Service) addFinal(c library.Candidate) error {
	slug, _, err := s.d.WS.NewVideoDir(c.Title)
	if err != nil {
		return err
	}
	v := store.Video{Slug: slug, Title: c.Title, Origin: "import", SourcePath: c.MP4, Status: store.StatusReady,
		VoiceProfile: s.d.Settings().DefaultVoice, Editable: c.ScriptPath != ""}
	if err := s.d.Store.CreateVideo(&v); err != nil {
		return err
	}
	version := 0
	if c.ScriptPath != "" {
		b, err := os.ReadFile(c.ScriptPath)
		if err == nil {
			err = s.saveVersion(&v, b, scriptmodel.Notes{Slides: map[string]scriptmodel.SlideNote{}}, "import",
				"Import từ final", nil)
		}
		if err != nil {
			v.Editable = false
			_ = s.d.Store.UpdateVideo(v)
		} else {
			version = 1
		}
	}
	if err := s.addFinalRender(v.ID, version, c); err != nil {
		return err
	}
	s.changed(v.ID)
	return nil
}

func (s *Service) addFinalRender(videoID int64, version int, c library.Candidate) error {
	fin := time.Now()
	r := store.Render{VideoID: videoID, Version: version, Kind: "render", MP4Path: c.MP4,
		ReportPath: c.Report, State: "DONE", FinishedAt: &fin}
	return s.d.Store.AddRender(&r)
}

// refreshFromFinal stores the final's script as a new version when it differs from the current one
// (the video was re-published).
func (s *Service) refreshFromFinal(v store.Video, c library.Candidate) error {
	if c.ScriptPath == "" || !v.Editable {
		return nil
	}
	unlock := s.lockVideo(v.ID)
	defer unlock()
	v, err := s.d.Store.GetVideo(v.ID)
	if err != nil || isBusy(v) {
		return err
	}
	b, err := os.ReadFile(c.ScriptPath)
	if err != nil {
		return nil
	}
	if cur, _, _, err := s.current(v); err == nil && string(cur) == string(b) {
		return nil
	}
	notes := scriptmodel.Notes{Slides: map[string]scriptmodel.SlideNote{}}
	if err := s.saveVersion(&v, b, notes, "import", "Cập nhật từ final", nil); err != nil {
		return err
	}
	if err := s.addFinalRender(v.ID, v.CurrentVersion, c); err != nil {
		return err
	}
	s.changed(v.ID)
	return nil
}

// Hide removes a video from the library without touching files.
func (s *Service) Hide(videoID int64) error {
	v, err := s.d.Store.GetVideo(videoID)
	if err != nil {
		return err
	}
	v.Hidden = true
	if err := s.d.Store.UpdateVideo(v); err != nil {
		return err
	}
	s.changed(videoID)
	return nil
}

// trashTarget is the folder that deleting v moves to the Trash: the app's folder for videos made
// in the app, final/<name> for published videos, "" for old draft imports (only forgotten).
func (s *Service) trashTarget(v store.Video) string {
	if v.Origin != "import" {
		return s.videoDir(v)
	}
	if v.SourcePath == "" {
		return ""
	}
	dir := filepath.Dir(v.SourcePath)
	final := library.FinalDir(s.d.Settings().VideoRepo)
	if filepath.Dir(dir) != final { // only ever a folder directly inside final/
		return ""
	}
	return dir
}

// Delete moves the video's folder to the Trash (recoverable) and forgets it (PRD F2.3). For a
// published video that is make-video-free/final/<name>, so the next scan does not bring it back;
// the app's own copy (scripts, chat) goes to the Trash too. Draft imports are only forgotten.
func (s *Service) Delete(videoID int64) error {
	defer s.lockVideo(videoID)()
	v, err := s.d.Store.GetVideo(videoID)
	if err != nil {
		return err
	}
	if s.hasActiveJob(videoID) {
		return ErrBusy
	}
	targets := []string{s.trashTarget(v)}
	if v.Origin == "import" {
		targets = append(targets, s.videoDir(v))
	}
	for _, dir := range targets {
		if dir == "" {
			continue
		}
		if _, err := os.Stat(dir); err == nil && s.d.Trash != nil {
			if err := s.d.Trash(dir); err != nil {
				return err
			}
		}
	}
	if err := s.d.Store.DeleteVideo(videoID); err != nil {
		return err
	}
	s.changed(videoID)
	return nil
}

// --- media.Resolver ---

// RenderPath is the mp4 of a render.
func (s *Service) RenderPath(id int64) (string, error) {
	r, err := s.d.Store.GetRender(id)
	if err != nil {
		return "", err
	}
	return r.MP4Path, nil
}

// VideoDir is a video's workspace folder.
func (s *Service) VideoDir(videoID int64) (string, error) {
	v, err := s.d.Store.GetVideo(videoID)
	if err != nil {
		return "", err
	}
	return s.videoDir(v), nil
}

// ThumbPath returns (creating if needed) the thumbnail of a video's newest finished render.
func (s *Service) ThumbPath(ctx context.Context, videoID int64) (string, error) {
	renders, err := s.d.Store.ListRenders(videoID)
	if err != nil {
		return "", err
	}
	for _, r := range renders {
		if r.Kind == "render" && r.State == "DONE" {
			out := workspace.ThumbPath(s.d.WS.Root, videoID)
			if err := library.EnsureThumb(ctx, s.d.Runner, r.MP4Path, out); err != nil {
				return "", err
			}
			return out, nil
		}
	}
	return "", store.ErrNotFound
}
