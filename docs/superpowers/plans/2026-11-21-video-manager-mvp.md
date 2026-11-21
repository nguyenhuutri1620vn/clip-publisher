# clip-publisher MVP — Video Manager Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a macOS Wails desktop app that lists existing/new history-short videos, turns free-form Vietnamese scene text into a validated `script.json` via Claude CLI, renders it through the two sibling Python projects with step-by-step progress, and supports chat/manual edits with versioning.

**Architecture:** Go backend split into small `internal/` packages (workspace files, SQLite store, settings, process runner, pipeline commands + stdout progress, Claude script generator, importer, serial job queues) wired by one `internal/service` orchestrator; `app.go` exposes it to a thin React/TS frontend via Wails bindings + events, and an asset-server `http.Handler` streams local mp4/png files. All external work is `exec` of CLIs (no shell).

**Tech Stack:** Go 1.23, Wails v2.9.2 (`react-ts` template), `modernc.org/sqlite` (no CGO), React 18 + TypeScript (Vite, plain CSS), macOS `osascript` for notifications / Trash.

**Spec:** `docs/prd.md` (requirement IDs F1.x–F6.x, C1–C7). Background: `docs/raw_requirement.md` §0.1. Working rules: `CLAUDE.md`.

## Global Constraints

- Never write, move or delete anything inside `make-video-free` or `make-voice-free` (C2). Read-only: `examples/`, `voices/`, `music/`, `docs/`, `output/`.
- Every external command: `exec.CommandContext(name, args...)`, never `sh -c` (C4).
- Script / notes / render files are never overwritten: new version = new file `script.v{N}.json`; all writes via `workspace.WriteFileAtomic` (temp file + rename).
- One render-type job (render or preview) runs at a time (C6). Claude jobs run serially on their own queue.
- Claude call: `claude -p --output-format json --tools "" --no-session-persistence`, prompt on stdin, timeout **5 min** (F3.7); at most **3** validate-repair rounds (F3.6).
- Script limits copied from make-video-free: voice per slide 0.3–14 s, slide ≤ 15 s, narration ≤ 600 chars, slide `id` matches `^[a-z0-9][a-z0-9_-]{0,31}$`, total 15–90 s is a warning only.
- Never strip `[CẦN KIỂM TRA` markers or `claim` flags from Claude output.
- Defaults: workspace `~/ClipPublisher`, voice `narrator_mid`, music none, timing-mismatch warning threshold **2 s**, voice ETA **30 s/voice**.
- User-facing strings in Vietnamese; code identifiers, comments, commit messages in English.
- New Go modules / npm packages beyond `modernc.org/sqlite` and the Wails template need the owner's OK (CLAUDE.md rule 7).
- Tests never run the real VoiceStudio, renderer or Claude: use fake executables in `testdata/fakes/`.

## Review Focus

