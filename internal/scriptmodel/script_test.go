package scriptmodel

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

func load(t *testing.T) (*Script, []byte) {
	t.Helper()
	b, err := os.ReadFile("testdata/sample.json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return s, b
}

func jsonEqual(a, b []byte) bool {
	var x, y any
	json.Unmarshal(a, &x)
	json.Unmarshal(b, &y)
	return reflect.DeepEqual(x, y)
}

func TestParseSample(t *testing.T) {
	s, _ := load(t)
	if s.Title != "Nỏ thần Cổ Loa" || len(s.Slides) != 3 || len(s.Sources) != 2 {
		t.Fatalf("%+v", s)
	}
	if s.Sources[1] != "Báo Nhân Dân [CẦN KIỂM TRA: số kg]" {
		t.Fatalf("source obj: %q", s.Sources[1])
	}
	want := []string{"template: group", "layers: sun, clouds, elephant", ""}
	for i, w := range want {
		if s.Slides[i].SceneSummary != w {
			t.Errorf("slide %d summary %q", i, s.Slides[i].SceneSummary)
		}
	}
	if !s.Slides[1].Claim || s.Slides[0].Claim || s.Slides[1].ID != "rua-vang" {
		t.Fatal("claim/id")
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	if _, err := Parse([]byte(`{"slides": 3}`)); err == nil {
		t.Fatal("expected error")
	}
	if _, err := Parse([]byte(`nope`)); err == nil {
		t.Fatal("expected error")
	}
}

func TestWithSlideTextPreservesOthers(t *testing.T) {
	s, _ := load(t)
	out, err := s.WithSlideText("rua-vang", "Thế kỷ 3 TCN", "Thành Cổ Loa", "Lời mới.")
	if err != nil {
		t.Fatal(err)
	}
	s2, err := Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	if s2.Slides[1].Narration != "Lời mới." {
		t.Fatal("narration not changed")
	}
	if !jsonEqual(s.Slides[0].Raw, s2.Slides[0].Raw) || !jsonEqual(s.Slides[2].Raw, s2.Slides[2].Raw) {
		t.Fatal("other slides changed")
	}
	var a, b map[string]json.RawMessage
	json.Unmarshal(s.Slides[1].Raw, &a)
	json.Unmarshal(s2.Slides[1].Raw, &b)
	if !jsonEqual(a["scene"], b["scene"]) || !jsonEqual(a["sfx"], b["sfx"]) || !jsonEqual(a["source"], b["source"]) {
		t.Fatal("other fields of slide changed")
	}
	if s2.Title != s.Title || len(s2.Sources) != 2 {
		t.Fatal("top-level changed")
	}
	if !bytes.Contains(out, []byte("\n  \"")) {
		t.Fatal("not indented")
	}
	if _, err := s.WithSlideText("missing", "", "", ""); err == nil {
		t.Fatal("unknown id accepted")
	}
}

func TestWithSlideRaw(t *testing.T) {
	s, _ := load(t)
	out, err := s.WithSlideRaw("cau-vuc", []byte(`{"id":"cau-vuc","title":"X","narration":"Y"}`))
	if err != nil {
		t.Fatal(err)
	}
	s2, _ := Parse(out)
	if s2.Slides[2].Title != "X" || s2.Slides[2].Subtitle != "" {
		t.Fatalf("%+v", s2.Slides[2])
	}
	if _, err := s.WithSlideRaw("cau-vuc", []byte(`{"id":"other"}`)); err == nil {
		t.Fatal("id change accepted")
	}
	if _, err := s.WithSlideRaw("cau-vuc", []byte(`[1]`)); err == nil {
		t.Fatal("non-object accepted")
	}
}

func TestNeedsCheck(t *testing.T) {
	if !NeedsCheck("x [CẦN KIỂM TRA: năm]") || NeedsCheck("đã kiểm tra") ||
		!NeedsCheck("[CẦN BỔ SUNG NGUỒN: báo cáo khảo cổ]") || !NeedsCheck("Đại Việt sử ký [cần kiểm tra số trang]") {
		t.Fatal("NeedsCheck")
	}
}

func TestParseTimeHint(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"0:00 – 0:04", 4, true},
		{"0:04-0:12", 8, true},
		{"1:00 — 1:30", 30, true},
		{"4s", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := ParseTimeHint(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("%q → %v %v", c.in, got, ok)
		}
	}
}

func TestEstimate(t *testing.T) {
	got := EstimateNarrationSeconds("một hai ba [pause 400ms] bốn")
	if math.Abs(got-(4.0/4.5+0.4)) > 1e-9 { // 4.5 syllables/s measured on 15 real narrator_mid voices
		t.Fatalf("got %v", got)
	}
}

func TestWarnings(t *testing.T) {
	long := strings.Repeat("chữ ", 62)
	s := &Script{Slides: []Slide{{ID: "a", Narration: long}}}
	w := ScriptWarnings(s)
	if len(w) == 0 || w[0].SlideID != "a" || !strings.Contains(w[0].Message, "14s") {
		t.Fatalf("%+v", w)
	}
	tiny := &Script{Slides: []Slide{{ID: "a", Narration: "một"}, {ID: "b", Narration: "hai"}}}
	w = ScriptWarnings(tiny)
	if len(w) != 1 || w[0].SlideID != "" || !strings.Contains(w[0].Message, "15–90s") {
		t.Fatalf("%+v", w)
	}
}

func TestParseNotes(t *testing.T) {
	n, err := ParseNotes(nil)
	if err != nil || len(n.Slides) != 0 {
		t.Fatal("empty notes")
	}
	n, err = ParseNotes([]byte(`{"slides":{"hook":{"note":"cần ảnh thật","time_hint":"0:00 – 0:04"}}}`))
	if err != nil || n.Slides["hook"].Note != "cần ảnh thật" || n.Slides["hook"].TimeHint != "0:00 – 0:04" {
		t.Fatalf("%+v %v", n, err)
	}
}

func TestDiff(t *testing.T) {
	a, _ := load(t)
	b := &Script{Slides: []Slide{
		{ID: "hook", Title: "MỚI", Subtitle: a.Slides[0].Subtitle, Narration: a.Slides[0].Narration, Raw: a.Slides[0].Raw},
		a.Slides[1],
		{ID: "moi", Title: "x", Raw: json.RawMessage(`{"id":"moi"}`)},
	}}
	b.Slides[0].Raw = json.RawMessage(strings.Replace(string(a.Slides[0].Raw), "TRUYỀN THUYẾT NỎ THẦN", "MỚI", 1))
	got := Diff(a, b)
	want := []SlideChange{
		{ID: "hook", Kind: "changed", Fields: []string{"title"}},
		{ID: "moi", Kind: "added"},
		{ID: "cau-vuc", Kind: "removed"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestWithSlideSourceAddsCreditOnce(t *testing.T) {
	s, _ := load(t)
	credit := "Ảnh: Tác giả X, CC BY-SA 4.0, https://commons.wikimedia.org/wiki/File:A.jpg"
	out, err := s.WithSlideSource("hook", credit)
	if err != nil {
		t.Fatal(err)
	}
	s2, err := Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.Sources) != len(s.Sources)+1 || s2.Sources[len(s2.Sources)-1] != credit {
		t.Fatalf("sources: want credit appended, got %q", s2.Sources)
	}
	if !s2.Slides[0].Claim {
		t.Fatalf("slide hook: want claim true")
	}
	out2, err := s2.WithSlideSource("hook", credit)
	if err != nil {
		t.Fatal(err)
	}
	s3, _ := Parse(out2)
	if len(s3.Sources) != len(s2.Sources) {
		t.Fatalf("duplicate credit added: %q", s3.Sources)
	}
}

func TestWithSlideSourceKeepsExistingCitation(t *testing.T) {
	src := []byte(`{"schema_version":"2.2","sources":["Sách cũ"],"slides":[{"id":"a","title":"A","narration":"x","claim":true,"source":0}]}`)
	s, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	credit := "Ảnh: X, CC BY 4.0, https://c.example/F"
	out, err := s.WithSlideSource("a", credit)
	if err != nil {
		t.Fatal(err)
	}
	if got := sourceIndices(t, out); !reflect.DeepEqual(got, []int{0, 1}) {
		t.Fatalf("source = %v, want [0 1] so the scene cites the photo too", got)
	}
	s2, _ := Parse(out)
	out2, err := s2.WithSlideSource("a", credit)
	if err != nil {
		t.Fatal(err)
	}
	if got := sourceIndices(t, out2); !reflect.DeepEqual(got, []int{0, 1}) {
		t.Fatalf("repeat pick changed source to %v", got)
	}
}

// sourceIndices returns slide "a"'s source as a list, whether it was written as an int or an array.
func sourceIndices(t *testing.T, script []byte) []int {
	t.Helper()
	var doc struct {
		Slides []struct {
			Source json.RawMessage `json:"source"`
		} `json:"slides"`
	}
	if err := json.Unmarshal(script, &doc); err != nil {
		t.Fatal(err)
	}
	var many []int
	if err := json.Unmarshal(doc.Slides[0].Source, &many); err == nil {
		return many
	}
	var one int
	if err := json.Unmarshal(doc.Slides[0].Source, &one); err != nil {
		t.Fatal(err)
	}
	return []int{one}
}
