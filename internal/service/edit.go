package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"clip-publisher/internal/scriptgen"
	"clip-publisher/internal/scriptmodel"
	"clip-publisher/internal/store"
	"clip-publisher/internal/workspace"
)

var errNotEditable = errors.New("Video import không có script gốc — chỉ xem")

func (s *Service) editable(id int64) (store.Video, error) {
	v, err := s.d.Store.GetVideo(id)
	if err != nil {
		return v, err
	}
	if !v.Editable {
		return v, errNotEditable
	}
	if isBusy(v) {
		return v, ErrBusy
	}
	return v, nil
}

// Chat asks Claude to change the current script (PRD F6.2).
func (s *Service) Chat(videoID int64, message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return errors.New("Hãy nhập yêu cầu")
	}
	v, err := s.editable(videoID)
	if err != nil {
		return err
	}
	raw, _, notes, err := s.current(v)
	if err != nil {
		return err
	}
	dir := s.videoDir(v)
	if err := workspace.AppendChat(dir, workspace.ChatMessage{Role: "user", Text: message, At: time.Now()}); err != nil {
		return err
	}
	chat, _ := workspace.ReadChat(dir)
	prev := v.Status
	if err := s.d.Store.SetStatus(videoID, store.StatusGeneratingScript, ""); err != nil {
		return err
	}
	s.runClaude(videoID, "chat", scriptgen.Request{Current: raw, Notes: &notes, Chat: chat, Instruction: message}, prev)
	s.changed(videoID)
	return nil
}

// saveManual validates script bytes and stores them as a manual version.
func (s *Service) saveManual(v store.Video, script []byte, author, summary string) error {
	_, _, notes, _ := s.current(v)
	if notes.Slides == nil {
		notes = scriptmodel.Notes{Slides: map[string]scriptmodel.SlideNote{}}
	}
	errs, warns, err := s.validate(context.Background(), s.videoDir(v), script)
	if err != nil {
		return err
	}
	if len(errs) > 0 {
		return fmt.Errorf("Script không hợp lệ:\n%s", strings.Join(errs, "\n"))
	}
	if err := s.saveVersion(&v, script, notes, author, summary, warns); err != nil {
		return err
	}
	if err := s.d.Store.SetStatus(v.ID, store.StatusScriptReview, ""); err != nil {
		return err
	}
	s.changed(v.ID)
	return nil
}

// EditSlideText changes one slide's title, subtitle and narration (PRD F6.3).
func (s *Service) EditSlideText(videoID int64, slideID, title, subtitle, narration string) error {
	v, err := s.editable(videoID)
	if err != nil {
		return err
	}
	_, sc, _, err := s.current(v)
	if err != nil {
		return err
	}
	out, err := sc.WithSlideText(slideID, title, subtitle, narration)
	if err != nil {
		return err
	}
	return s.saveManual(v, out, "manual", "Sửa tay cảnh "+slideID)
}

// EditSlideRaw replaces one slide's JSON (PRD F6.3, advanced).
func (s *Service) EditSlideRaw(videoID int64, slideID, raw string) error {
	v, err := s.editable(videoID)
	if err != nil {
		return err
	}
	_, sc, _, err := s.current(v)
	if err != nil {
		return err
	}
	out, err := sc.WithSlideRaw(slideID, []byte(raw))
	if err != nil {
		return err
	}
	return s.saveManual(v, out, "manual", "Sửa JSON cảnh "+slideID)
}

// RestoreVersion makes a new version with the content of version n (PRD F6.4).
func (s *Service) RestoreVersion(videoID int64, n int) error {
	v, err := s.editable(videoID)
	if err != nil {
		return err
	}
	ver, err := s.d.Store.GetVersion(videoID, n)
	if err != nil {
		return err
	}
	script, err := os.ReadFile(ver.ScriptPath)
	if err != nil {
		return err
	}
	nb, _ := os.ReadFile(ver.NotesPath)
	notes, _ := scriptmodel.ParseNotes(nb)
	errs, warns, err := s.validate(context.Background(), s.videoDir(v), script)
	if err != nil {
		return err
	}
	_ = errs // restoring an old version is allowed even if it no longer validates
	if err := s.saveVersion(&v, script, notes, "restore", fmt.Sprintf("Khôi phục bản v%d", n), warns); err != nil {
		return err
	}
	if err := s.d.Store.SetStatus(videoID, store.StatusScriptReview, ""); err != nil {
		return err
	}
	s.changed(videoID)
	return nil
}