1. **App launched from Finder has a minimal `PATH`** (no `/opt/homebrew/bin`, no nvm) → `ffmpeg`, `claude` (a `#!/usr/bin/env node` script) fail. Expect every child process to get `PATH` with `/opt/homebrew/bin`, `/usr/local/bin` and `dir(ClaudeBin)` prepended. Test in Task 4.
2. **Paths with spaces in `--voice-cmd`** (e.g. a workspace or repo under `~/My Videos`): make_video.py `shlex.split`s the template, so unquoted paths break voice recording. Expect POSIX-quoted paths in the template. Test in Task 6.
3. **Claude wraps JSON in prose or ```json fences**, or returns `is_error: true`. Expect the JSON object to be extracted, or a clear Vietnamese error. Test in Task 7.
4. **App quits or crashes mid-render**: orphan python/ffmpeg processes and a job stuck `RUNNING`. Expect cancel/quit to kill the whole process group and the next launch to show `FAILED` "Bị gián đoạn" with Retry. Tests in Tasks 4 and 9.
5. **Edit or chat while the same video is rendering / generating**: expect `ErrBusy` ("Video đang render, hãy đợi hoặc huỷ") and no new version. Test in Task 9.

---

## File Structure

```
main.go                      Wails bootstrap: options, asset handler, OnStartup/OnBeforeClose
app.go                       App struct: bound methods → service; event bridge
internal/workspace/          paths, slug, atomic write, chat log           (Task 1)
internal/store/              SQLite open+migrate, videos/versions/renders/jobs/settings (Task 2)
internal/config/             Settings defaults/load/save, voices/music lists (Task 3)
internal/runner/             exec with process group, line streaming, cancel, ChildEnv (Task 4)
internal/scriptmodel/        typed view of script.json, edits, diff, warnings, notes (Task 5)
internal/pipeline/           validate/preview/render commands, voice-cmd quoting, progress parser, report, Doctor (Task 6)
internal/scriptgen/          contract loader, prompt, Claude envelope parsing, repair loop (Task 7)
internal/library/            scan make-video-free/output, sha-match scripts, thumbnails (Task 8)
internal/jobs/               serial queue with cancel + positions (Task 9)
internal/service/            orchestrator: create/chat/edit/preview/render/retry/cancel/recover/import (Task 9)
internal/media/              http.Handler serving /media/... with Range (Task 10)
internal/notify/             osascript notification + move-to-Trash (Task 10)
frontend/src/                api.ts, App.tsx, components/*.tsx, styles.css (Tasks 11–13)
testdata/fakes/              fake_make_video.sh, fake_claude.sh, fake_validate.sh
```

---

### Task 1: Scaffold + workspace package

**Files:**
- Create: Wails project in repo root (`wails init -n clip-publisher -t react-ts` in a temp dir, then move contents into the repo root, keeping `docs/`, `CLAUDE.md`), `.gitignore` (add `build/bin`, `frontend/node_modules`, `frontend/dist`)
- Create: `internal/workspace/workspace.go`, `internal/workspace/chat.go`
- Test: `internal/workspace/workspace_test.go`
- Modify: `CLAUDE.md` section "Lệnh" (replace the "Chưa scaffold" note with the real commands)

**Interfaces:**
- Produces:
  - `type Workspace struct{ Root string }`; `func New(root string) (*Workspace, error)` (creates `Root/videos`, `Root/logs`, `Root/thumbs`)
  - `func Slugify(s string) string`
  - `func (w *Workspace) NewVideoDir(hint string) (slug, dir string, err error)`
  - `func (w *Workspace) VideoDir(slug string) string`
  - `func ScriptPath(dir string, n int) string` → `dir/script.v{n}.json`; `NotesPath` → `notes.v{n}.json`; `RenderMP4Path(dir, n)` → `renders/v{n}.mp4`; `RenderLogPath(dir, n)` → `renders/render.v{n}.log`; `PreviewMP4Path(dir, n)` → `renders/preview.v{n}/preview.mp4`; `VoiceDir(dir)` → `voice`; `InputPath(dir)` → `input.md`
  - `func WriteFileAtomic(path string, data []byte) error` (creates parent dirs)
  - `type ChatMessage struct{ Role string \`json:"role"\`; Text string \`json:"text"\`; At time.Time \`json:"at"\` }`; `func AppendChat(dir string, m ChatMessage) error`; `func ReadChat(dir string) ([]ChatMessage, error)` (file `chat.jsonl`; missing file → empty slice)

- [ ] **Step 1: Scaffold** — run `wails init` as above, `go mod tidy`, confirm `wails build` produces `build/bin/clip-publisher.app`. Module path: `clip-publisher`.
- [ ] **Step 2: Write failing tests** in `workspace_test.go`:
  - `TestSlugify`: `"Nỏ thần Cổ Loa"`→`"no-than-co-loa"`; `"Đền Thượng — 2004!"`→`"den-thuong-2004"`; `"   "`→`"video"`; 80-char input → length ≤ 48 and no trailing `-`.
  - `TestNewVideoDirUnique`: two calls with `"Cổ Loa"` → slugs `"co-loa"`, `"co-loa-2"`; both dirs exist.
  - `TestWriteFileAtomicNoPartial`: write to `a/b/c.json`; content equals; no `*.tmp` left in dir.
  - `TestChatRoundTrip`: append 2 messages, `ReadChat` returns them in order; `ReadChat` on empty dir → `len==0, err==nil`.
- [ ] **Step 3: Run** `go test ./internal/workspace/...` → FAIL (undefined).
- [ ] **Step 4: Implement.** Slugify: `golang.org/x/text/unicode/norm` is NOT allowed (new dep) — strip diacritics with NFD-free approach: map `đ/Đ`→`d`, then use a hand table of Vietnamese vowels with marks → base vowel (a, ă, â, e, ê, i, o, ô, ơ, u, ư, y in all 5 tones + upper case), lowercase, non `[a-z0-9]` → `-`, collapse/trim `-`, cut to 48.
- [ ] **Step 5: Run** `go test ./internal/workspace/...` → PASS.
- [ ] **Step 6: Commit** `feat: scaffold Wails app and workspace package`.

### Task 2: SQLite store

**Files:**
- Create: `internal/store/store.go`, `internal/store/schema.sql` (embedded), `internal/store/videos.go`, `internal/store/versions.go`, `internal/store/renders.go`, `internal/store/jobs.go`, `internal/store/settings.go`
- Test: `internal/store/store_test.go`

**Interfaces:**
- Produces (all methods on `*Store`, errors wrapped; `ErrNotFound` sentinel):
  - `func Open(path string) (*Store, error)`; `func (s *Store) Close() error` — migrations via `PRAGMA user_version`; `PRAGMA journal_mode=WAL; foreign_keys=ON`.
  - `type Status string` consts `StatusDraft="DRAFT"`, `StatusGeneratingScript="GENERATING_SCRIPT"`, `StatusScriptReview="SCRIPT_REVIEW"`, `StatusRendering="RENDERING"`, `StatusReady="READY"`, `StatusFailed="FAILED"`.
  - `type Video struct{ ID int64; Slug, Title, Origin, SourcePath string; Status Status; StatusDetail, VoiceProfile, Music string; CurrentVersion int; Editable, Hidden bool; CreatedAt, UpdatedAt time.Time }` (Origin `"app"|"import"`; `SourcePath` = original mp4 for imports)
  - `CreateVideo(v *Video) error` (sets ID, timestamps); `GetVideo(id int64) (Video, error)`; `ListVideos(includeHidden bool) ([]Video, error)` ordered `updated_at DESC`; `UpdateVideo(v Video) error` (bumps `updated_at`); `SetStatus(id int64, st Status, detail string) error`; `DeleteVideo(id int64) error` (cascades); `FindVideoBySource(path string) (Video, error)`
  - `type Version struct{ VideoID int64; N int; ScriptPath, NotesPath, Author, Summary string; CreatedAt time.Time }` (Author `"claude"|"manual"|"import"|"restore"`); `AddVersion(v Version) error`; `ListVersions(videoID int64) ([]Version, error)` ascending N; `GetVersion(videoID int64, n int) (Version, error)`
  - `type Render struct{ ID, VideoID int64; Version int; Kind, MP4Path, ReportPath, LogPath, State, Error string; Seconds float64; StartedAt time.Time; FinishedAt *time.Time }` (Kind `"render"|"preview"`, State `"RUNNING"|"DONE"|"FAILED"|"CANCELED"`); `AddRender(r *Render) error`; `FinishRender(id int64, state, errMsg string, seconds float64) error`; `ListRenders(videoID int64) ([]Render, error)` newest first; `LatestRender(videoID int64, kind string) (Render, error)`
  - `type Job struct{ ID, VideoID int64; Kind, State, Error string; CreatedAt time.Time }` (Kind `"script"|"chat"|"render"|"preview"`; State `"QUEUED"|"RUNNING"|"DONE"|"FAILED"|"CANCELED"`); `AddJob(j *Job) error`; `SetJobState(id int64, state, errMsg string) error`; `LastJob(videoID int64) (Job, error)`; `MarkInterrupted() ([]Job, error)` — every `QUEUED`/`RUNNING` job → `FAILED` with error `"interrupted"`, returns them; also every `RUNNING` render → `FAILED`.
  - `GetSetting(key string) (string, bool, error)`; `SetSetting(key, value string) error`

- [ ] **Step 1: Write failing tests** (temp-dir DB):
  - `TestVideoCRUD`: create → get equals; `ListVideos(false)` excludes hidden; order by `UpdatedAt` desc after `SetStatus` on the older one.
  - `TestVersionsOrdered`: add N=2 then N=1 → list returns 1,2; duplicate (video, N) → error.
  - `TestMarkInterrupted`: jobs RUNNING, QUEUED, DONE → returns 2; their state `FAILED`, error `"interrupted"`; DONE untouched; RUNNING render → FAILED.
  - `TestDeleteCascades`: delete video → versions/renders/jobs gone.
  - `TestReopenKeepsData`: close, `Open` again, data present (migration idempotent).
- [ ] **Step 2: Run** `go test ./internal/store/...` → FAIL.
- [ ] **Step 3: Implement** with `database/sql` + `modernc.org/sqlite` (driver name `"sqlite"`). Schema mirrors the structs (PRD §7); `UNIQUE(video_id, n)` on versions; `ON DELETE CASCADE`.
- [ ] **Step 4: Run** → PASS. **Step 5: Commit** `feat: add SQLite store`.

### Task 3: Settings, voice/music lists

**Files:**
- Create: `internal/config/settings.go`
- Test: `internal/config/settings_test.go`

**Interfaces:**
- Consumes: `store.Store.GetSetting/SetSetting`.
- Produces:
  - `type Settings struct{ VideoRepo, VoiceRepo, VideoPython, VoiceBin, ClaudeBin, Workspace, DefaultVoice, DefaultMusic string }` (json tags snake_case)
  - `func Defaults(home string, lookPath func(string) (string, error)) Settings` — `VideoRepo=home/Documents/Code/make-video-free`, `VoiceRepo=home/Documents/Code/make-voice-free`, `VideoPython=VideoRepo/.venv/bin/python`, `VoiceBin=VoiceRepo/.venv/bin/make-voice-free`, `ClaudeBin=lookPath("claude")` else `"claude"`, `Workspace=home/ClipPublisher`, `DefaultVoice="narrator_mid"`, `DefaultMusic=""`
  - `func Load(st *store.Store, defaults Settings) (Settings, error)` — key `"settings"` JSON overlays non-empty fields on defaults; `func Save(st *store.Store, s Settings) error`
  - `func ListVoices(s Settings) ([]string, error)` — basenames without `.json` of `VoiceRepo/voices/*.json`, sorted
  - `func VoiceProfilePath(s Settings, name string) string`
  - `func ListMusic(s Settings) ([]string, error)` — file names in `VideoRepo/music` with ext `.mp3 .wav .m4a .flac`, sorted
  - `func MusicPath(s Settings, name string) string` (`""` → `""`)

- [ ] **Step 1: Failing tests:** `TestDefaults` (exact paths above with `home="/Users/x"`, lookPath stub returning `/opt/n/bin/claude`); `TestLoadOverlay` (saved `{"default_voice":"narrator_deep"}` → DefaultVoice deep, others default); `TestListVoicesMusic` (temp dirs with `a.json b.json x.wav`, `m.mp3 n.txt` → `[a b]`, `[m.mp3]`).
- [ ] **Step 2: Run** `go test ./internal/config/...` → FAIL. **Step 3: Implement.** **Step 4:** PASS. **Step 5: Commit** `feat: add settings`.

### Task 4: Process runner

**Files:**
- Create: `internal/runner/runner.go`, `internal/runner/env.go`
- Create: `testdata/fakes/sleeper.sh` (prints `start`, spawns `sleep 30 &` child, waits)
- Test: `internal/runner/runner_test.go`

**Interfaces:**
- Produces:
  - `type Cmd struct{ Name string; Args []string; Dir string; Env []string; Stdin string; Timeout time.Duration }`
  - `type Result struct{ ExitCode int; Output string }` (stdout+stderr interleaved, full text)
  - `type Runner interface{ Run(ctx context.Context, c Cmd, onLine func(string)) (Result, error) }`; `type ExecRunner struct{}`
  - `var ErrTimeout = errors.New("timeout")`, `var ErrCanceled = errors.New("canceled")`
  - `func ChildEnv(base []string, extraDirs ...string) []string` — returns `base` with `PATH` = `extraDirs…` + `/opt/homebrew/bin` + `/usr/local/bin` + original PATH entries, de-duplicated, order kept; adds `PATH` if absent; sets `PYTHONUNBUFFERED=1` and `LANG=en_US.UTF-8` if absent.

- [ ] **Step 1: Failing tests:**
  - `TestRunStreamsLines`: fake `testdata/fakes/lines.sh` (stdout `a`,`b`, stderr `c`, exit 3) → onLine got `a,b,c` (any order between streams), `ExitCode==3`, `err==nil`.
  - `TestStdin`: `/bin/cat` with Stdin `"xin chào"` → Output `"xin chào"`.
  - `TestCancelKillsGroup`: run `sleeper.sh`, cancel ctx after `start` line → returns `ErrCanceled` within 5 s; `pgrep -f "sleep 30"` finds no process started by the test (record child pid via script output `child=<pid>` and assert `syscall.Kill(pid,0)` fails).
  - `TestTimeout`: `Timeout: 200ms` on sleeper → `ErrTimeout`.
  - `TestChildEnvPath` (Review Focus #1): base `PATH=/usr/bin:/bin`, extra `/Users/x/.nvm/v/bin` → `PATH=/Users/x/.nvm/v/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin`; base without PATH → PATH still set; duplicate `/usr/local/bin` in base appears once.
- [ ] **Step 2: Run** `go test ./internal/runner/...` → FAIL.
- [ ] **Step 3: Implement** with `SysProcAttr{Setpgid: true}`; on cancel/timeout `syscall.Kill(-pgid, SIGTERM)`, after 3 s `SIGKILL`; read both pipes with `bufio.Scanner` (buffer 1 MiB) in goroutines, serialize `onLine` calls with a mutex.
- [ ] **Step 4: Run** → PASS. **Step 5: Commit** `feat: add process runner with group cancel`.

### Task 5: Script model, notes, warnings, diff

**Files:**
- Create: `internal/scriptmodel/script.go`, `internal/scriptmodel/notes.go`, `internal/scriptmodel/warnings.go`, `internal/scriptmodel/diff.go`
- Test: `internal/scriptmodel/*_test.go`, fixture `internal/scriptmodel/testdata/sample.json` (3 slides: one with `scene.template`, one with `scene.layers`, one with `[pause 400ms]` and a `[CẦN KIỂM TRA]` source)

**Interfaces:**
- Produces:
  - `type Slide struct{ ID, Title, Subtitle, Narration string; Claim bool; SceneSummary string; Raw json.RawMessage }` — `SceneSummary`: `"template: group"` or `"layers: sun, clouds, elephant"` or `""`
  - `type Script struct{ Title string; Sources []string; Slides []Slide; raw map[string]json.RawMessage }` (Sources: string items or `.text` of object items)
  - `func Parse(data []byte) (*Script, error)`
  - `func (s *Script) WithSlideText(id, title, subtitle, narration string) ([]byte, error)` — returns full JSON (indent 2) with only those 3 fields changed, all other keys/slides byte-equivalent after re-marshal
  - `func (s *Script) WithSlideRaw(id string, raw []byte) ([]byte, error)` — replaces one slide object; error if `raw` is not a JSON object or its `id` differs
  - `func NeedsCheck(text string) bool` — contains `"[CẦN KIỂM TRA"`
  - `type SlideNote struct{ Note string \`json:"note"\`; TimeHint string \`json:"time_hint"\` }`; `type Notes struct{ Slides map[string]SlideNote \`json:"slides"\` }`; `func ParseNotes(data []byte) (Notes, error)` (empty input → empty Notes)
  - `func ParseTimeHint(h string) (seconds float64, ok bool)` — `"0:04 – 0:12"` → 8; accepts `–`, `—`, `-`, optional spaces, `m:ss`; anything else → false
  - `func EstimateNarrationSeconds(text string) float64` — syllables (space-separated tokens after removing `[pause …]`) / 3.0 + sum of pause ms / 1000
  - `type Warning struct{ SlideID, Message string }`
  - `func ScriptWarnings(s *Script) []Warning` — per slide: estimate > 12.5 s → `"Lời đọc ~%.0fs, có thể vượt giới hạn 14s"`; whole script: Σ(estimate + 0.8) outside 15–90 → SlideID `""`, `"Tổng thời lượng ước tính %.0fs (nên 15–90s)"`
  - `type SlideChange struct{ ID, Kind string; Fields []string }` (Kind `"added"|"removed"|"changed"`); `func Diff(a, b *Script) []SlideChange` — compares by `ID`; Fields among `title, subtitle, narration, scene, other` in that order

- [ ] **Step 1: Failing tests:** `TestParseSample` (3 slides, summaries `"template: group"`, `"layers: sun, clouds, elephant"`, sources len 2); `TestWithSlideTextPreservesOthers` (edit slide 2 narration → re-parse: slide 1 Raw unchanged JSON-equal, slide 2 narration new, `scene` intact, `sfx` intact); `TestWithSlideRawRejectsIdChange`; `TestParseTimeHint` (table: `"0:00 – 0:04"`→4, `"0:04-0:12"`→8, `"1:00 — 1:30"`→30, `"4s"`→false); `TestEstimate` (`"một hai ba [pause 400ms] bốn"` → 4/3+0.4 ≈ 1.733); `TestWarnings` (60-syllable narration → slide warning; 2 tiny slides → total warning); `TestDiff` (rename title of s1, remove s3, add s4 → `[{s1 changed [title]} {s3 removed} {s4 added}]` sorted by position in b then removed ones last).
- [ ] **Step 2: Run** `go test ./internal/scriptmodel/...` → FAIL. **Step 3: Implement.** **Step 4:** PASS. **Step 5: Commit** `feat: add script model, notes and warnings`.

### Task 6: Pipeline commands, progress, report

**Files:**
- Create: `internal/pipeline/commands.go`, `internal/pipeline/progress.go`, `internal/pipeline/report.go`, `internal/pipeline/validate.go`
- Test: `internal/pipeline/*_test.go`; fixtures `internal/pipeline/testdata/make_video.log` (copy the `$ …` lines + output of a real run, e.g. `make-voice-free/voice/co_loa_v2.log`, trimmed to ~30 lines), `testdata/report.json` (copy of `make-video-free/output/khuc_thua_du.report.json`)

**Interfaces:**
- Consumes: `config.Settings`, `config.VoiceProfilePath`, `config.MusicPath`, `runner.Cmd`, `runner.ChildEnv`, `scriptmodel.*`
- Produces:
  - `func ValidateCmd(s config.Settings, script string) runner.Cmd` — `VideoPython -m history_shorts validate --script <script>`, Dir `VideoRepo`
  - `func PreviewCmd(s config.Settings, script, outMP4 string) runner.Cmd` — `… render --script <script> --out <outMP4> --preview`
  - `func RenderCmd(s config.Settings, script, workDir, outMP4, voiceProfile, music string) runner.Cmd` — `VideoPython tools/make_video.py --script … --work … --out … [--music <MusicPath>] --voice-cmd <VoiceCmdTemplate>`, Dir `VideoRepo`
  - `func VoiceCmdTemplate(voiceBin, profilePath string) string` → `<q(voiceBin)> voice {text} {wav} --voice <q(profilePath)> --allow-noncommercial`; `q` leaves `[A-Za-z0-9_./-]+` as is, otherwise wraps in single quotes with `'` → `'\''`
  - `func ParseValidate(output string) (errs, warns []string)` — warns: lines starting `WARN `; errs: other lines containing `›`, plus lines starting `ERROR`
  - `type Step string` consts `StepValidate="validate"`, `StepExport="export"`, `StepVoice="voice"`, `StepAudio="audio"`, `StepRender="render"`
  - `type Progress struct{ Step Step; VoiceDone int; VoiceID string }`; `func (p *Progress) Feed(line string) bool` — only lines starting `"$ "`: contains `history_shorts validate`→validate; `export-narration`→export; `audio-manifest`→audio; `history_shorts render`→render; otherwise voice: `VoiceDone++`, `VoiceID` = basename without ext of the first arg ending in `.txt`; returns true if changed
  - `func PendingVoices(voiceDir string, s *scriptmodel.Script) int` — slides with non-empty narration where `voiceDir/<id>.txt` is missing, its content trimmed ≠ narration trimmed, or `<id>.wav`/`.flac` missing
  - `type ReportSlide struct{ Index int \`json:"index"\`; Duration float64 \`json:"duration"\`; PNG string \`json:"png"\` }`; `type Report struct{ TotalDuration float64 \`json:"total_duration"\`; Slides []ReportSlide \`json:"slides"\` }`; `func ReadReport(path string) (*Report, error)`; `func ReportPath(mp4 string) string` (`x.mp4` → `x.report.json`)
  - `func SlideStarts(r *Report) []float64` (cumulative)
  - `func TimingWarnings(s *scriptmodel.Script, n scriptmodel.Notes, r *Report) []scriptmodel.Warning` — slide i ↔ report slide i; `|duration − TimeHint| > 2` → `"Cảnh dài %.1fs, kịch bản ghi %.0fs"`
  - `func Tail(text string, n int) []string`
  - `type Check struct{ Name string; OK bool; Detail string }`; `func Doctor(ctx context.Context, s config.Settings, r runner.Runner) []Check` (file `internal/pipeline/doctor.go`, F1.3) — runs `VideoPython -m history_shorts --version` (Dir VideoRepo), `VoiceBin --help`, `ClaudeBin --version`, `ffmpeg -version`, all with `ChildEnv`; each 10 s timeout; Detail = first output line or `"Không chạy được: <err>"`

- [ ] **Step 1: Failing tests:**
  - `TestDoctorReportsFailure`: fake runner fails the claude command → that Check `OK=false` with Detail starting `"Không chạy được"`, others `OK=true`.
  - `TestRenderCmd`: exact arg slice for settings with `VideoRepo=/r/v`, music `"a.mp3"` → `--music /r/v/music/a.mp3`; with music `""` no `--music`.
  - `TestVoiceCmdTemplateQuotes` (Review Focus #2): `voiceBin="/Users/x/My Code/mvf/.venv/bin/make-voice-free"`, profile `/a/voices/narrator_mid.json` → `'/Users/x/My Code/mvf/.venv/bin/make-voice-free' voice {text} {wav} --voice /a/voices/narrator_mid.json --allow-noncommercial`; a path containing `'` round-trips through a Go port of `shlex.split` written in the test (or run `python3 -c "import shlex,sys;print(shlex.split(sys.argv[1]))"`, skip if python3 missing).
  - `TestParseValidate`: input from the real output format (`WARN script.json › slides[1].subtitle: …`, `script.json › slides[2].scene.layers[4].x: must be 0–1080`, `1 errors, 1 warnings`) → 1 err, 1 warn.
  - `TestProgressFromLog`: feeding `testdata/make_video.log` yields step sequence validate → export → voice (VoiceDone = number of voice lines in fixture, last VoiceID `cau-vuc` or as in fixture) → audio → render.
  - `TestPendingVoices`: temp voice dir: slide a (txt equal + wav) → 0; slide b txt differs → counted; slide c no files → counted; slide without narration ignored.
  - `TestReportAndStarts`: fixture report → starts `[0, 4.867, 9.8, …]` (first 3 values from fixture); `TimingWarnings` with notes hint `"0:00 – 0:08"` on slide 1 → 1 warning.
- [ ] **Step 2: Run** `go test ./internal/pipeline/...` → FAIL. **Step 3: Implement.** **Step 4:** PASS. **Step 5: Commit** `feat: add pipeline commands and progress parsing`.

### Task 7: Script generation with Claude

**Files:**
- Create: `internal/scriptgen/contract.go`, `internal/scriptgen/prompt.go`, `internal/scriptgen/claude.go`, `internal/scriptgen/generate.go`
- Create: `testdata/fakes/fake_claude.sh` — reads stdin, writes it to `$FAKE_CLAUDE_LOG` if set, then prints the content of `$FAKE_CLAUDE_REPLY_<n>` file where n = call count stored in `$FAKE_CLAUDE_STATE` (counter file), exits `$FAKE_CLAUDE_EXIT` (default 0)
- Test: `internal/scriptgen/*_test.go`

**Interfaces:**
- Consumes: `runner.Runner/Cmd/ChildEnv`, `config.Settings`, `scriptmodel.Notes`, `workspace.ChatMessage`
- Produces:
  - `func LoadContract(videoRepo string) (string, error)` — from `docs/raw_requirement.md`: text from line starting `## 6. Data Contracts` up to (excl.) `## 7.` + text from `## Appendix A` up to (excl.) `## Appendix B`; error if either heading missing
  - `func LoadExamples(videoRepo string) ([]string, error)` — contents of `examples/hai_ba_trung/script.json` and `examples/vuong_trieu_nu/script.json` (skip missing; error if none)
  - `type Request struct{ Input string; Current []byte; Notes *scriptmodel.Notes; Chat []workspace.ChatMessage; Instruction string; Errors []string }`
  - `type Output struct{ Script json.RawMessage \`json:"script"\`; Notes scriptmodel.Notes \`json:"notes"\`; Summary string \`json:"summary"\` }`
  - `func BuildPrompt(contract string, examples []string, r Request) string`
  - `func ParseEnvelope(stdout []byte) (string, error)` — Claude `--output-format json` object: if `is_error` true → error with `result`; else return `result`
  - `func ExtractOutput(text string) (*Output, error)` — strip ```` ```json ````/```` ``` ```` fences if present; else take substring from first `{` to last `}`; unmarshal; error if `script` empty
  - `type Generator struct{ Runner runner.Runner; Settings config.Settings; Contract string; Examples []string; Timeout time.Duration; MaxRepairs int }`
  - `type Validator func(ctx context.Context, script []byte) (errs []string, err error)`
  - `type InvalidScriptError struct{ Errors []string; Last *Output }` (implements `error`, message Vietnamese: `"Script vẫn lỗi sau %d lần sửa"`)
  - `func (g *Generator) Generate(ctx context.Context, r Request, validate Validator) (*Output, error)`

Prompt content that the code must include (exact rules; wording free): role = writer of `script.json` for history-shorts; output **only** one JSON object `{"script":…, "notes":{"slides":{"<id>":{"note":…, "time_hint":…}}}, "summary":"…"}`; use `schema_version` `"2.2"`; mapping "Chữ trên hình" → `title`/`subtitle`, "Lời đọc" → `narration`, "Hình ảnh" → closest `scene` from the catalog + `motion`/`transition`, "Thời gian" → `notes.time_hint` only; `id` = Vietnamese slug without diacritics matching `^[a-z0-9][a-z0-9_-]{0,31}$`; respect limits from Global Constraints; factual statements → `claim: true` + `source`; fill `sources`, append `[CẦN KIỂM TRA: …]` to any uncertain detail; when a "Hình ảnh" asks for something not drawable (real photo, artifact) → choose closest layers and say so in `note` incl. "cần ảnh thật"; when `Current` is given: apply `Instruction`, keep unchanged slides byte-identical, keep all existing `[CẦN KIỂM TRA` markers; when `Errors` given: fix exactly these validator errors. Then sections: CONTRACT, EXAMPLES, (CURRENT SCRIPT + NOTES), (CHAT HISTORY last 10), (VALIDATOR ERRORS), INPUT / INSTRUCTION.

- [ ] **Step 1: Failing tests:**
  - `TestLoadContract`: fake repo doc with headings `## 6. Data Contracts`, `## 7. CLI`, `## Appendix A — …`, `## Appendix B — …` → contains §6 and Appendix A, not `## 7.` nor Appendix B; missing heading → error.
  - `TestBuildPromptSections`: new request has `INPUT` and no `CURRENT SCRIPT`; edit request has both `CURRENT SCRIPT` and `INSTRUCTION`; errors appear verbatim; chat trimmed to last 10.
  - `TestParseEnvelope`: `{"type":"result","is_error":false,"result":"X"}` → `"X"`; `is_error:true` → error containing result text.
  - `TestExtractOutput` (Review Focus #3): plain JSON; fenced ```` ```json\n{…}\n``` ````; prose `"Đây là script:\n{…}\nChúc vui"`; no braces → error; `script` missing → error.
  - `TestGenerateRepairs`: fake_claude reply 1 invalid, reply 2 valid; validator returns errs for reply-1 script and none for reply-2 → Output = reply 2; 2nd prompt (from `FAKE_CLAUDE_LOG`) contains the validator error text.
  - `TestGenerateGivesUp`: validator always errs → `*InvalidScriptError` after 1 + 3 calls, `Errors` = last errors.
  - `TestGenerateClaudeFails`: exit 1 → error mentioning `"Claude"`.
- [ ] **Step 2: Run** `go test ./internal/scriptgen/...` → FAIL.
- [ ] **Step 3: Implement.** Claude cmd: `{Name: s.ClaudeBin, Args: ["-p","--output-format","json","--tools","","--no-session-persistence"], Stdin: prompt, Env: runner.ChildEnv(os.Environ(), filepath.Dir(s.ClaudeBin)), Timeout: g.Timeout}`. Tests set `ClaudeBin` to `testdata/fakes/fake_claude.sh`.
- [ ] **Step 4:** PASS. **Step 5: Commit** `feat: generate script.json with Claude CLI`.

### Task 8: Library import + thumbnails

**Files:**
- Create: `internal/library/scan.go`, `internal/library/thumb.go`
- Test: `internal/library/scan_test.go`

**Interfaces:**
- Consumes: `scriptmodel.Parse`, `runner.Runner`, `config.Settings`
- Produces:
  - `type Candidate struct{ MP4, Report, Title, ScriptPath string; Duration float64 }`
  - `func Scan(videoRepo string) ([]Candidate, error)` — for each `output/*.mp4` with sibling `<stem>.report.json`: read `script_sha256`, `total_duration`; hash every `examples/*/script.json` (sha256 of file bytes); match → `ScriptPath` + `Title` from that script; else `Title` = stem with `-`/`_` → space. mp4 without report skipped. Sorted by mp4 mtime desc.
  - `func ThumbCmd(mp4, outJPG string) runner.Cmd` — `ffmpeg -y -ss 0.5 -i <mp4> -frames:v 1 -vf scale=270:-1 <outJPG>`
  - `func EnsureThumb(ctx context.Context, r runner.Runner, mp4, outJPG string) error` — no-op if `outJPG` exists and newer than mp4

- [ ] **Step 1: Failing tests:** temp repo: `output/a.mp4`+`a.report.json` (sha of `examples/x/script.json`) → ScriptPath set, Title from script; `output/b.mp4`+report with unknown sha → ScriptPath `""`, Title `"b"`; `output/c.mp4` without report → absent; `TestThumbCmd` exact args.
- [ ] **Step 2: Run** → FAIL. **Step 3: Implement.** **Step 4:** PASS. **Step 5: Commit** `feat: scan existing renders for import`.

### Task 9: Job queue + service orchestrator

**Files:**
- Create: `internal/jobs/queue.go`; `internal/service/service.go`, `internal/service/create.go`, `internal/service/edit.go`, `internal/service/render.go`, `internal/service/recover.go`, `internal/service/importer.go`, `internal/service/dto.go`
- Create: `testdata/fakes/fake_make_video.sh` — prints the `$ …` lines of a run (validate, export-narration, 2 voice lines with `.txt` args, audio-manifest, render), writes a tiny file at the `--out` path and a report JSON at `<out stem>.report.json` (2 slides, 4.0 s each); exits `$FAKE_MV_EXIT`; sleeps `$FAKE_MV_SLEEP` s before the render line. `testdata/fakes/fake_validate.sh` — prints `$FAKE_VALIDATE_OUT`, exits `$FAKE_VALIDATE_EXIT`. (Service tests point `VideoPython` at a dispatcher fake that routes `-m history_shorts validate|render --preview` and `tools/make_video.py` to these.)
- Test: `internal/jobs/queue_test.go`, `internal/service/service_test.go`

**Interfaces:**
- Consumes: everything from Tasks 1–8.
- Produces:
  - `jobs`: `type Queue struct`; `func NewQueue() *Queue`; `func (q *Queue) Enqueue(key string, fn func(ctx context.Context) error) int` (returns 0 if starts now, else 1-based waiting position); `func (q *Queue) Cancel(key string) bool` (cancels running ctx or removes waiting); `func (q *Queue) Position(key string) int` (0 running, -1 absent); `func (q *Queue) Wait()` (tests only: until idle)
  - `service`:
    - `var ErrBusy = errors.New("Video đang được xử lý, hãy đợi hoặc huỷ")`
    - `type Deps struct{ Store *store.Store; WS *workspace.Workspace; Settings func() config.Settings; Runner runner.Runner; Gen *scriptgen.Generator; Emit func(event string, data any); Notify func(title, body string) }`
    - `func New(d Deps) *Service` (creates `renderQ`, `claudeQ`)
    - `func (s *Service) CreateFromText(input, voice, music string) (int64, error)` — dir from first non-empty input line, write `input.md`, video `GENERATING_SCRIPT`, enqueue claude job → on success: version 1 (author claude), status `SCRIPT_REVIEW`, title from script; on failure: `FAILED`, detail `"Tạo script: <msg>"`
    - `func (s *Service) Chat(videoID int64, message string) error` — `ErrBusy` if status `RENDERING`/`GENERATING_SCRIPT`; append user msg; status `GENERATING_SCRIPT`; job → new version (author claude, summary) + assistant chat msg = summary; status `SCRIPT_REVIEW`
    - `func (s *Service) EditSlideText(videoID int64, slideID, title, subtitle, narration string) error`; `func (s *Service) EditSlideRaw(videoID int64, slideID, raw string) error` — `ErrBusy` as above; run validate; validator errors → return error listing them, no version; else new version (author manual), notes copied from current, status `SCRIPT_REVIEW`
    - `func (s *Service) RestoreVersion(videoID int64, n int) error` — new version N+1 with content of n (author restore)
    - `func (s *Service) Preview(videoID int64) error` — render queue; `PreviewCmd` → `PreviewMP4Path`; render row kind preview; emits `video:changed`
    - `func (s *Service) Render(videoID int64) error` — status `RENDERING`; render queue job runs `RenderCmd` with `VoiceDir`, streams lines to `RenderLogPath` + `Progress.Feed` → emit `job:progress` `{videoId, step, voiceDone, voiceTotal(PendingVoices), voiceId, elapsedSec, queuePos}` and `job:log` `{videoId, line}`; success → render DONE, status `READY`, `Notify("Video đã render xong", title)`; failure → `FAILED`, detail `"Render (<step>): " + last line`; cancel → status `SCRIPT_REVIEW`, render CANCELED
    - `func (s *Service) Retry(videoID int64) error` — re-runs per `LastJob(videoID).Kind` (script → claude again from `input.md` or last chat message; render/preview → same)
    - `func (s *Service) Cancel(videoID int64) error`
    - `func (s *Service) Recover() error` — `Store.MarkInterrupted()`; each affected video → `FAILED`, detail `"Bị gián đoạn (app đã tắt khi đang chạy)"`
    - `func (s *Service) Import(ctx context.Context) (int, error)` — `library.Scan`; skip existing `FindVideoBySource`; create `origin=import`, status `READY`, `Editable = ScriptPath != ""`; editable → workspace dir, copy script → `script.v1.json` (author import); render row DONE pointing at original mp4 + report; returns count added
    - `func (s *Service) Hide(videoID int64) error`; `func (s *Service) Delete(videoID int64) error` — app origin: `notify.MoveToTrash(dir)` then DB delete; import origin → Hide
    - `func (s *Service) List() ([]VideoSummary, error)`; `func (s *Service) Detail(videoID int64) (VideoDetail, error)`
  - DTOs (`dto.go`, json camelCase): `VideoSummary{ID, Title, Status, StatusDetail, Origin string/…, Editable bool, Duration float64, UpdatedAt string, ThumbURL string, QueuePos int}`; `SlideDTO{ID, Title, Subtitle, Narration, SceneSummary, Note, TimeHint string; Claim, NeedsCheck bool; PreviewURL string; StartSec float64; RawJSON string}`; `VersionDTO{N int; Author, Summary, CreatedAt string}`; `RenderDTO{ID int64; Version int; Kind, State, URL, Error string; Seconds float64}`; `ChangeDTO = scriptmodel.SlideChange`; `VideoDetail{Video VideoSummary; Voice, Music string; CurrentVersion int; Versions []VersionDTO; Slides []SlideDTO; Sources []SourceDTO{Text string; NeedsCheck bool}; Warnings []scriptmodel.Warning; Changes []ChangeDTO /*current vs previous*/; Renders []RenderDTO; Chat []workspace.ChatMessage; LogTail []string}` — URLs from Task 10's `media.RenderURL(id)`, `media.FileURL(videoID, rel)`, `media.ThumbURL(videoID)`; `Warnings` = `ScriptWarnings` + validator warns (from the validate run done when the version was created; cached in memory by script path, re-run lazily once after app restart) + `TimingWarnings` of latest DONE render of the current version
  - Events: `"video:changed"` payload `{id}`; `"job:progress"`; `"job:log"`.

- [ ] **Step 1: Failing queue tests:** two jobs same queue run sequentially (second's `Enqueue` returns 1); `Cancel` of waiting job → never runs; `Cancel` of running → its ctx done.
- [ ] **Step 2: Failing service tests** (temp workspace, real store, fakes, `Emit` collects events):
  - `TestCreateFromTextHappyPath`: fake claude valid → after `Wait`, status `SCRIPT_REVIEW`, 1 version, `input.md` content equal, title from script.
  - `TestCreateFromTextInvalidAfterRepairs`: → `FAILED`, detail starts `"Tạo script:"`.
  - `TestRenderHappyPath`: → `READY`, render DONE, progress events end at step `render`, Notify called once, log file contains the `$ ` lines.
  - `TestRenderFailure`: `FAKE_MV_EXIT=5` → `FAILED`, detail starts `"Render (render):"`.
  - `TestEditBlockedWhileRendering` (Review Focus #5): `FAKE_MV_SLEEP=2`, start render, `EditSlideText` and `Chat` → `ErrBusy`; version count unchanged.
  - `TestCancelRender`: cancel during sleep → status `SCRIPT_REVIEW`, render `CANCELED`.
  - `TestRecoverMarksInterrupted` (Review Focus #4): insert video `RENDERING` + job `RUNNING`; new Service `Recover()` → `FAILED`, detail `"Bị gián đoạn (app đã tắt khi đang chạy)"`; `Retry` then runs render.
  - `TestEditCreatesVersionAndDiff`: edit narration of slide 1 → version 2 author manual; `Detail().Changes == [{id, "changed", ["narration"]}]`.
  - `TestImportIdempotent`: run `Import` twice → second returns 0.
- [ ] **Step 3: Run** `go test ./internal/jobs/... ./internal/service/...` → FAIL.
- [ ] **Step 4: Implement.** Keep each service file focused (create/edit/render/recover/importer). Render job writes log via `os.OpenFile(append)` and flushes per line.
- [ ] **Step 5: Run** `go test ./...` → PASS. **Step 6: Commit** `feat: add job queue and service orchestrator`.

### Task 10: Media handler, notifications, Wails bindings

**Files:**
- Create: `internal/media/handler.go`, `internal/notify/notify.go`
- Modify: `main.go`, `app.go`
- Test: `internal/media/handler_test.go`, `internal/notify/notify_test.go`

**Interfaces:**
- Consumes: `service.Service`, `store.Store`, `workspace.Workspace`
- Produces:
  - `func RenderURL(renderID int64) string` → `/media/render/<id>`; `func FileURL(videoID int64, rel string) string` → `/media/file/<videoID>/<url-escaped rel>`; `func ThumbURL(videoID int64) string` → `/media/thumb/<videoID>`
  - `type Resolver interface{ RenderPath(id int64) (string, error); VideoDir(videoID int64) (string, error); ThumbPath(ctx context.Context, videoID int64) (string, error) }` (implemented by `*service.Service`; ThumbPath calls `library.EnsureThumb` into `Root/thumbs/<id>.jpg` using the latest DONE render or import mp4)
  - `func Handler(r Resolver) http.Handler` — `http.ServeContent` (Range support); `file` route: cleaned `rel` must stay inside `VideoDir` (reject `..`, absolute) → 404
  - `notify`: `func Send(title, body string) error` (`osascript -e 'display notification "<body>" with title "<title>"'` with `"` and `\` escaped); `func AppleScriptString(s string) string`; `func MoveToTrash(path string) error` (`osascript -e 'tell application "Finder" to delete POSIX file "<path>"'`)
  - `app.go` bound methods (each a 1-line delegate): `ListVideos`, `GetVideo(id)`, `CreateVideo(input, voice, music)`, `Chat(id, msg)`, `EditSlideText(id, slideID, title, subtitle, narration)`, `EditSlideRaw(id, slideID, raw)`, `RestoreVersion(id, n)`, `Preview(id)`, `Render(id)`, `Retry(id)`, `Cancel(id)`, `ImportExisting()`, `Hide(id)`, `Delete(id)`, `OpenFolder(id)` (`open <dir>`), `GetSettings`, `SaveSettings(s)`, `ListVoices`, `ListMusic`, `RunDoctor`
  - `main.go`: `AssetServer: &assetserver.Options{Assets: assets, Handler: media.Handler(svc)}`; `OnStartup`: open store at `<Workspace>/app.db`, load settings, `svc.Recover()`, `svc.Import()` in background; `Emit` = `runtime.EventsEmit(ctx, …)`; `OnBeforeClose`: if a render is running → `runtime.MessageDialog` "Đang render, thoát sẽ huỷ. Thoát?" → on yes cancel all and allow close

