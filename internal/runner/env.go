package runner

import "strings"

// Folders that a Finder-launched app lacks on PATH but the tools need (ffmpeg, node).
var systemDirs = []string{"/opt/homebrew/bin", "/usr/local/bin"}

// ChildEnv returns base with PATH = extraDirs + Homebrew dirs + the original PATH (deduplicated),
// plus PYTHONUNBUFFERED=1 and LANG=en_US.UTF-8 when not already set.
func ChildEnv(base []string, extraDirs ...string) []string {
	out := make([]string, 0, len(base)+3)
	var oldPath string
	hasPy, hasLang := false, false
	for _, e := range base {
		switch {
		case strings.HasPrefix(e, "PATH="):
			oldPath = strings.TrimPrefix(e, "PATH=")
			continue
		case strings.HasPrefix(e, "PYTHONUNBUFFERED="):
			hasPy = true
		case strings.HasPrefix(e, "LANG="):
			hasLang = true
		}
		out = append(out, e)
	}
	seen := map[string]bool{}
	var dirs []string
	add := func(d string) {
		if d != "" && d != "." && !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	for _, d := range extraDirs {
		add(d)
	}
	for _, d := range systemDirs {
		add(d)
	}
	for _, d := range strings.Split(oldPath, ":") {
		add(d)
	}
	out = append(out, "PATH="+strings.Join(dirs, ":"))
	if !hasPy {
		out = append(out, "PYTHONUNBUFFERED=1")
	}
	if !hasLang {
		out = append(out, "LANG=en_US.UTF-8")
	}
	return out
}
