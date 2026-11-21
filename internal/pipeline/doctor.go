package pipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	"clip-publisher/internal/config"
	"clip-publisher/internal/runner"
)

// Check is the result of probing one external tool.
type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// Doctor probes every tool the app needs (PRD F1.3).
func Doctor(ctx context.Context, s config.Settings, r runner.Runner) []Check {
	e := env(s)
	probes := []struct {
		name string
		cmd  runner.Cmd
	}{
		{"make-video-free (history-shorts)", runner.Cmd{Name: s.VideoPython, Args: []string{"-m", "history_shorts", "--version"}, Dir: s.VideoRepo, Env: e}},
		{"make-voice-free", runner.Cmd{Name: s.VoiceBin, Args: []string{"--help"}, Env: e}},
		{"Claude Code CLI", runner.Cmd{Name: s.ClaudeBin, Args: []string{"--version"}, Env: e}},
		{"FFmpeg", runner.Cmd{Name: "ffmpeg", Args: []string{"-version"}, Env: e}},
	}
	out := make([]Check, 0, len(probes))
	for _, p := range probes {
		p.cmd.Timeout = 10 * time.Second
		if p.cmd.Name == "ffmpeg" {
			p.cmd.Name = LookIn(e, "ffmpeg")
		}
		res, err := r.Run(ctx, p.cmd, nil)
		c := Check{Name: p.name}
		switch {
		case err != nil:
			c.Detail = fmt.Sprintf("Không chạy được: %v", err)
		case res.ExitCode != 0:
			c.Detail = fmt.Sprintf("Không chạy được: thoát với mã %d — %s", res.ExitCode, firstLine(res.Output))
		default:
			c.OK, c.Detail = true, firstLine(res.Output)
		}
		out = append(out, c)
	}
	return out
}

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}
