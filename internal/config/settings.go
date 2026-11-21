// Package config holds the app settings: where the sibling projects and tools live.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"clip-publisher/internal/store"
)

// Settings are the user-editable paths and defaults (PRD F1).
type Settings struct {
	VideoRepo    string `json:"video_repo"`
	VoiceRepo    string `json:"voice_repo"`
	VideoPython  string `json:"video_python"`
	VoiceBin     string `json:"voice_bin"`
	ClaudeBin    string `json:"claude_bin"`
	Workspace    string `json:"workspace"`
	DefaultVoice string `json:"default_voice"`
	DefaultMusic string `json:"default_music"`
}

const settingsKey = "settings"

// Defaults returns the settings for a home directory.
func Defaults(home string, lookPath func(string) (string, error)) Settings {
	code := filepath.Join(home, "Documents", "Code")
	video := filepath.Join(code, "make-video-free")
	voice := filepath.Join(code, "make-voice-free")
	claude := "claude"
	if p, err := lookPath("claude"); err == nil {
		claude = p
	}
	return Settings{
		VideoRepo:    video,
		VoiceRepo:    voice,
		VideoPython:  filepath.Join(video, ".venv", "bin", "python"),
		VoiceBin:     filepath.Join(voice, ".venv", "bin", "make-voice-free"),
		ClaudeBin:    claude,
		Workspace:    filepath.Join(home, "ClipPublisher"),
		DefaultVoice: "narrator_mid",
	}
}

// Load overlays the saved non-empty fields on defaults. DefaultMusic may be saved empty on purpose,
// so it is taken from the saved value whenever settings were saved.
func Load(st *store.Store, defaults Settings) (Settings, error) {
	raw, ok, err := st.GetSetting(settingsKey)
	if err != nil || !ok {
		return defaults, err
	}
	var saved Settings
	if err := json.Unmarshal([]byte(raw), &saved); err != nil {
		return defaults, err
	}
	out := defaults
	pick := func(dst *string, v string) {
		if v != "" {
			*dst = v
		}
	}
	pick(&out.VideoRepo, saved.VideoRepo)
	pick(&out.VoiceRepo, saved.VoiceRepo)
	pick(&out.VideoPython, saved.VideoPython)
	pick(&out.VoiceBin, saved.VoiceBin)
	pick(&out.ClaudeBin, saved.ClaudeBin)
	pick(&out.Workspace, saved.Workspace)
	pick(&out.DefaultVoice, saved.DefaultVoice)
	out.DefaultMusic = saved.DefaultMusic
	return out, nil
}

// Save stores the settings.
func Save(st *store.Store, s Settings) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return st.SetSetting(settingsKey, string(b))
}

func listFiles(dir string, keep func(name string) (string, bool)) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if name, ok := keep(e.Name()); ok {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
}

// ListVoices returns the voice profile names in make-voice-free/voices.
func ListVoices(s Settings) ([]string, error) {
	return listFiles(filepath.Join(s.VoiceRepo, "voices"), func(n string) (string, bool) {
		return strings.TrimSuffix(n, ".json"), strings.HasSuffix(n, ".json")
	})
}

// VoiceProfilePath is the JSON profile file of a voice name.
func VoiceProfilePath(s Settings, name string) string {
	return filepath.Join(s.VoiceRepo, "voices", name+".json")
}

var musicExt = map[string]bool{".mp3": true, ".wav": true, ".m4a": true, ".flac": true}

// ListMusic returns the music file names in make-video-free/music.
func ListMusic(s Settings) ([]string, error) {
	return listFiles(filepath.Join(s.VideoRepo, "music"), func(n string) (string, bool) {
		return n, musicExt[strings.ToLower(filepath.Ext(n))]
	})
}

// MusicPath is the absolute path of a music name ("" means no music).
func MusicPath(s Settings, name string) string {
	if name == "" {
		return ""
	}
	return filepath.Join(s.VideoRepo, "music", name)
}
