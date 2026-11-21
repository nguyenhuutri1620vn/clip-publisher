// Package pipeline builds the commands that run make-video-free / make-voice-free and reads what
// they print and write.
package pipeline

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"clip-publisher/internal/config"
	"clip-publisher/internal/runner"
)

func env(s config.Settings) []string {
	return runner.ChildEnv(os.Environ(), filepath.Dir(s.ClaudeBin))
}

func hs(s config.Settings, args ...string) runner.Cmd {
	return runner.Cmd{
		Name: s.VideoPython,
		Args: append([]string{"-m", "history_shorts"}, args...),
		Dir:  s.VideoRepo,
		Env:  env(s),
	}
}

// ValidateCmd checks a script.json with history-shorts.
func ValidateCmd(s config.Settings, script string) runner.Cmd {
	return hs(s, "validate", "--script", script)
}

// PreviewCmd draws the slide PNGs (no voice, no FFmpeg) into <dir of outMP4>/slides.
func PreviewCmd(s config.Settings, script, outMP4 string) runner.Cmd {
	return hs(s, "render", "--script", script, "--out", outMP4, "--preview")
}

// RenderCmd runs the one-command pipeline tools/make_video.py (voice + video).
func RenderCmd(s config.Settings, script, workDir, outMP4, voiceProfile, music string) runner.Cmd {
	args := []string{"tools/make_video.py", "--script", script, "--work", workDir, "--out", outMP4}
	if m := config.MusicPath(s, music); m != "" {
		args = append(args, "--music", m)
	}
	args = append(args, "--voice-cmd", VoiceCmdTemplate(s.VoiceBin, voiceProfile))
	return runner.Cmd{Name: s.VideoPython, Args: args, Dir: s.VideoRepo, Env: env(s)}
}

var safeArg = regexp.MustCompile(`^[A-Za-z0-9_./-]+$`)

func shellQuote(s string) string {
	if safeArg.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// VoiceCmdTemplate is the --voice-cmd value; make_video.py shlex-splits it, so paths are quoted.
func VoiceCmdTemplate(voiceBin, profilePath string) string {
	return shellQuote(voiceBin) + " voice {text} {wav} --voice " + shellQuote(profilePath) + " --allow-noncommercial"
}

// ParseValidate splits `history_shorts validate` output into errors and warnings
// (the "WARN " prefix is removed from warnings).
func ParseValidate(output string) (errs, warns []string) {
	for _, l := range strings.Split(output, "\n") {
		l = strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(l, "WARN "):
			warns = append(warns, strings.TrimPrefix(l, "WARN "))
		case strings.HasPrefix(l, "ERROR"), strings.Contains(l, "›"):
			errs = append(errs, l)
		}
	}
	return errs, warns
}

// Tail returns the last n non-empty lines of text.
func Tail(text string, n int) []string {
	lines := []string{}
	for _, l := range strings.Split(text, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}
