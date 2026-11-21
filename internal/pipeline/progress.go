package pipeline

import (
	"os"
	"path/filepath"
	"strings"

	"clip-publisher/internal/scriptmodel"
)

// Step is one stage of tools/make_video.py.
type Step string

const (
	StepValidate Step = "validate"
	StepExport   Step = "export"
	StepVoice    Step = "voice"
	StepAudio    Step = "audio"
	StepRender   Step = "render"
)

// Progress follows make_video.py through the "$ <command>" lines it prints before each step.
type Progress struct {
	Step      Step   `json:"step"`
	VoiceDone int    `json:"voiceDone"`
	VoiceID   string `json:"voiceId"`
}

// Feed reads one output line; it returns true when the progress changed.
func (p *Progress) Feed(line string) bool {
	if !strings.HasPrefix(line, "$ ") {
		return false
	}
	switch {
	case strings.Contains(line, "history_shorts validate"):
		p.Step = StepValidate
	case strings.Contains(line, "export-narration"):
		p.Step = StepExport
	case strings.Contains(line, "audio-manifest"):
		p.Step = StepAudio
	case strings.Contains(line, "history_shorts render"):
		p.Step = StepRender
	default:
		p.Step = StepVoice
		p.VoiceDone++
		p.VoiceID = ""
		for _, f := range strings.Fields(line) {
			f = strings.Trim(f, `'"`)
			if strings.HasSuffix(f, ".txt") {
				p.VoiceID = strings.TrimSuffix(filepath.Base(f), ".txt")
				break
			}
		}
	}
	return true
}

// PendingVoices counts narrated slides whose voice must be (re)recorded in voiceDir.
func PendingVoices(voiceDir string, s *scriptmodel.Script) int {
	n := 0
	for _, sl := range s.Slides {
		if strings.TrimSpace(sl.Narration) == "" {
			continue
		}
		txt, err := os.ReadFile(filepath.Join(voiceDir, sl.ID+".txt"))
		if err != nil || strings.TrimSpace(string(txt)) != strings.TrimSpace(sl.Narration) || !hasVoice(voiceDir, sl.ID) {
			n++
		}
	}
	return n
}

func hasVoice(dir, id string) bool {
	for _, ext := range []string{".wav", ".flac"} {
		if _, err := os.Stat(filepath.Join(dir, id+ext)); err == nil {
			return true
		}
	}
	return false
}
