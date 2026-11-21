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
	"clip-publisher/internal/pipeline"
	"clip-publisher/internal/store"
	"clip-publisher/internal/workspace"
)

const (
	voiceETA  = 30 // seconds per voice (PRD F5.2)
	renderETA = 60 // seconds for audio mix + encode
)

func (s *Service) setProgress(id int64, p *ProgressDTO) {
	s.mu.Lock()
	s.progress[id] = p
	s.mu.Unlock()
}

func (s *Service) clearProgress(id int64) {
	s.mu.Lock()
	delete(s.progress, id)
	s.mu.Unlock()
}

func (s *Service) emitProgress(id int64) {
	s.mu.Lock()
	p, ok := s.progress[id]
	var snap ProgressDTO
	if ok {
		snap = *p
	}
	s.mu.Unlock()
	if ok {
		s.d.Emit("job:progress", snap.withTimes())
	}
}

func (s *Service) addLog(id int64, line string) {
	s.mu.Lock()
	t := append(s.logTail[id], line)
	if len(t) > liveLogLines {
		t = t[len(t)-liveLogLines:]
	}
	s.logTail[id] = t
	s.mu.Unlock()
	s.d.Emit("job:log", map[string]any{"videoId": id, "line": line})
}

// Preview draws the slide PNGs of the current version (PRD F4.3). Status is unchanged.
func (s *Service) Preview(videoID int64) error {
	defer s.lockVideo(videoID)()
	return s.preview(videoID)
}

func (s *Service) preview(videoID int64) error {
	v, err := s.editable(videoID)
	if err != nil {
		return err
	}
	if s.renderQ.Position(renderKey(videoID)) >= 0 {
		return ErrBusy
	}
	ver, err := s.d.Store.GetVersion(videoID, v.CurrentVersion)
	if err != nil {
		return err
	}
	out := workspace.PreviewMP4Path(s.videoDir(v), v.CurrentVersion)
	r := store.Render{VideoID: videoID, Version: v.CurrentVersion, Kind: "preview", MP4Path: out, State: "RUNNING"}
	if err := s.d.Store.AddRender(&r); err != nil {
		return err
	}
	job := store.Job{VideoID: videoID, Kind: "preview", State: "QUEUED"}
	_ = s.d.Store.AddJob(&job)
	s.renderQ.Enqueue(renderKey(videoID), func(ctx context.Context) error {
		_ = s.d.Store.SetJobState(job.ID, "RUNNING", "")
		start := time.Now()
		_ = os.MkdirAll(filepath.Dir(out), 0o755)
		cmd := pipeline.PreviewCmd(s.d.Settings(), ver.ScriptPath, out)
		cmd.OnStart = s.recordPID(job.ID)
		res, err := s.d.Runner.Run(ctx, cmd, nil)
		state, msg := "DONE", ""
		switch {
		case ctx.Err() != nil:
			state = "CANCELED"
		case err != nil:
			state, msg = "FAILED", err.Error()
		case res.ExitCode != 0:
			state, msg = "FAILED", strings.Join(pipeline.Tail(res.Output, 3), " / ")
		}
		_ = s.d.Store.FinishRender(r.ID, state, msg, time.Since(start).Seconds())
		_ = s.d.Store.SetJobState(job.ID, state, msg)
		s.changed(videoID)
		return nil
	})
	s.changed(videoID)
	return nil
}

// Render runs the voice + video pipeline on the current version (PRD F5).
func (s *Service) Render(videoID int64) error {
	defer s.lockVideo(videoID)()
	return s.render(videoID)
}

