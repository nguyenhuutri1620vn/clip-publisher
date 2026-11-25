package scriptgen

import (
	"encoding/json"
	"strings"

	"clip-publisher/internal/scriptmodel"
	"clip-publisher/internal/workspace"
)

// Request is what Claude is asked to do: create from Input, or edit Current per Instruction,
// optionally fixing validator Errors.
type Request struct {
	Input       string
	Current     []byte
	Notes       *scriptmodel.Notes
	Chat        []workspace.ChatMessage
	Instruction string
	Errors      []string
}

const maxChat = 10

const rules = `You write script.json files for "history-shorts", a CLI that renders 1080x1920 Vietnamese-history
short videos in a lacquer-painting style. The owner writes scenes in Vietnamese like:
  CẢNH n / Thời gian / Hình ảnh / Chữ trên hình / Lời đọc

Reply with ONE JSON object and nothing else (no prose, no code fences):
{"script": <the complete script.json>, "notes": {"slides": {"<slide id>": {"note": "...", "time_hint": "..."}}}, "summary": "<one short Vietnamese sentence describing what you did or changed>"}

Rules:
- "schema_version": "2.4". Follow the CONTRACT exactly; use only layer types, templates, roles and fields it defines.
- Characters: match each person's sex. Women use the ancient_female roles (leader, general, general_alt, boat_crew).
  Men use the warrior_male roles: viet_general / viet_soldier for Vietnamese men, northern_general / northern_soldier
  for northern (Chinese) invaders. Never draw a man with a female role. Each kit accepts only its own props.
- "Chữ trên hình" -> slide "title" (short) and "subtitle"; "Lời đọc" -> "narration" (Vietnamese, max 600 chars).
- "Hình ảnh" -> the closest "scene" built from the CONTRACT's templates/layers, plus a fitting "motion" and "transition".
- "Thời gian" goes ONLY into notes.slides.<id>.time_hint (verbatim, e.g. "0:04 – 0:12"); never into the script. Slide length comes from the voice.
- Each slide "id" is a Vietnamese slug without diacritics matching ^[a-z0-9][a-z0-9_-]{0,31}$, unique.
- A narration must be speakable in 0.3–14 seconds (about 40 syllables max); a slide lasts at most 15 s. Split long scenes.
- Factual statements get "claim": true and "source" (index into "sources"). Fill "sources" with real citations you are confident exist.
  Append "[CẦN KIỂM TRA: <what to verify>]" to every source or detail (numbers, years, pages) you are not certain of.
  Never remove an existing "[CẦN KIỂM TRA" marker or "claim" flag.
- When "Hình ảnh" asks for something the renderer cannot draw (a real photo, a specific artifact, a person's likeness),
  use the closest layers and say so in notes.slides.<id>.note, including the words "cần ảnh thật" when a real photo is wanted.
- Write notes and summary in Vietnamese.`

const editRules = `You are editing CURRENT SCRIPT. Apply the INSTRUCTION; keep every slide the instruction does not concern
byte-identical (same id, same fields). Keep existing notes for unchanged slides. Return the complete script.`

const fixRules = `The previous script failed validation with the VALIDATOR ERRORS below. Fix exactly these errors, change nothing else.`

// BuildPrompt assembles the full prompt sent on stdin.
func BuildPrompt(contract string, examples []string, r Request) string {
	var b strings.Builder
	w := func(title, body string) {
		b.WriteString("\n\n===== " + title + " =====\n")
		b.WriteString(strings.TrimSpace(body))
	}
	b.WriteString(rules)
	if r.Current != nil {
		b.WriteString("\n\n" + editRules)
	}
	if len(r.Errors) > 0 {
		b.WriteString("\n\n" + fixRules)
	}
	w("CONTRACT", contract)
	for _, ex := range examples {
		w("EXAMPLE script.json", ex)
	}
	if r.Current != nil {
		w("CURRENT SCRIPT", string(r.Current))
		if r.Notes != nil {
			nb, _ := json.Marshal(r.Notes)
			w("CURRENT NOTES", string(nb))
		}
	}
	if len(r.Chat) > 0 {
		chat := r.Chat
		if len(chat) > maxChat {
			chat = chat[len(chat)-maxChat:]
		}
		var cb strings.Builder
		for _, m := range chat {
			cb.WriteString(m.Role + ": " + m.Text + "\n")
		}
		w("CHAT HISTORY", cb.String())
	}
	if len(r.Errors) > 0 {
		w("VALIDATOR ERRORS", strings.Join(r.Errors, "\n"))
	}
	if r.Input != "" {
		w("INPUT", r.Input)
	}
	if r.Instruction != "" {
		w("INSTRUCTION", r.Instruction)
	}
	b.WriteString("\n")
	return b.String()
}
