package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"clip-publisher/internal/scriptgen"
	"clip-publisher/internal/scriptmodel"
	"clip-publisher/internal/store"
	"clip-publisher/internal/workspace"
)

func jsonIndent(v any) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	return append(b, '\n'), err
}

// titleHint picks a folder name from the owner's text: the first "Chữ trên hình:" value, else the
// first non-empty line.
func titleHint(input string) string {
	first := ""
	for _, l := range strings.Split(input, "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if first == "" {
			first = l
		}
		if i := strings.Index(l, "Chữ trên hình:"); i >= 0 {
			if v := strings.TrimSpace(l[i+len("Chữ trên hình:"):]); v != "" {
				return v
			}
		}
	}
	return first
}

// CreateFromText starts a new video from free-form text (PRD F3.1).
func (s *Service) CreateFromText(input, voice, music string) (int64, error) {
	if strings.TrimSpace(input) == "" {
		return 0, errors.New("Hãy dán kịch bản trước")
	}
	set := s.d.Settings()
	if voice == "" {
		voice = set.DefaultVoice
	}
	slug, dir, err := s.d.WS.NewVideoDir(titleHint(input))
	if err != nil {
		return 0, err
	}
	if err := workspace.WriteFileAtomic(workspace.InputPath(dir), []byte(input)); err != nil {
		return 0, err
	}
	v := store.Video{Slug: slug, Title: titleHint(input), Origin: "app", Status: store.StatusGeneratingScript,
		VoiceProfile: voice, Music: music, Editable: true}
	if err := s.d.Store.CreateVideo(&v); err != nil {
		return 0, err
	}
	s.runClaude(v.ID, "script", scriptgen.Request{Input: input}, store.StatusFailed)
	s.changed(v.ID)
	return v.ID, nil
}

func (s *Service) generator() (*scriptgen.Generator, error) {
	set := s.d.Settings()
	contract, err := scriptgen.LoadContract(set.VideoRepo)
	if err != nil {
		return nil, err
	}
	examples, err := scriptgen.LoadExamples(set.VideoRepo)
	if err != nil {
		return nil, err
	}
	return &scriptgen.Generator{Runner: s.d.Runner, Settings: set, Contract: contract, Examples: examples,
		Timeout: s.d.ClaudeTimeout, MaxRepairs: maxRepairs}, nil
}

var stepLabel = map[string]string{"script": "Tạo script", "chat": "Sửa bằng Claude"}

// runClaude queues a Claude job; onCancel is the status the video returns to when canceled.
func (s *Service) runClaude(videoID int64, kind string, req scriptgen.Request, onCancel store.Status) {
	job := store.Job{VideoID: videoID, Kind: kind, State: "QUEUED"}
	_ = s.d.Store.AddJob(&job)
	s.claudeQ.Enqueue(claudeKey(videoID), func(ctx context.Context) error {
		_ = s.d.Store.SetJobState(job.ID, "RUNNING", "")
		s.setProgress(videoID, &ProgressDTO{VideoID: videoID, Kind: kind, StartedAt: time.Now()})
		defer s.clearProgress(videoID)
		err := s.claudeJob(ctx, videoID, kind, req)
		switch {
		case ctx.Err() != nil:
			_ = s.d.Store.SetJobState(job.ID, "CANCELED", "")
			_ = s.d.Store.SetStatus(videoID, onCancel, "Đã huỷ: "+stepLabel[kind])
		case err != nil:
			_ = s.d.Store.SetJobState(job.ID, "FAILED", err.Error())
			_ = s.d.Store.SetStatus(videoID, store.StatusFailed, stepLabel[kind]+": "+err.Error())
		default:
			_ = s.d.Store.SetJobState(job.ID, "DONE", "")
		}
		s.changed(videoID)
		return err
	})
}

func (s *Service) claudeJob(ctx context.Context, videoID int64, kind string, req scriptgen.Request) error {
	v, err := s.d.Store.GetVideo(videoID)
	if err != nil {
		return err
	}
	gen, err := s.generator()
	if err != nil {
		return err
	}
	dir := s.videoDir(v)
	var warns []string
	out, err := gen.Generate(ctx, req, func(ctx context.Context, script []byte) ([]string, error) {
		errs, w, err := s.validate(ctx, dir, script)
		warns = w
		return errs, err
	})
	var inv *scriptgen.InvalidScriptError
	if errors.As(err, &inv) && inv.Last != nil {
		// Keep Claude's last attempt so the owner can fix it by hand (PRD F3.6).
		if serr := s.saveVersion(&v, inv.Last.Script, inv.Last.Notes, "claude", inv.Last.Summary, warns); serr != nil {
			return serr
		}
		return errors.New(inv.Error() + ":\n" + strings.Join(inv.Errors, "\n"))
	}
	if err != nil {
		return err
	}
	if out.Notes.Slides == nil {
		out.Notes.Slides = map[string]scriptmodel.SlideNote{}
	}
	if err := s.saveVersion(&v, out.Script, out.Notes, "claude", out.Summary, warns); err != nil {
		return err
	}
	if kind == "chat" && out.Summary != "" {
		_ = workspace.AppendChat(dir, workspace.ChatMessage{Role: "assistant", Text: out.Summary, At: time.Now()})
	}
	return s.d.Store.SetStatus(videoID, store.StatusScriptReview, "")
}

// lastUserMessage returns the newest owner message of a video's chat.
func lastUserMessage(dir string) string {
	msgs, _ := workspace.ReadChat(dir)
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			return msgs[i].Text
		}
	}
	return ""
}

func readInput(dir string) string {
	b, _ := os.ReadFile(workspace.InputPath(dir))
	return string(b)
}

func scriptgenRequestNew(input string) scriptgen.Request { return scriptgen.Request{Input: input} }

func scriptgenRequestEdit(raw []byte, notes scriptmodel.Notes, chat []workspace.ChatMessage, msg string) scriptgen.Request {
	return scriptgen.Request{Current: raw, Notes: &notes, Chat: chat, Instruction: msg}
}
