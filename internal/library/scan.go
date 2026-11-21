// Package library reads the finished videos make-video-free publishes into its final/ folder.
//
// Convention (make-video-free/tools/publish_final.py): one folder per video,
// final/<name>/{<name>.mp4, <name>.report.json, script.json, final.json}. Anything in output/ or
// in worktrees is a draft and never shown.
package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"clip-publisher/internal/pipeline"
	"clip-publisher/internal/scriptmodel"
)

// Candidate is one published video.
type Candidate struct {
	MP4        string
	Report     string
	Title      string
	ScriptPath string
	Duration   float64
}

// FinalDir is where make-video-free publishes finished videos.
func FinalDir(videoRepo string) string { return filepath.Join(videoRepo, "final") }

type manifest struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	Video string `json:"video"`
}

// Scan lists the videos in final/, newest first. Folders without their video or report are skipped.
func Scan(videoRepo string) ([]Candidate, error) {
	manifests, err := filepath.Glob(filepath.Join(FinalDir(videoRepo), "*", "final.json"))
	if err != nil {
		return nil, err
	}
	type withTime struct {
		c   Candidate
		mod int64
	}
	var found []withTime
	for _, mf := range manifests {
		dir := filepath.Dir(mf)
		var m manifest
		if b, err := os.ReadFile(mf); err != nil || json.Unmarshal(b, &m) != nil {
			continue
		}
		if m.Video == "" {
			m.Video = filepath.Base(dir) + ".mp4"
		}
		mp4 := filepath.Join(dir, filepath.Base(m.Video))
		st, err := os.Stat(mp4)
		if err != nil {
			continue
		}
		rep := pipeline.ReportPath(mp4)
		r, err := pipeline.ReadReport(rep)
		if err != nil {
			continue
		}
		c := Candidate{MP4: mp4, Report: rep, Duration: r.TotalDuration, Title: m.Title}
		if sp := filepath.Join(dir, "script.json"); fileExists(sp) {
			c.ScriptPath = sp
			if c.Title == "" {
				if b, err := os.ReadFile(sp); err == nil {
					if s, err := scriptmodel.Parse(b); err == nil {
						c.Title = s.Title
					}
				}
			}
		}
		if c.Title == "" {
			c.Title = filepath.Base(dir)
		}
		found = append(found, withTime{c, st.ModTime().UnixNano()})
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].mod > found[j].mod })
	out := make([]Candidate, len(found))
	for i, f := range found {
		out[i] = f.c
	}
	return out, nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
