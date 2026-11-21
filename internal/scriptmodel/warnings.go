package scriptmodel

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Warning is a non-blocking problem shown before render. SlideID "" = whole script.
type Warning struct {
	SlideID string `json:"slideId"`
	Message string `json:"message"`
}

const (
	syllablesPerSecond = 3.0
	voiceRiskSeconds   = 12.5 // make-voice-free rejects > 14 s
	slidePadding       = 0.8  // theme lead-in 0.3 s + tail 0.5 s
	minTotal, maxTotal = 15.0, 90.0
)

var pauseRe = regexp.MustCompile(`\[pause (\d+)ms\]`)

// EstimateNarrationSeconds guesses how long the voice of a narration lasts.
func EstimateNarrationSeconds(text string) float64 {
	pauses := 0.0
	for _, m := range pauseRe.FindAllStringSubmatch(text, -1) {
		ms, _ := strconv.Atoi(m[1])
		pauses += float64(ms) / 1000
	}
	words := len(strings.Fields(pauseRe.ReplaceAllString(text, " ")))
	return float64(words)/syllablesPerSecond + pauses
}

// ScriptWarnings flags narrations at risk of the 14 s limit and a total outside 15–90 s.
func ScriptWarnings(s *Script) []Warning {
	out := []Warning{}
	total := 0.0
	for _, sl := range s.Slides {
		est := EstimateNarrationSeconds(sl.Narration)
		if est > voiceRiskSeconds {
			out = append(out, Warning{sl.ID, fmt.Sprintf("Lời đọc ~%.0fs, có thể vượt giới hạn 14s", est)})
		}
		total += est + slidePadding
	}
	if len(s.Slides) > 0 && (total < minTotal || total > maxTotal) {
		out = append(out, Warning{"", fmt.Sprintf("Tổng thời lượng ước tính %.0fs (nên 15–90s)", total)})
	}
	return out
}
