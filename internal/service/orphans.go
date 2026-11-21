package service

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Commands the app starts; a leftover process group is only killed if it runs one of these.
var pipelineMarkers = []string{"make_video.py", "history_shorts", "make-voice-free", "omnivoice", "ffmpeg", "claude"}

// isPipelineGroup reports whether process group pgid still runs one of the app's commands
// (pids are reused, so a bare pid is not proof the group is ours).
func isPipelineGroup(pgid int) bool {
	out, err := exec.Command("/bin/ps", "-A", "-o", "pgid=,command=").Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if g, err := strconv.Atoi(fields[0]); err != nil || g != pgid {
			continue
		}
		cmd := strings.Join(fields[1:], " ")
		for _, m := range pipelineMarkers {
			if strings.Contains(cmd, m) {
				return true
			}
		}
	}
	return false
}

// killGroup stops a process group: SIGTERM, then SIGKILL after 3 s.
func killGroup(pgid int) {
	if syscall.Kill(-pgid, syscall.SIGTERM) != nil {
		return
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(-pgid, 0) != nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
}