func (s *Service) render(videoID int64) error {
	v, err := s.editable(videoID)
	if err != nil {
		return err
	}
	if s.renderQ.Position(renderKey(videoID)) >= 0 {
		return ErrBusy
	}
	ver, err := s.d.Store.GetVersion(videoID, v.CurrentVersion)
	if err != nil {
		return err
	}
	_, sc, _, err := s.current(v)
	if err != nil {
		return err
	}
	set := s.d.Settings()
	dir := s.videoDir(v)
	r := store.Render{VideoID: videoID, Version: v.CurrentVersion, Kind: "render", State: "RUNNING"}
	if err := s.d.Store.AddRender(&r); err != nil {
		return err
	}
	// Files are named after the render ID, so a re-render never overwrites an earlier one.
	r.MP4Path = workspace.RenderMP4Path(dir, v.CurrentVersion, r.ID)
	r.LogPath = workspace.RenderLogPath(dir, v.CurrentVersion, r.ID)
	r.ReportPath = pipeline.ReportPath(r.MP4Path)
	if err := s.d.Store.SetRenderPaths(r.ID, r.MP4Path, r.ReportPath, r.LogPath); err != nil {
		return err
	}
	job := store.Job{VideoID: videoID, Kind: "render", State: "QUEUED"}
	_ = s.d.Store.AddJob(&job)
	if err := s.d.Store.SetStatus(videoID, store.StatusRendering, ""); err != nil {
		return err
	}
	voiceDir := workspace.VoiceDir(dir)
	p := &ProgressDTO{VideoID: videoID, Kind: "render", VoiceTotal: pipeline.PendingVoices(voiceDir, sc)}
	s.setProgress(videoID, p)
	s.mu.Lock()
	s.logTail[videoID] = nil
	s.mu.Unlock()

	pos := s.renderQ.Enqueue(renderKey(videoID), func(ctx context.Context) error {
		_ = s.d.Store.SetJobState(job.ID, "RUNNING", "")
		s.mu.Lock()
		p.QueuePos, p.StartedAt = 0, time.Now()
		s.mu.Unlock()
		s.emitProgress(videoID)
		s.changed(videoID)
		s.renderJob(ctx, videoID, v.Title, r, job, ver.ScriptPath, voiceDir, v.VoiceProfile, v.Music, set, p)
		s.clearProgress(videoID)
		s.changed(videoID)
		return nil
	})
	s.mu.Lock()
	p.QueuePos = pos
	s.mu.Unlock()
	s.changed(videoID)
	return nil
}

