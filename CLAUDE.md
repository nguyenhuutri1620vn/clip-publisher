# CLAUDE.md

Quy tắc làm việc cho Claude Code trong repo này. Đọc file này và `docs/prd.md` trước mọi thay đổi. Mã yêu cầu (F1.x, C1…) theo `docs/prd.md`; bối cảnh dài hạn (đăng bài, lộ trình) ở `docs/raw_requirement.md`.

## Project trong 1 đoạn

`clip-publisher` là app desktop macOS (Wails v2: Go + React/TypeScript) để quản lý và tạo short video lịch sử Việt Nam. Nó **điều phối** 2 project Python anh em, không chứa logic dựng hình hay tổng hợp giọng:

| Project | Đường dẫn | Vai trò | Cách gọi |
|---|---|---|---|
| make-video-free (`history-shorts`) | `~/Documents/Code/make-video-free` | `script.json` → mp4 1080×1920 + `<tên>.report.json` | `.venv/bin/python -m history_shorts validate/render …`, pipeline `tools/make_video.py` (chạy với cwd = repo đó) |
| make-voice-free | `~/Documents/Code/make-voice-free` | narration `.txt` → `.wav` | `.venv/bin/make-voice-free voice {text} {wav} --voice voices/<profile>.json --allow-noncommercial` (truyền qua `--voice-cmd` của make_video.py) |
| Claude Code CLI | `claude` trên PATH | văn bản kịch bản → `script.json`, sửa script theo chat | `claude -p …` không tương tác, chỉ trả văn bản; app tự ghi file |

Hợp đồng `script.json` (schema 2.2, layer catalog, giới hạn voice 0.3–14 s / cảnh ≤ 15 s) nằm ở `make-video-free/docs/raw_requirement.md` mục 6 + Phụ lục A. Đó là nguồn sự thật, không chép cứng vào code.

## Quy tắc cứng

1. **Không sửa, di chuyển hay xoá file trong 2 repo anh em.** Chỉ đọc (examples, voices, music, docs, output) và gọi CLI của chúng. Cần thay đổi bên đó → dừng lại, báo chủ kênh. (C2, phản mục tiêu)
2. **Không chạy lệnh qua shell.** `exec.Command(name, args...)`; `--voice-cmd` là chuỗi template do make_video.py tự tách. (C4)
3. **Không bao giờ ghi đè phiên bản script hay render cũ.** Ghi file bằng ghi tạm rồi rename.
4. **Không tự gỡ nhãn `[CẦN KIỂM TRA]`** hay cờ `claim`; không dùng ảnh AI làm bằng chứng lịch sử.
5. Claude CLI chỉ được trả văn bản / JSON; không cho nó công cụ ghi file. Mọi output của Claude phải qua `history_shorts validate` trước khi coi là hợp lệ.
6. 1 render chạy một lúc. (C6)
7. Thêm dependency (Go module, npm package) → hỏi trước.
8. Thông báo lỗi hiển thị cho người dùng viết bằng tiếng Việt; code, tên biến, commit message bằng tiếng Anh.

## Lệnh

Wails v2.9.2, Go 1.23, Node 23. Module path: `clip-publisher`.

```bash
wails dev                         # chạy app ở chế độ dev (hot reload frontend)
wails build                       # build build/bin/clip-publisher.app
go test ./...                     # test Go (dùng fake trong testdata/fakes, không gọi tool thật)
go vet ./...
cd frontend && npm run build      # kiểm tra TypeScript + bundle
```

## Cấu trúc

```
main.go, app.go          Wails bootstrap + bound methods (mỏng, chỉ gọi service); /media/* qua AssetServer.Middleware
internal/workspace       đường dẫn ~/ClipPublisher, slug tiếng Việt, ghi file atomic, chat.jsonl
internal/store           SQLite (modernc, không CGO): videos, versions, renders, jobs, settings
internal/config          Settings mặc định/lưu, danh sách voice/nhạc
internal/runner          exec không shell, stream từng dòng, huỷ cả process group, ChildEnv (PATH khi mở từ Finder)
internal/scriptmodel     đọc/sửa script.json giữ thứ tự key, notes, cảnh báo, diff phiên bản
internal/pipeline        lệnh validate/preview/make_video.py, đọc tiến độ từ dòng "$ ...", report, Doctor
internal/scriptgen       prompt + gọi `claude -p`, vòng sửa lỗi validate (tối đa 3)
internal/library         quét make-video-free/output để import, thumbnail bằng ffmpeg
internal/jobs            hàng đợi tuần tự (render 1 cái một lúc)
internal/service         điều phối trạng thái video; package duy nhất app.go gọi tới
internal/media           phục vụ mp4/png/thumbnail cho webview (Range, chặn path traversal)
internal/notify          osascript: thông báo macOS, chuyển vào Thùng rác
frontend/src             React: Library, Settings, NewVideo, VideoView (Player, RenderProgress, ScriptReview, ChatPanel, VersionBar)
```

Sau khi đổi method của `App` hoặc struct trả về: `wails generate module` để cập nhật `frontend/wailsjs`.

## Kiểm thử

- Không gọi VoiceStudio, render thật hay Claude thật trong test tự động: dùng script giả (fake `make_video.py`, fake `claude`) in ra stdout giống thật.
- Logic cần test kỹ: phân tích stdout `$ …` ra tiến độ, ghép video import theo `script_sha256`, phiên bản script, state machine video, hàng đợi job.

## Ghi chú môi trường

- macOS, M1 Pro. Voice ≈ 30 s / cảnh; render 1 video vài phút.
- Giọng mặc định: `narrator_mid` (`make-voice-free/voices/`). Các file `.wav` tham chiếu của profile bị git-ignore và không tạo lại được — không bao giờ động vào.
- Workspace mặc định của app: `~/ClipPublisher`.