Note on F5.4: `osascript` notifications cannot open a specific video when clicked (macOS opens Script Editor). MVP shows the notification without click routing; the finished video is highlighted in the library instead. Record this as a known deviation in `docs/prd.md` §11.

- [ ] **Step 1: Failing tests:** `TestHandlerRange` (request `Range: bytes=0-3` on a render → 206, 4 bytes); `TestHandlerRejectsTraversal` (`/media/file/1/..%2F..%2Fetc%2Fpasswd` → 404); `TestHandlerUnknownRender` → 404; `TestAppleScriptString` (`He said "hi" \ ok` → `"He said \"hi\" \\ ok"`).
- [ ] **Step 2: Run** `go test ./internal/media/... ./internal/notify/...` → FAIL. **Step 3: Implement** handler, notify, then wire `main.go`/`app.go`. **Step 4:** PASS; `wails build` succeeds; `wails dev` starts and `ListVideos` returns imported videos (check in devtools console: `await window.go.main.App.ListVideos()`).
- [ ] **Step 5: Edit** `docs/prd.md` §11: add the notification deviation line. **Step 6: Commit** `feat: wire Wails bindings, media handler, notifications`.

### Task 11: Frontend — shell, Library, Settings

**Files:**
- Create: `frontend/src/api.ts`, `frontend/src/components/Library.tsx`, `frontend/src/components/Settings.tsx`, `frontend/src/components/StatusBadge.tsx`
- Modify: `frontend/src/App.tsx`, `frontend/src/style.css` (replace template content)

