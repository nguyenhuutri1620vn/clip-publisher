package pipeline

import (
	"os"
	"path/filepath"
	"strings"
)

// LookIn finds name on the PATH of env (exec.LookPath only sees the app's own PATH, which is
// minimal when launched from Finder). Falls back to name.
func LookIn(env []string, name string) string {
	for _, e := range env {
		if !strings.HasPrefix(e, "PATH=") {
			continue
		}
		for _, d := range strings.Split(strings.TrimPrefix(e, "PATH="), ":") {
			p := filepath.Join(d, name)
			if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
				return p
			}
		}
	}
	return name
}
