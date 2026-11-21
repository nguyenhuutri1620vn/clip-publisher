package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"clip-publisher/internal/store"
)

func lookOK(string) (string, error)   { return "/opt/n/bin/claude", nil }
func lookFail(string) (string, error) { return "", errors.New("nope") }

func TestDefaults(t *testing.T) {
	got := Defaults("/Users/x", lookOK)
	want := Settings{
		VideoRepo:    "/Users/x/Documents/Code/make-video-free",
		VoiceRepo:    "/Users/x/Documents/Code/make-voice-free",
		VideoPython:  "/Users/x/Documents/Code/make-video-free/.venv/bin/python",
		VoiceBin:     "/Users/x/Documents/Code/make-voice-free/.venv/bin/make-voice-free",
		ClaudeBin:    "/opt/n/bin/claude",
		Workspace:    "/Users/x/ClipPublisher",
		DefaultVoice: "narrator_mid",
		DefaultMusic: "",
	}
	if got != want {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if Defaults("/h", lookFail).ClaudeBin != "claude" {
		t.Fatal("fallback claude")
	}
}

func TestLoadOverlay(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	def := Defaults("/Users/x", lookOK)
	got, err := Load(st, def)
	if err != nil || got != def {
		t.Fatalf("empty load: %+v %v", got, err)
	}
	st.SetSetting("settings", `{"default_voice":"narrator_deep"}`)
	got, _ = Load(st, def)
	if got.DefaultVoice != "narrator_deep" || got.VideoRepo != def.VideoRepo {
		t.Fatalf("overlay: %+v", got)
	}
	got.DefaultMusic = "a.mp3"
	if err := Save(st, got); err != nil {
		t.Fatal(err)
	}
	again, _ := Load(st, def)
	if again != got {
		t.Fatalf("round trip %+v", again)
	}
}

func TestListVoicesMusic(t *testing.T) {
	root := t.TempDir()
	s := Settings{VoiceRepo: filepath.Join(root, "voice"), VideoRepo: filepath.Join(root, "video")}
	os.MkdirAll(filepath.Join(s.VoiceRepo, "voices"), 0o755)
	os.MkdirAll(filepath.Join(s.VideoRepo, "music"), 0o755)
	for _, f := range []string{"b.json", "a.json", "x.wav"} {
		os.WriteFile(filepath.Join(s.VoiceRepo, "voices", f), nil, 0o644)
	}
	for _, f := range []string{"m.mp3", "n.txt", "k.flac"} {
		os.WriteFile(filepath.Join(s.VideoRepo, "music", f), nil, 0o644)
	}
	v, err := ListVoices(s)
	if err != nil || !reflect.DeepEqual(v, []string{"a", "b"}) {
		t.Fatalf("voices %v %v", v, err)
	}
	m, err := ListMusic(s)
	if err != nil || !reflect.DeepEqual(m, []string{"k.flac", "m.mp3"}) {
		t.Fatalf("music %v %v", m, err)
	}
	if VoiceProfilePath(s, "a") != filepath.Join(s.VoiceRepo, "voices", "a.json") {
		t.Fatal("profile path")
	}
	if MusicPath(s, "") != "" || MusicPath(s, "m.mp3") != filepath.Join(s.VideoRepo, "music", "m.mp3") {
		t.Fatal("music path")
	}
}

// Review #4: Finder's PATH has no nvm/~/.local, so Defaults must probe the usual install dirs.
func TestFindClaudeProbesUserDirs(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, ".nvm", "versions", "node", "v23.10.0", "bin")
	os.MkdirAll(bin, 0o755)
	os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\n"), 0o755)
	if got := FindClaude(home, lookFail); got != filepath.Join(bin, "claude") {
		t.Fatalf("nvm: %q", got)
	}
	local := filepath.Join(home, ".local", "bin")
	os.MkdirAll(local, 0o755)
	os.WriteFile(filepath.Join(local, "claude"), []byte("#!/bin/sh\n"), 0o755)
	if got := FindClaude(home, lookFail); got != filepath.Join(local, "claude") {
		t.Fatalf("local: %q", got)
	}
	if got := FindClaude(home, lookOK); got != "/opt/n/bin/claude" {
		t.Fatalf("lookPath first: %q", got)
	}
	if Defaults(home, lookFail).ClaudeBin != filepath.Join(local, "claude") {
		t.Fatal("Defaults does not use FindClaude")
	}
}
