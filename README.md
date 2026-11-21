# Clip Publisher

App desktop macOS để quản lý và tạo short video lịch sử Việt Nam. Nó điều phối 2 project có sẵn trên máy:

- **make-video-free** (`history-shorts`): `script.json` → video 1080×1920 phong cách sơn mài.
- **make-voice-free**: lời đọc → giọng đọc (VoiceStudio, offline).

và **Claude Code CLI** để chuyển kịch bản viết tự nhiên ("CẢNH 1 / Thời gian / Hình ảnh / Chữ trên hình / Lời đọc") thành `script.json`.

Bạn làm 2 việc: dán kịch bản và duyệt. App lo phần còn lại: Claude viết script (tự sửa tới khi `history_shorts validate` chấp nhận), xem trước khung hình, đọc voice, render, báo khi xong. Chưa ưng thì chat với Claude hoặc sửa tay từng cảnh; mỗi lần sửa là một phiên bản mới.

## Cần có trên máy

- macOS, Go 1.23+, Node 20+, Wails CLI v2.9 (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.9.2`)
- `~/Documents/Code/make-video-free` và `~/Documents/Code/make-voice-free` đã cài theo README của chúng (có `.venv`)
- `ffmpeg` (Homebrew), `claude` (Claude Code CLI, đã đăng nhập)

Đường dẫn khác mặc định thì sửa trong **Cài đặt** của app; nút **Kiểm tra công cụ** cho biết cái nào thiếu.

## Chạy

```bash
wails dev      # chế độ dev; cũng mở được ở http://localhost:34115
wails build    # tạo build/bin/clip-publisher.app
```

Dữ liệu nằm ở `~/ClipPublisher/`: `app.db` (SQLite) và `videos/<tên>/` (input.md, script.vN.json, notes.vN.json, chat.jsonl, voice/, renders/). App không sửa gì trong 2 repo kia; video cũ trong `make-video-free/output/` được import để xem (và sửa được nếu script gốc nằm trong `examples/`).

## Phát triển

```bash
go test ./...                 # dùng fake trong testdata/fakes, không gọi tool thật
go vet ./...
cd frontend && npm run build  # kiểm tra TypeScript
```

Tài liệu: `docs/prd.md` (PRD), `docs/raw_requirement.md` (bối cảnh, lộ trình đăng bài), `CLAUDE.md` (quy tắc làm việc).
