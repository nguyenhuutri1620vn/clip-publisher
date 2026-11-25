// Package scriptmodel is a typed, edit-preserving view of history-shorts script.json files.
package scriptmodel

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Slide is the part of a slide the app shows and edits; Raw keeps everything else.
type Slide struct {
	ID           string
	Title        string
	Subtitle     string
	Narration    string
	Claim        bool
	SceneSummary string
	Raw          json.RawMessage
}

// Script is a parsed script.json.
type Script struct {
	Title   string
	Sources []string
	Slides  []Slide
	top     *object
	slides  []*object
}

type slideFields struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Subtitle  string `json:"subtitle"`
	Narration string `json:"narration"`
	Claim     bool   `json:"claim"`
	Scene     *struct {
		Template string `json:"template"`
		Layers   []struct {
			Type string `json:"type"`
		} `json:"layers"`
	} `json:"scene"`
}

// Parse reads a script.json.
func Parse(data []byte) (*Script, error) {
	top, err := parseObject(data)
	if err != nil {
		return nil, fmt.Errorf("script.json không hợp lệ: %w", err)
	}
	s := &Script{top: top}
	if raw, ok := top.vals["title"]; ok {
		_ = json.Unmarshal(raw, &s.Title)
	}
	if raw, ok := top.vals["sources"]; ok {
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, fmt.Errorf("sources: %w", err)
		}
		for _, it := range items {
			var str string
			if json.Unmarshal(it, &str) == nil {
				s.Sources = append(s.Sources, str)
				continue
			}
			var obj struct {
				Text string `json:"text"`
			}
			_ = json.Unmarshal(it, &obj)
			s.Sources = append(s.Sources, obj.Text)
		}
	}
	var rawSlides []json.RawMessage
	if err := json.Unmarshal(top.vals["slides"], &rawSlides); err != nil {
		return nil, fmt.Errorf("slides: %w", err)
	}
	for i, rs := range rawSlides {
		obj, err := parseObject(rs)
		if err != nil {
			return nil, fmt.Errorf("slides[%d]: %w", i, err)
		}
		var f slideFields
		if err := json.Unmarshal(rs, &f); err != nil {
			return nil, fmt.Errorf("slides[%d]: %w", i, err)
		}
		s.slides = append(s.slides, obj)
		s.Slides = append(s.Slides, Slide{
			ID: f.ID, Title: f.Title, Subtitle: f.Subtitle, Narration: f.Narration, Claim: f.Claim,
			SceneSummary: sceneSummary(f), Raw: rs,
		})
	}
	return s, nil
}

func sceneSummary(f slideFields) string {
	if f.Scene == nil {
		return ""
	}
	if f.Scene.Template != "" {
		return "template: " + f.Scene.Template
	}
	types := make([]string, 0, len(f.Scene.Layers))
	for _, l := range f.Scene.Layers {
		types = append(types, l.Type)
	}
	if len(types) == 0 {
		return ""
	}
	return "layers: " + strings.Join(types, ", ")
}

func (s *Script) index(id string) (int, error) {
	for i, sl := range s.Slides {
		if sl.ID == id {
			return i, nil
		}
	}
	return -1, fmt.Errorf("không có cảnh %q", id)
}

func (s *Script) render(slides []*object) ([]byte, error) {
	return s.renderWith(s.top, slides)
}

// renderWith serialises base with its "slides" replaced by slides.
func (s *Script) renderWith(base *object, slides []*object) ([]byte, error) {
	list := make([]json.RawMessage, len(slides))
	for i, o := range slides {
		b, err := o.MarshalJSON()
		if err != nil {
			return nil, err
		}
		list[i] = b
	}
	top := copyObject(base)
	if err := top.set("slides", list); err != nil {
		return nil, err
	}
	b, err := top.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return indent(b)
}

func copySlides(in []*object) []*object {
	out := make([]*object, len(in))
	for i, o := range in {
		c := &object{keys: append([]string(nil), o.keys...), vals: map[string]json.RawMessage{}}
		for k, v := range o.vals {
			c.vals[k] = v
		}
		out[i] = c
	}
	return out
}

// WithSlideText returns the full script JSON with one slide's title, subtitle and narration replaced.
func (s *Script) WithSlideText(id, title, subtitle, narration string) ([]byte, error) {
	i, err := s.index(id)
	if err != nil {
		return nil, err
	}
	slides := copySlides(s.slides)
	for k, v := range map[string]string{"title": title, "subtitle": subtitle, "narration": narration} {
		_, exists := slides[i].vals[k]
		if v == "" && !exists {
			continue
		}
		if err := slides[i].set(k, v); err != nil {
			return nil, err
		}
	}
	return s.render(slides)
}

func copyObject(o *object) *object {
	c := &object{keys: append([]string(nil), o.keys...), vals: map[string]json.RawMessage{}}
	for k, v := range o.vals {
		c.vals[k] = v
	}
	return c
}

// WithSlideSource appends credit to the top-level sources (once) and, if the slide has no
// source yet, marks it as a claim citing that entry. Existing sources and other slides are kept.
func (s *Script) WithSlideSource(id, credit string) ([]byte, error) {
	i, err := s.index(id)
	if err != nil {
		return nil, err
	}
	var items []json.RawMessage
	if raw, ok := s.top.vals["sources"]; ok {
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, fmt.Errorf("sources: %w", err)
		}
	}
	idx := -1
	for k, src := range s.Sources {
		if src == credit {
			idx = k
			break
		}
	}
	if idx < 0 {
		b, err := json.Marshal(credit)
		if err != nil {
			return nil, err
		}
		items = append(items, b)
		idx = len(items) - 1
	}
	top := copyObject(s.top)
	if err := top.set("sources", items); err != nil {
		return nil, err
	}
	slides := copySlides(s.slides)
	if _, has := slides[i].vals["source"]; !has {
		if err := slides[i].set("claim", true); err != nil {
			return nil, err
		}
		if err := slides[i].set("source", idx); err != nil {
			return nil, err
		}
	}
	return s.renderWith(top, slides)
}

// WithSlideRaw returns the full script JSON with one slide object replaced by raw.
func (s *Script) WithSlideRaw(id string, raw []byte) ([]byte, error) {
	i, err := s.index(id)
	if err != nil {
		return nil, err
	}
	obj, err := parseObject(bytes.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("JSON của cảnh phải là một object: %w", err)
	}
	var f struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &f)
	if f.ID != id {
		return nil, errors.New("không được đổi id của cảnh")
	}
	slides := copySlides(s.slides)
	slides[i] = obj
	return s.render(slides)
}

// NeedsCheck reports whether text carries an unverified-fact marker such as
// "[CẦN KIỂM TRA: …]" or "[CẦN BỔ SUNG NGUỒN: …]" (any case).
func NeedsCheck(text string) bool {
	return checkMarker.MatchString(text)
}

var checkMarker = regexp.MustCompile(`(?i)\[\s*cần (kiểm tra|bổ sung)`)
