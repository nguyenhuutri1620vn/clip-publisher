package scriptmodel

import (
	"bytes"
	"encoding/json"
	"reflect"
)

// SlideChange describes how a slide differs between two versions.
type SlideChange struct {
	ID     string   `json:"id"`
	Kind   string   `json:"kind"` // "added" | "removed" | "changed"
	Fields []string `json:"fields,omitempty"`
}

var namedFields = []string{"title", "subtitle", "narration", "scene"}

func fieldsOf(raw json.RawMessage) map[string]any {
	m := map[string]any{}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	_ = dec.Decode(&m)
	return m
}

// Diff lists changed/added slides in b's order, then removed ones in a's order.
func Diff(a, b *Script) []SlideChange {
	old := map[string]Slide{}
	for _, s := range a.Slides {
		old[s.ID] = s
	}
	inB := map[string]bool{}
	out := []SlideChange{}
	for _, s := range b.Slides {
		inB[s.ID] = true
		prev, ok := old[s.ID]
		if !ok {
			out = append(out, SlideChange{ID: s.ID, Kind: "added"})
			continue
		}
		fa, fb := fieldsOf(prev.Raw), fieldsOf(s.Raw)
		var fields []string
		for _, f := range namedFields {
			if !reflect.DeepEqual(fa[f], fb[f]) {
				fields = append(fields, f)
			}
			delete(fa, f)
			delete(fb, f)
		}
		if !reflect.DeepEqual(fa, fb) {
			fields = append(fields, "other")
		}
		if len(fields) > 0 {
			out = append(out, SlideChange{ID: s.ID, Kind: "changed", Fields: fields})
		}
	}
	for _, s := range a.Slides {
		if !inB[s.ID] {
			out = append(out, SlideChange{ID: s.ID, Kind: "removed"})
		}
	}
	return out
}
