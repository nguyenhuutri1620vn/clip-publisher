package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"clip-publisher/internal/library"
	"clip-publisher/internal/pipeline"
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

// Import adds make-video-free/output renders that are not in the library yet (PRD F2.2).
func (s *Service) Import() (int, error) {
	cands, err := library.Scan(s.d.Settings().VideoRepo)
	if err != nil {
		return 0, err
	}
	cands, err = s.relinkMoved(cands)
	if err != nil {
		return 0, err
	}
	added := 0
	for _, c := range cands {
		if _, err := s.d.Store.FindVideoBySource(c.MP4); err == nil {
			continue
		} else if !errors.Is(err, store.ErrNotFound) {
			return added, err
		}
		slug, _, err := s.d.WS.NewVideoDir(c.Title)
		if err != nil {
			return added, err
		}
		v := store.Video{Slug: slug, Title: c.Title, Origin: "import", SourcePath: c.MP4, Status: store.StatusReady,
			VoiceProfile: s.d.Settings().DefaultVoice, Editable: c.ScriptPath != ""}
		if err := s.d.Store.CreateVideo(&v); err != nil {
			return added, err
		}
		version := 0
		if c.ScriptPath != "" {
			b, err := os.ReadFile(c.ScriptPath)
			if err == nil {
				err = s.saveVersion(&v, b, scriptmodel.Notes{Slides: map[string]scriptmodel.SlideNote{}}, "import",
					"Import từ "+c.ScriptPath, nil)
			}
			if err != nil {
				v.Editable = false
				_ = s.d.Store.UpdateVideo(v)
			} else {
				version = 1
			}
		}
		fin := time.Now()
		r := store.Render{VideoID: v.ID, Version: version, Kind: "render", MP4Path: c.MP4,
			ReportPath: pipeline.ReportPath(c.MP4), State: "DONE", FinishedAt: &fin}
		if err := s.d.Store.AddRender(&r); err != nil {
			return added, err
		}
		added++
		s.changed(v.ID)
	}
	return added, nil
}

// relinkMoved points imports whose file was moved (same file name, new folder) at the new
// location and hides imports whose file is gone. It returns the candidates still to add.
func (s *Service) relinkMoved(cands []library.Candidate) ([]library.Candidate, error) {
	vs, err := s.d.Store.ListVideos(true)
	if err != nil {
		return nil, err
	}
	tracked := map[string]bool{}
	for _, v := range vs {
		if v.Origin == "import" {
			tracked[v.SourcePath] = true
		}
	}
	byName := map[string][]library.Candidate{}
	for _, c := range cands {
		if !tracked[c.MP4] {
			byName[filepath.Base(c.MP4)] = append(byName[filepath.Base(c.MP4)], c)
		}
	}
	used := map[string]bool{}
	for _, v := range vs {
		if v.Origin != "import" || v.SourcePath == "" {
			continue
		}
		if _, err := os.Stat(v.SourcePath); err == nil {
			continue
		}
		matches := byName[filepath.Base(v.SourcePath)]
		if len(matches) == 1 && !used[matches[0].MP4] {
			c := matches[0]
			used[c.MP4] = true
			v.SourcePath = c.MP4
			v.Hidden = false
			if err := s.d.Store.UpdateVideo(v); err != nil {
				return nil, err
			}
			if r, err := s.d.Store.LatestRender(v.ID, "render"); err == nil {
				if err := s.d.Store.SetRenderPaths(r.ID, c.MP4, c.Report, r.LogPath); err != nil {
					return nil, err
				}
			}
		} else if !v.Hidden {
			v.Hidden = true // the file is gone: drop it from the library, keep the row
			if err := s.d.Store.UpdateVideo(v); err != nil {
				return nil, err
			}
		}
		s.changed(v.ID)
	}
	rest := cands[:0:0]
	for _, c := range cands {
		if !used[c.MP4] {
			rest = append(rest, c)
		}
	}
	return rest, nil
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

// Delete moves an app video's folder to the Trash and forgets it; imports are only hidden (PRD F2.3).
func (s *Service) Delete(videoID int64) error {
	defer s.lockVideo(videoID)()
	v, err := s.d.Store.GetVideo(videoID)
	if err != nil {
		return err
	}
	if s.hasActiveJob(videoID) {
		return ErrBusy
	}
	if v.Origin == "import" {
		return s.Hide(videoID)
	}
	dir := s.videoDir(v)
	if _, err := os.Stat(dir); err == nil && s.d.Trash != nil {
		if err := s.d.Trash(dir); err != nil {
			return err
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
