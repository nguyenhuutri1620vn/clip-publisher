package library

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"clip-publisher/internal/pipeline"
	"clip-publisher/internal/runner"
)

// ThumbCmd grabs one small frame of mp4 into outJPG.
func ThumbCmd(ffmpeg, mp4, outJPG string) runner.Cmd {
	return runner.Cmd{
		Name: ffmpeg,
		Args: []string{"-y", "-ss", "0.5", "-i", mp4, "-frames:v", "1", "-vf", "scale=270:-1", outJPG},
	}
}

// EnsureThumb creates outJPG unless it exists and is newer than mp4.
func EnsureThumb(ctx context.Context, r runner.Runner, mp4, outJPG string) error {
	src, err := os.Stat(mp4)
	if err != nil {
		return err
	}
	if dst, err := os.Stat(outJPG); err == nil && dst.ModTime().After(src.ModTime()) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(outJPG), 0o755); err != nil {
		return err
	}
	env := runner.ChildEnv(os.Environ())
	c := ThumbCmd(pipeline.LookIn(env, "ffmpeg"), mp4, outJPG)
	c.Env, c.Timeout = env, 20*time.Second
	res, err := r.Run(ctx, c, nil)
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("ffmpeg thumbnail exit %d", res.ExitCode)
	}
	return nil
}
