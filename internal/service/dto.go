package service

import (
	"time"

	"clip-publisher/internal/photos"
	"clip-publisher/internal/pipeline"
	"clip-publisher/internal/scriptmodel"
	"clip-publisher/internal/workspace"
)

// ProgressDTO is the live state of a running job, sent as "job:progress".
type ProgressDTO struct {
	pipeline.Progress
	VideoID    int64     `json:"videoId"`
	Kind       string    `json:"kind"` // "script" | "chat" | "render"
	VoiceTotal int       `json:"voiceTotal"`
	QueuePos   int       `json:"queuePos"`
	StartedAt  time.Time `json:"-"`
	ElapsedSec float64   `json:"elapsedSec"`
	EtaSec     float64   `json:"etaSec"`
}

func (p ProgressDTO) withTimes() ProgressDTO {
	if !p.StartedAt.IsZero() {
		p.ElapsedSec = time.Since(p.StartedAt).Seconds()
	}
	if p.Kind == "render" {
		left := p.VoiceTotal - p.VoiceDone
		if left < 0 {
			left = 0
		}
		p.EtaSec = float64(left*voiceETA + renderETA)
		if p.Step == pipeline.StepRender || p.Step == pipeline.StepAudio {
			p.EtaSec = renderETA
		}
	}
	return p
}

// VideoSummary is one row of the library.
type VideoSummary struct {
	ID           int64   `json:"id"`
	Title        string  `json:"title"`
	Status       string  `json:"status"`
	StatusDetail string  `json:"statusDetail"`
	Origin       string  `json:"origin"`
	Editable     bool    `json:"editable"`
	Duration     float64 `json:"duration"`
	UpdatedAt    string  `json:"updatedAt"`
	ThumbURL     string  `json:"thumbUrl"`
	QueuePos     int     `json:"queuePos"` // -1 = not queued, 0 = running, n = waiting
	Location     string  `json:"location"` // folder that "Xoá" moves to the Trash ("" = nothing on disk)
}

// SlideDTO is one slide card.
type SlideDTO struct {
	ID           string       `json:"id"`
	Title        string       `json:"title"`
	Subtitle     string       `json:"subtitle"`
	Narration    string       `json:"narration"`
	SceneSummary string       `json:"sceneSummary"`
	Note         string       `json:"note"`
	TimeHint     string       `json:"timeHint"`
	Claim        bool         `json:"claim"`
	NeedsCheck   bool         `json:"needsCheck"`
	EstSeconds   float64      `json:"estSeconds"`
	PreviewURL   string       `json:"previewUrl"`
	StartSec     float64      `json:"startSec"`
	RawJSON      string       `json:"rawJson"`
	PhotoQueries []string     `json:"photoQueries"`
	Photos       []photos.Ref `json:"photos"`
}

// VersionDTO is one script version.
type VersionDTO struct {
	N         int    `json:"n"`
	Author    string `json:"author"`
	Summary   string `json:"summary"`
	CreatedAt string `json:"createdAt"`
}

// RenderDTO is one render or preview.
type RenderDTO struct {
	ID        int64   `json:"id"`
	Version   int     `json:"version"`
	Kind      string  `json:"kind"`
	State     string  `json:"state"`
	URL       string  `json:"url"`
	Error     string  `json:"error"`
	Seconds   float64 `json:"seconds"`
	StartedAt string  `json:"startedAt"`
}

// SourceDTO is one citation.
type SourceDTO struct {
	Text       string `json:"text"`
	NeedsCheck bool   `json:"needsCheck"`
}

// VideoDetail is everything the video panel shows.
type VideoDetail struct {
	Video          VideoSummary              `json:"video"`
	Voice          string                    `json:"voice"`
	Music          string                    `json:"music"`
	CurrentVersion int                       `json:"currentVersion"`
	Versions       []VersionDTO              `json:"versions"`
	Slides         []SlideDTO                `json:"slides"`
	Sources        []SourceDTO               `json:"sources"`
	Warnings       []scriptmodel.Warning     `json:"warnings"`
	Changes        []scriptmodel.SlideChange `json:"changes"`
	Renders        []RenderDTO               `json:"renders"`
	Chat           []workspace.ChatMessage   `json:"chat"`
	LogTail        []string                  `json:"logTail"`
	Progress       *ProgressDTO              `json:"progress"`
}
