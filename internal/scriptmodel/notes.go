package scriptmodel

import (
	"encoding/json"
	"regexp"
	"strconv"
)

// SlideNote is Claude's remark on a slide plus the owner's original "Thời gian".
type SlideNote struct {
	Note         string   `json:"note"`
	TimeHint     string   `json:"time_hint"`
	PhotoQueries []string `json:"photo_queries,omitempty"`
}

// Notes are kept next to script.vN.json (never inside it).
type Notes struct {
	Slides map[string]SlideNote `json:"slides"`
}

// ParseNotes reads notes JSON; empty input is empty notes.
func ParseNotes(data []byte) (Notes, error) {
	n := Notes{Slides: map[string]SlideNote{}}
	if len(data) == 0 {
		return n, nil
	}
	if err := json.Unmarshal(data, &n); err != nil {
		return n, err
	}
	if n.Slides == nil {
		n.Slides = map[string]SlideNote{}
	}
	return n, nil
}

var timeHintRe = regexp.MustCompile(`^\s*(\d+):(\d{2})\s*[–—-]\s*(\d+):(\d{2})\s*$`)

// ParseTimeHint turns "0:04 – 0:12" into 8 seconds.
func ParseTimeHint(h string) (float64, bool) {
	m := timeHintRe.FindStringSubmatch(h)
	if m == nil {
		return 0, false
	}
	n := func(s string) int { v, _ := strconv.Atoi(s); return v }
	start := n(m[1])*60 + n(m[2])
	end := n(m[3])*60 + n(m[4])
	if end <= start {
		return 0, false
	}
	return float64(end - start), true
}
