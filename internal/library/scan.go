// Package library finds videos already rendered by make-video-free so they can be imported.
package library

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"clip-publisher/internal/pipeline"
	"clip-publisher/internal/scriptmodel"
)

// Candidate is an mp4 in make-video-free/output with its report.
type Candidate struct {
	MP4        string
	Report     string
	Title      string
	ScriptPath string // "" when no example script matches the report's script_sha256
	Duration   float64
}

// roots are make-video-free itself plus its Claude Code worktrees (.claude/worktrees/*), where
// the owner usually renders new videos.
func roots(videoRepo string) []string {
	wts, _ := filepath.Glob(filepath.Join(videoRepo, ".claude", "worktrees", "*"))
	sort.Strings(wts)
	return append([]string{videoRepo}, wts...)
}

// Scan lists <root>/output/*.mp4 and <root>/output/<folder>/*.mp4 that have a
// <stem>.report.json next to them, for every root, newest first.
func Scan(videoRepo string) ([]Candidate, error) {
	var mp4s []string
	scripts := map[string]string{}
	for _, root := range roots(videoRepo) {
		for _, pattern := range []string{"*.mp4", filepath.Join("*", "*.mp4")} {
			found, err := filepath.Glob(filepath.Join(root, "output", pattern))
			if err != nil {
				return nil, err
			}
			mp4s = append(mp4s, found...)
		}
		hashes, err := exampleHashes(root)
		if err != nil {
			return nil, err
		}
		for h, p := range hashes {
			if _, seen := scripts[h]; !seen { // the main repo wins over worktrees
				scripts[h] = p
			}
		}
	}
	type withTime struct {
		c   Candidate
		mod int64
	}
	var found []withTime
	for _, mp4 := range mp4s {
		rep := pipeline.ReportPath(mp4)
		r, err := pipeline.ReadReport(rep)
		if err != nil {
			continue
		}
		c := Candidate{MP4: mp4, Report: rep, Duration: r.TotalDuration}
		if sp, ok := scripts[r.ScriptSHA256]; ok {
			c.ScriptPath = sp
			if b, err := os.ReadFile(sp); err == nil {
				if s, err := scriptmodel.Parse(b); err == nil {
					c.Title = s.Title
				}
			}
		}
		if c.Title == "" {
			stem := strings.TrimSuffix(filepath.Base(mp4), ".mp4")
			c.Title = strings.NewReplacer("_", " ", "-", " ").Replace(stem)
		}
		st, _ := os.Stat(mp4)
		found = append(found, withTime{c, st.ModTime().UnixNano()})
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].mod > found[j].mod })
	out := make([]Candidate, len(found))
	for i, f := range found {
		out[i] = f.c
	}
	return out, nil
}

func exampleHashes(videoRepo string) (map[string]string, error) {
	paths, err := filepath.Glob(filepath.Join(videoRepo, "examples", "*", "script.json"))
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(b)
		out[hex.EncodeToString(sum[:])] = p
	}
	return out, nil
}