func (s *Service) renderJob(ctx context.Context, videoID int64, title string, r store.Render, job store.Job,
	script, voiceDir, voice, music string, set config.Settings, p *ProgressDTO) {
	start := time.Now()
	var logMu sync.Mutex
	_ = os.MkdirAll(filepath.Dir(r.LogPath), 0o755)
	logf, _ := os.OpenFile(r.LogPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	cmd := pipeline.RenderCmd(set, script, voiceDir, r.MP4Path, config.VoiceProfilePath(set, voice), music)
	cmd.OnStart = s.recordPID(job.ID)
	res, err := s.d.Runner.Run(ctx, cmd, func(line string) {
		logMu.Lock()
		if logf != nil {
			fmt.Fprintln(logf, line)
		}
		logMu.Unlock()
		s.mu.Lock()
		changed := (&p.Progress).Feed(line)
		s.mu.Unlock()
		s.addLog(videoID, line)
		if changed {
			s.emitProgress(videoID)
		}
	})
	if logf != nil {
		logf.Close()
	}
	secs := time.Since(start).Seconds()
	s.mu.Lock()
	step := string(p.Step)
	s.mu.Unlock()
	_, statErr := os.Stat(r.MP4Path)
	switch {
	case ctx.Err() != nil || errors.Is(err, context.Canceled):
		_ = s.d.Store.FinishRender(r.ID, "CANCELED", "", secs)
		_ = s.d.Store.SetJobState(job.ID, "CANCELED", "")
		_ = s.d.Store.SetStatus(videoID, store.StatusScriptReview, "Đã huỷ render")
	case err != nil || res.ExitCode != 0 || statErr != nil:
		msg := lastMeaningful(res.Output)
		if err != nil {
			msg = err.Error()
		}
		if step == "" {
			step = string(pipeline.StepValidate)
		}
		_ = s.d.Store.FinishRender(r.ID, "FAILED", msg, secs)
		_ = s.d.Store.SetJobState(job.ID, "FAILED", msg)
		_ = s.d.Store.SetStatus(videoID, store.StatusFailed, fmt.Sprintf("Render (%s): %s", step, msg))
	default:
		_ = s.d.Store.FinishRender(r.ID, "DONE", "", secs)
		_ = s.d.Store.SetJobState(job.ID, "DONE", "")
		_ = s.d.Store.SetStatus(videoID, store.StatusReady, "")
		s.d.Notify("Video đã render xong", title)
	}
}

// lastMeaningful prefers the last ERROR line, else the last line.
func lastMeaningful(out string) string {
	tail := pipeline.Tail(out, 20)
	for i := len(tail) - 1; i >= 0; i-- {
		if strings.HasPrefix(strings.TrimSpace(tail[i]), "ERROR") {
			return strings.TrimSpace(tail[i])
		}
	}
	if len(tail) == 0 {
		return "không có output"
	}
	return strings.TrimSpace(tail[len(tail)-1])
}

// Cancel stops (or un-queues) whatever job the video has.
func (s *Service) Cancel(videoID int64) error {
	if pos := s.renderQ.Position(renderKey(videoID)); pos >= 0 {
		s.renderQ.Cancel(renderKey(videoID))
		if pos > 0 { // never started: settle its rows here
			s.settleCanceledRender(videoID)
		}
		return nil
	}
	if pos := s.claudeQ.Position(claudeKey(videoID)); pos >= 0 {
		s.claudeQ.Cancel(claudeKey(videoID))
		if pos > 0 {
			if j, err := s.d.Store.LastJob(videoID); err == nil {
				_ = s.d.Store.SetJobState(j.ID, "CANCELED", "")
			}
			_ = s.d.Store.SetStatus(videoID, store.StatusFailed, "Đã huỷ")
			s.changed(videoID)
		}
		return nil
	}
	return errors.New("Video không có việc nào đang chạy")
}

func (s *Service) settleCanceledRender(videoID int64) {
	if r, err := s.d.Store.LatestRender(videoID, "render"); err == nil && r.State == "RUNNING" {
		_ = s.d.Store.FinishRender(r.ID, "CANCELED", "", 0)
	}
	if r, err := s.d.Store.LatestRender(videoID, "preview"); err == nil && r.State == "RUNNING" {
		_ = s.d.Store.FinishRender(r.ID, "CANCELED", "", 0)
	}
	if j, err := s.d.Store.LastJob(videoID); err == nil {
		_ = s.d.Store.SetJobState(j.ID, "CANCELED", "")
	}
	if v, err := s.d.Store.GetVideo(videoID); err == nil && v.Status == store.StatusRendering {
		_ = s.d.Store.SetStatus(videoID, store.StatusScriptReview, "Đã huỷ render")
	}
	s.clearProgress(videoID)
	s.changed(videoID)
}

// Retry re-runs the last job of a video (PRD F3.6, F5.4, F5.6).
func (s *Service) Retry(videoID int64) error {
	defer s.lockVideo(videoID)()
	v, err := s.d.Store.GetVideo(videoID)
	if err != nil {
		return err
	}
	if isBusy(v) && s.hasActiveJob(videoID) {
		return ErrBusy
	}
	if isBusy(v) {
		_ = s.d.Store.SetStatus(videoID, store.StatusFailed, v.StatusDetail)
	}
	j, err := s.d.Store.LastJob(videoID)
	if err != nil {
		return errors.New("không có việc nào để chạy lại")
	}
	dir := s.videoDir(v)
	switch j.Kind {
	case "script":
		input := readInput(dir)
		if strings.TrimSpace(input) == "" {
			return errors.New("không tìm thấy kịch bản gốc (input.md)")
		}
		if err := s.d.Store.SetStatus(videoID, store.StatusGeneratingScript, ""); err != nil {
			return err
		}
		s.runClaude(videoID, "script", scriptgenRequestNew(input), store.StatusFailed)
		s.changed(videoID)
		return nil
	case "chat":
		msg := lastUserMessage(dir)
		if msg == "" {
			return errors.New("không có yêu cầu nào để gửi lại")
		}
		_ = s.d.Store.SetStatus(videoID, store.StatusScriptReview, "")
		raw, _, notes, err := s.current(v)
		if err != nil {
			return err
		}
		chat, _ := workspace.ReadChat(dir)
		_ = s.d.Store.SetStatus(videoID, store.StatusGeneratingScript, "")
		s.runClaude(videoID, "chat", scriptgenRequestEdit(raw, notes, chat, msg), store.StatusScriptReview)
		s.changed(videoID)
		return nil
	case "preview":
		return s.preview(videoID)
	default:
		_ = s.d.Store.SetStatus(videoID, store.StatusScriptReview, "")
		return s.render(videoID)
	}
}

func (s *Service) hasActiveJob(id int64) bool {
	return s.renderQ.Position(renderKey(id)) >= 0 || s.claudeQ.Position(claudeKey(id)) >= 0
}
