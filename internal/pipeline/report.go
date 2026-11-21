package pipeline

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"

	"clip-publisher/internal/scriptmodel"
)

// ReportSlide is one slide of <video>.report.json.
type ReportSlide struct {
	Index    int     `json:"index"`
	Duration float64 `json:"duration"`
	PNG      string  `json:"png"`
}

// Report is the part of <video>.report.json the app uses.
type Report struct {
	ScriptSHA256  string        `json:"script_sha256"`
	TotalDuration float64       `json:"total_duration"`
	Slides        []ReportSlide `json:"slides"`
}

// ReadReport reads a render report.
func ReadReport(path string) (*Report, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Report
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("report %s: %w", path, err)
	}
	return &r, nil
}

// ReportPath is the report written next to a video: x.mp4 → x.report.json.
func ReportPath(mp4 string) string {
	return strings.TrimSuffix(mp4, ".mp4") + ".report.json"
}

// SlideStarts returns each slide's start time in seconds.
func SlideStarts(r *Report) []float64 {
	out := make([]float64, len(r.Slides))
	t := 0.0
	for i, s := range r.Slides {
		out[i] = t
		t += s.Duration
	}
	return out
}

const timingTolerance = 2.0

// TimingWarnings compares real slide lengths with the owner's "Thời gian" hints.
func TimingWarnings(s *scriptmodel.Script, n scriptmodel.Notes, r *Report) []scriptmodel.Warning {
	out := []scriptmodel.Warning{}
	for i, sl := range s.Slides {
		if i >= len(r.Slides) {
			break
		}
		hint, ok := scriptmodel.ParseTimeHint(n.Slides[sl.ID].TimeHint)
		if !ok {
			continue
		}
		d := r.Slides[i].Duration
		if math.Abs(d-hint) > timingTolerance {
			out = append(out, scriptmodel.Warning{SlideID: sl.ID,
				Message: fmt.Sprintf("Cảnh dài %.1fs, kịch bản ghi %.0fs", d, hint)})
		}
	}
	return out
}
