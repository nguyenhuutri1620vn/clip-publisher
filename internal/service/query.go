package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"clip-publisher/internal/media"
	"clip-publisher/internal/pipeline"
	"clip-publisher/internal/scriptmodel"
	"clip-publisher/internal/store"
	"clip-publisher/internal/workspace"
)

func (s *Service) summary(v store.Video) VideoSummary {
	out := VideoSummary{ID: v.ID, Title: v.Title, Status: string(v.Status), StatusDetail: v.StatusDetail,
		Origin: v.Origin, Editable: v.Editable, UpdatedAt: v.UpdatedAt.Format(time.RFC3339),
		ThumbURL: media.ThumbURL(v.ID), QueuePos: s.renderQ.Position(renderKey(v.ID)),
		Location: s.trashTarget(v)}
	if r, err := s.d.Store.LatestRender(v.ID, "render"); err == nil && r.State == "DONE" {
		if rep, err := pipeline.ReadReport(r.ReportPath); err == nil {
			out.Duration = rep.TotalDuration
		}
	}
	if out.QueuePos < 0 && s.claudeQ.Position(claudeKey(v.ID)) >= 0 {
		out.QueuePos = s.claudeQ.Position(claudeKey(v.ID))
	}
	return out
}

// List returns the library (hidden videos excluded).
func (s *Service) List() ([]VideoSummary, error) {
	vs, err := s.d.Store.ListVideos(false)
	if err != nil {
		return nil, err
	}
	out := make([]VideoSummary, 0, len(vs))
	for _, v := range vs {
		out = append(out, s.summary(v))
	}
	return out, nil
}

// Detail returns everything the video panel needs.
func (s *Service) Detail(videoID int64) (VideoDetail, error) {
	v, err := s.d.Store.GetVideo(videoID)
	if err != nil {
		return VideoDetail{}, err
	}
	d := VideoDetail{Video: s.summary(v), Voice: v.VoiceProfile, Music: v.Music, CurrentVersion: v.CurrentVersion,
		Versions: []VersionDTO{}, Slides: []SlideDTO{}, Sources: []SourceDTO{}, Warnings: []scriptmodel.Warning{},
		Changes: []scriptmodel.SlideChange{}, Renders: []RenderDTO{}, Chat: []workspace.ChatMessage{}, LogTail: []string{}}
	dir := s.videoDir(v)

	vers, _ := s.d.Store.ListVersions(videoID)
	for _, ver := range vers {
		d.Versions = append(d.Versions, VersionDTO{N: ver.N, Author: ver.Author, Summary: ver.Summary,
			CreatedAt: ver.CreatedAt.Local().Format("02/01 15:04")})
	}
	renders, _ := s.d.Store.ListRenders(videoID)
	for _, r := range renders {
		d.Renders = append(d.Renders, RenderDTO{ID: r.ID, Version: r.Version, Kind: r.Kind, State: r.State,
			URL: media.RenderURL(r.ID), Error: r.Error, Seconds: r.Seconds, StartedAt: r.StartedAt.Local().Format("02/01 15:04")})
	}
	if v.Editable {
		d.Chat, _ = workspace.ReadChat(dir)
	}

	s.mu.Lock()
	if p, ok := s.progress[videoID]; ok {
		snap := p.withTimes()
		d.Progress = &snap
	}
	d.LogTail = append(d.LogTail, s.logTail[videoID]...)
	s.mu.Unlock()
	if len(d.LogTail) == 0 && v.Status == store.StatusFailed {
		if r, err := s.d.Store.LatestRender(videoID, "render"); err == nil && r.LogPath != "" {
			if b, err := os.ReadFile(r.LogPath); err == nil {
				d.LogTail = pipeline.Tail(string(b), 20)
			}
		}
	}

	_, sc, notes, err := s.current(v)
	if err != nil || sc == nil {
		return d, nil
	}
	for _, src := range sc.Sources {
		d.Sources = append(d.Sources, SourceDTO{Text: src, NeedsCheck: scriptmodel.NeedsCheck(src)})
	}
	var starts []float64
	var report *pipeline.Report
	for _, r := range renders { // newest first
		if r.Kind == "render" && r.State == "DONE" && r.Version == v.CurrentVersion {
			if rep, err := pipeline.ReadReport(r.ReportPath); err == nil {
				report, starts = rep, pipeline.SlideStarts(rep)
			}
			break
		}
	}
	previewDir := filepath.Dir(workspace.PreviewMP4Path(dir, v.CurrentVersion))
	for i, sl := range sc.Slides {
		n := notes.Slides[sl.ID]
		dto := SlideDTO{ID: sl.ID, Title: sl.Title, Subtitle: sl.Subtitle, Narration: sl.Narration,
			SceneSummary: sl.SceneSummary, Note: n.Note, TimeHint: n.TimeHint, Claim: sl.Claim,
			NeedsCheck: scriptmodel.NeedsCheck(sl.Title + sl.Subtitle + sl.Narration),
			EstSeconds: scriptmodel.EstimateNarrationSeconds(sl.Narration), RawJSON: string(sl.Raw)}
		png := filepath.Join(previewDir, "slides", fmt.Sprintf("%02d.png", i+1))
		if _, err := os.Stat(png); err == nil {
			rel, _ := filepath.Rel(dir, png)
			dto.PreviewURL = media.FileURL(videoID, filepath.ToSlash(rel))
		}
		if i < len(starts) {
			dto.StartSec = starts[i]
		}
		d.Slides = append(d.Slides, dto)
	}
	d.Warnings = append(d.Warnings, scriptmodel.ScriptWarnings(sc)...)
	for _, w := range s.validatorWarnings(v) {
		d.Warnings = append(d.Warnings, scriptmodel.Warning{Message: w})
	}
	if report != nil {
		d.Warnings = append(d.Warnings, pipeline.TimingWarnings(sc, notes, report)...)
	}
	if v.CurrentVersion > 1 {
		if prev, err := s.d.Store.GetVersion(videoID, v.CurrentVersion-1); err == nil {
			if b, err := os.ReadFile(prev.ScriptPath); err == nil {
				if ps, err := scriptmodel.Parse(b); err == nil {
					d.Changes = scriptmodel.Diff(ps, sc)
				}
			}
		}
	}
	return d, nil
}

// validatorWarnings returns cached validate warnings for the current version (re-validated once
// after a restart).
func (s *Service) validatorWarnings(v store.Video) []string {
	ver, err := s.d.Store.GetVersion(v.ID, v.CurrentVersion)
	if err != nil {
		return nil
	}
	s.mu.Lock()
	w, ok := s.warnings[ver.ScriptPath]
	s.mu.Unlock()
	if ok {
		return w
	}
	b, err := os.ReadFile(ver.ScriptPath)
	if err != nil {
		return nil
	}
	_, w, err = s.validate(context.Background(), s.videoDir(v), b)
	if err != nil {
		return nil
	}
	s.mu.Lock()
	s.warnings[ver.ScriptPath] = w
	s.mu.Unlock()
	return w
}