**Interfaces:**
- Consumes: Wails-generated `frontend/wailsjs/go/main/App` + `models`, `EventsOn` from `wailsjs/runtime`.
- Produces: `api.ts` re-exports typed calls and `onVideoChanged(cb)`, `onProgress(cb)`, `onLog(cb)` (each returns unsubscribe). `App.tsx` state: `view: {kind:"video", id} | {kind:"new"} | {kind:"settings"}`.

Layout: left sidebar 300 px (button **+ Video mới**, search box, list rows: thumbnail 54×96, title, `StatusBadge`, duration, relative time; gear → Settings at bottom), right panel = current view. Status labels: DRAFT "Nháp", GENERATING_SCRIPT "Claude đang viết…", SCRIPT_REVIEW "Chờ duyệt script", RENDERING "Đang render" (+ queue pos "Chờ #n"), READY "Sẵn sàng", FAILED "Lỗi". Imported non-editable rows show tag "Chỉ xem". Row menu: Mở thư mục, Ẩn / Xoá (confirm dialog). Settings form for all `Settings` fields + voice/music selects + "Kiểm tra" button listing `Check`s with ✓/✗ + Detail; Save button.

- [ ] **Step 1:** Implement components; list refreshes on `video:changed`.
- [ ] **Step 2: Verify** `cd frontend && npm run build` → no TS errors. `wails dev`: library lists the ~6 imported videos with thumbnails; search filters; Settings shows defaults, "Kiểm tra" shows 4 checks (all ✓ on the owner's machine).
- [ ] **Step 3: Commit** `feat(ui): library and settings`.

### Task 12: Frontend — New video, script review, render progress

**Files:**
- Create: `frontend/src/components/NewVideo.tsx`, `ScriptReview.tsx`, `SlideCard.tsx`, `RenderProgress.tsx`, `VideoView.tsx`

**Interfaces:**
- Consumes: `api.ts`; `VideoDetail`, `SlideDTO`, progress payload from Task 9.
- Produces: `VideoView` = the right panel for a video; switches body by status: `GENERATING_SCRIPT` → waiting card ("Claude đang viết script…", elapsed timer, Huỷ); `SCRIPT_REVIEW` → `ScriptReview`; `RENDERING` → `RenderProgress`; `READY` → player area (Task 13); `FAILED` → error box (detail + `LogTail` collapsible + Thử lại).

- NewVideo: big textarea (placeholder = the CẢNH 1… example format), voice + music selects (defaults from settings), **Tạo script** → `CreateVideo` → navigate to that video.
- ScriptReview: top bar (title, version selector, Preview khung hình, **Render**); warnings list (yellow); sources list with `[CẦN KIỂM TRA` items highlighted; one `SlideCard` per slide: index, title, subtitle, narration, scene summary, Claude note (highlighted box), time hint, preview PNG (if preview done), estimated voice seconds; `NeedsCheck` text spans highlighted.
- RenderProgress: 5-step stepper (Kiểm tra · Xuất lời đọc · Đọc voice k/N · Ghép audio · Render video), current step spinner, elapsed + ETA (`(voiceTotal − voiceDone) × 30 s` + 60 s for render), "Chờ #n" when queued, collapsible live log (last 200 lines from `job:log`), Huỷ.

- [ ] **Step 1:** Implement. **Step 2: Verify** `npm run build` OK; `wails dev` with Settings `ClaudeBin` and `VideoPython` pointed at the fakes in `testdata/fakes/`: create → waiting → review cards → Render → stepper advances through all 5 steps → READY. Then with real tools on one 2-slide input: script generated, preview PNGs appear, render completes.
- [ ] **Step 3: Commit** `feat(ui): new video, script review and render progress`.

### Task 13: Frontend — player, chat, manual edit, versions

**Files:**
- Create: `frontend/src/components/Player.tsx`, `ChatPanel.tsx`, `SlideEditor.tsx`, `VersionBar.tsx`
- Modify: `VideoView.tsx`, `ScriptReview.tsx`, `SlideCard.tsx`

- Player (READY and also shown above ScriptReview when a render of any version exists): `<video>` 9:16 (height fills panel, max 720 px), controls, loop toggle, speed select 0.75/1/1.25/1.5; render selector (version + time); scene list beside it — click → `currentTime = StartSec`; current scene highlighted while playing; timing warnings shown on scenes.
- ChatPanel (right column under player, editable videos only): messages from `Chat`, input + Gửi → `Chat(id, msg)`; disabled with reason while `RENDERING`/`GENERATING_SCRIPT`.
- SlideEditor: "Sửa" on a SlideCard → inline fields title/subtitle/narration + "JSON nâng cao" toggle (textarea `RawJSON`) → Lưu → `EditSlideText`/`EditSlideRaw`; validator errors shown inline.
- VersionBar: versions with author icon (Claude / tay / khôi phục / import) + summary; changes of current vs previous (`Changes`) as chips "cảnh hook: narration"; "Khôi phục bản này" → `RestoreVersion`.
- Non-editable imported videos: player only, banner "Video import không có script gốc — chỉ xem".

- [ ] **Step 1:** Implement. **Step 2: Verify** `npm run build`; `wails dev` with fakes: chat → new version + summary message → review; manual edit → version + chip; restore → new version; player seeks by scene; edit buttons disabled during render.
- [ ] **Step 3: Commit** `feat(ui): player, chat, manual edit and versions`.

### Task 14: End-to-end check on real tools + docs

**Files:**
- Modify: `CLAUDE.md` (commands, package map), `README.md` (create: what it is, setup, `wails dev`/`wails build`, Settings paths)

- [ ] **Step 1:** `go test ./... && go vet ./... && (cd frontend && npm run build)` → all pass.
- [ ] **Step 2:** `wails build`, open `build/bin/clip-publisher.app` **from Finder** (Review Focus #1). Settings → Kiểm tra: 4 ✓.
- [ ] **Step 3:** New video with the owner's "Nỏ thần" 2-scene sample → script review shows notes "cần ảnh thật" on scene 1, `[CẦN KIỂM TRA` highlighted → Preview khung hình → Render → READY, notification appears, player plays with voice `narrator_mid`.
- [ ] **Step 4:** Chat "cảnh 2 ngắn lại" → v2 → Render → only scene 2 voice re-recorded (stepper shows Voice 1/1).
- [ ] **Step 5:** Quit during render → relaunch → video `FAILED` "Bị gián đoạn" → Thử lại → READY; `pgrep -f make_video.py` empty after quit.
- [ ] **Step 6:** Confirm `git -C ~/Documents/Code/make-video-free status` and `make-voice-free` show no changes made by the app.
- [ ] **Step 7: Commit** `docs: README and CLAUDE.md commands`.
