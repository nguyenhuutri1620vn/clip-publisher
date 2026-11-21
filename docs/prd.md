# PRD — clip-publisher MVP: app desktop quản lý & tạo video

> Owner: Tri Nguyen · Ngày: 2026-11-21 · Trạng thái: **chờ duyệt**
> Nguồn: `docs/raw_requirement.md` (mục 0.1 là các quyết định đã chốt). Phần đăng bài tự động trong raw requirement **không** thuộc PRD này.

---

## 1. Mục đích & lý do

Chủ kênh làm short video lịch sử Việt Nam bằng 2 project Python có sẵn trên máy:

- **make-video-free** (`history-shorts`): `script.json` → video 1080×1920 phong cách tranh sơn mài.
- **make-voice-free**: narration `.txt` → giọng đọc `.wav` (VoiceStudio / OmniVoice, chạy offline).

Hiện tại không có chỗ nào để quản lý: video nằm rải rác trong `output/`, phải tự viết `script.json` (JSON dài, nhiều trường), tự gõ lệnh dài để chạy pipeline, không biết đang chạy tới đâu.

**MVP**: một app desktop nơi chủ kênh
1. thấy mọi video đã làm và xem lại ngay trong app;
2. bấm **New video**, dán kịch bản viết tự nhiên → Claude chuyển thành `script.json` → duyệt → render, có màn hình chờ cho biết đang ở bước nào;
3. chưa ưng thì sửa (chat với Claude hoặc sửa tay) rồi render lại, lặp tới khi đúng ý.

**Thành công** khi: từ lúc dán kịch bản tới lúc có video xem được, chủ kênh không phải mở terminal, không phải sửa JSON bằng tay, và luôn biết video đang ở trạng thái nào.

## 2. Mục tiêu / Không mục tiêu / Phản mục tiêu

**Mục tiêu**
- G1. Thư viện tập trung: video cũ (import) + video mới.
- G2. Kịch bản văn bản tự do → `script.json` hợp lệ, không phải sửa JSON tay.
- G3. Chạy pipeline voice + video từ app, có tiến độ theo bước.
- G4. Vòng sửa nhanh: sửa → render lại, chỉ cảnh đổi lời mới phải đọc lại voice.
- G5. Kiến trúc sẵn chỗ cho phase sau (ảnh thật, caption, đăng bài).

**Không mục tiêu (MVP)**
- Đăng lên TikTok / YouTube / Facebook, caption theo nền tảng, lịch đăng, tray, auto-start.
- Ảnh thật trong video (phase 2, mục 9).
- Windows, nhiều người dùng, đồng bộ nhiều máy.

**Phản mục tiêu** (có chạy được cũng coi là hỏng)
- App sửa, di chuyển hay xoá file trong 2 repo anh em (ngoài việc gọi CLI của chúng).
- App sinh ra video mà chủ kênh không biết nội dung khẳng định sự thật nào chưa được kiểm chứng (mất nhãn `[CẦN KIỂM TRA]`).
- Ảnh do AI tạo được trình bày như bằng chứng lịch sử.
- Mất công việc: script / video / lịch sử sửa bị ghi đè không lấy lại được.

## 3. Ràng buộc

- C1. macOS (M1 Pro). Go + **Wails v2**, UI React + TypeScript.
- C2. Chỉ giao tiếp với 2 project qua **CLI + file**. Không import code, không sửa repo của chúng trong MVP.
- C3. Gọi Claude qua **Claude Code CLI** (`claude -p`, đã cài: `~/.nvm/.../bin/claude`), dùng gói hiện có, không API key. Claude chỉ được trả về văn bản; app tự ghi file.
- C4. Mọi lệnh chạy dưới dạng `exec` với danh sách tham số, **không qua shell**.
- C5. Dữ liệu app: SQLite + thư mục làm việc trên máy. Không server, không mạng ngoài lời gọi Claude.
- C6. Render chạy **1 job một lúc** (voice + encode đã dùng nhiều CPU).
- C7. Các giới hạn của history-shorts phải được tôn trọng: voice mỗi cảnh 0.3–14 s, cảnh ≤ 15 s, `id` cảnh theo `^[a-z0-9][a-z0-9_-]{0,31}$`, narration ≤ 600 ký tự, chỉ dùng layer có trong catalog (spec make-video-free §6.3).

## 4. Cách làm đã cân nhắc

| Cách | Đánh đổi | Kết luận |
|---|---|---|
| **Wails (Go + React/TS)** | Go làm điều phối (exec, job, SQLite), React làm UI/player/chat. Nhẹ, khớp lộ trình đăng bài sau này. 2 project Python chỉ gọi được qua CLI — vốn đã là cách chúng giao tiếp. | **Chọn** |
| Python (FastAPI + pywebview) | Cùng ngôn ngữ, import thẳng `history_shorts`; nhưng 2 project có 2 venv với dependency nặng, import chung dễ xung đột; tray / đóng gói macOS kém hơn. | Loại |
| Electron | Hệ sinh thái lớn nhưng nặng, không có lợi thế gì hơn Wails ở đây. | Loại |

## 5. Khái niệm

- **Video**: 1 mục trong thư viện = 1 thư mục trong workspace (hoặc 1 file import).
- **Phiên bản script**: mỗi lần Claude tạo / sửa hoặc chủ kênh sửa tay → 1 bản `script.json` mới (v1, v2, …), không ghi đè.
- **Render**: 1 lần chạy pipeline cho 1 phiên bản script → 1 file mp4.
- **Trạng thái video**: `DRAFT` → `GENERATING_SCRIPT` → `SCRIPT_REVIEW` → `RENDERING` → `READY`; lỗi ở bước nào thì `FAILED` kèm bước + lý do, retry được. Sửa script khi đang `READY` → quay về `SCRIPT_REVIEW`.

## 6. Yêu cầu chức năng

### F1. Cấu hình (Settings)
- F1.1 Đường dẫn: repo make-video-free, repo make-voice-free (mặc định `~/Documents/Code/<tên>`), Python của từng venv (mặc định `<repo>/.venv/bin/python`, `<repo>/.venv/bin/make-voice-free`), lệnh `claude`, workspace (mặc định `~/ClipPublisher`).
- F1.2 Giọng mặc định: chọn từ `make-voice-free/voices/*.json`; mặc định `narrator_mid`. Nhạc mặc định: không nhạc; danh sách chọn lấy từ `make-video-free/music/`.
- F1.3 Nút "Kiểm tra": chạy thử từng công cụ (`python -m history_shorts --version`, `make-voice-free --help`, `claude --version`, `ffmpeg -version`) và báo cái nào thiếu.

### F2. Thư viện
- F2.1 Danh sách video: thumbnail (slide đầu), tiêu đề (`title` trong script), trạng thái, thời lượng, ngày cập nhật. Mới nhất ở trên. Tìm theo tiêu đề.
- F2.2 **Import** video có sẵn: quét `make-video-free/output/*.mp4` có `<tên>.report.json` đi kèm. Ghép với script nguồn bằng `script_sha256` trong report so với các `examples/*/script.json`; tìm được thì video import **sửa được** (app copy script vào workspace làm v1), không thì chỉ **xem**. File gốc không bị di chuyển hay sửa. Có nút "Quét lại".
- F2.3 Ẩn / xoá khỏi thư viện: video tạo trong app thì xoá thư mục (có xác nhận, chuyển vào Thùng rác macOS); video import thì chỉ ẩn.
- F2.4 Mở thư mục video trong Finder.

### F3. New video — từ văn bản sang script
- F3.1 Ô nhập văn bản tự do (dán kịch bản dạng "CẢNH n / Thời gian / Hình ảnh / Chữ trên hình / Lời đọc", hoặc chỉ là một ý tưởng). Chọn giọng, nhạc (mặc định từ F1.2). Bấm **Tạo script**.
- F3.2 App gọi Claude với: văn bản của chủ kênh + hợp đồng `script.json` (schema 2.2, catalog layer, giới hạn C7 — lấy từ `make-video-free/docs/raw_requirement.md` mục 6 và Phụ lục A, đọc lúc chạy để luôn khớp phiên bản hiện tại của repo đó) + 1–2 script mẫu từ `examples/`. Yêu cầu Claude trả về **chỉ** JSON gồm `script` (nội dung `script.json`) và `notes` (ghi chú theo từng cảnh, xem F3.5).
- F3.3 Ánh xạ: "Chữ trên hình" → `title` / `subtitle`; "Lời đọc" → `narration`; "Hình ảnh" → `scene` (chọn layer / template gần nhất trong catalog) + `motion`, `transition` hợp lý; "Thời gian" không ghi vào script (độ dài do voice quyết định) nhưng được lưu để so sánh (F5.5). Mỗi cảnh có `id` dạng slug tiếng Việt không dấu.
- F3.4 Sự thật & nguồn: câu khẳng định sự thật có `claim: true` và `source`; Claude điền `sources` và gắn `[CẦN KIỂM TRA]` cho mọi chi tiết chưa chắc (số liệu, năm, trang sách). App **không** tự gỡ nhãn này.
- F3.5 `notes` theo cảnh: ghi rõ chỗ không làm đúng được, đặc biệt "Hình ảnh" đòi thứ không có trong catalog (vd: "ảnh hiện vật mũi tên đồng → đang dùng `drum` + `map`; cần ảnh thật ở phase 2"). Lưu ở `notes.json` cạnh script (không đưa vào `script.json` để tránh cảnh báo unknown key).
- F3.6 Kiểm tra: app chạy `python -m history_shorts validate --script <file>`. Có lỗi → gửi lỗi lại cho Claude sửa, tối đa 3 vòng; vẫn lỗi → `FAILED` ở bước "Tạo script", hiện lỗi + nút sửa tay / thử lại.
- F3.7 Màn hình chờ trong lúc Claude chạy: "Claude đang viết script…", thời gian đã chạy, nút Huỷ. Timeout 5 phút → `FAILED`.

### F4. Duyệt script (cổng #1)
- F4.1 Hiển thị script theo từng cảnh dạng thẻ: số thứ tự, `title`, `subtitle`, `narration`, mô tả cảnh (tên layer/template), note của Claude (F3.5) nổi bật, nhãn `[CẦN KIỂM TRA]` được tô màu.
- F4.2 Cảnh báo trước khi render (không chặn): narration dài có nguy cơ vượt 14 s (ước lượng theo số từ), tổng thời lượng ước tính ngoài 15–90 s, validate warnings.
- F4.3 Nút **Preview khung hình**: chạy `render --preview` → hiện PNG từng cảnh trong thẻ (vài giây, không voice).
- F4.4 Nút **Render**.

### F5. Render & màn hình chờ
- F5.1 Chạy `make-video-free/tools/make_video.py` từ thư mục repo make-video-free với: `--script <workspace>/videos/<slug>/script.vN.json`, `--work <…>/voice`, `--out <…>/renders/vN.mp4`, `--music` (nếu có), `--voice-cmd "<make-voice-free> voice {text} {wav} --voice <profile.json> --allow-noncommercial"`.
- F5.2 Tiến độ theo bước, suy ra từ stdout (mỗi lệnh được in dòng `$ …`): Kiểm tra → Xuất lời đọc → **Voice k/N** (N = số cảnh cần đọc lại) → Ghép audio → Render video. Hiện thời gian đã chạy + ước tính còn lại (≈ 30 s / voice, render theo lần chạy trước). Log đầy đủ xem được (thu gọn mặc định), lưu `render.vN.log`.
- F5.3 Hàng đợi: 1 render chạy, các video khác xếp hàng, hiện vị trí. Huỷ được (dừng cả nhóm process). Đóng app khi đang render → hỏi xác nhận.
- F5.4 Xong → `READY`, thông báo macOS "Video đã render xong" (bấm mở đúng video). Lỗi → `FAILED` + 20 dòng log cuối + nút Thử lại (chạy lại cùng lệnh; voice đã có không đọc lại).
- F5.5 Sau render: đọc `vN.report.json`; so độ dài thực tế từng cảnh với "Thời gian" đã lưu (F3.3), lệch > 2 s thì cảnh báo trên thẻ cảnh.
- F5.6 App bị tắt giữa chừng → lần mở sau job đó hiện `FAILED (bị gián đoạn)` + Thử lại.

### F6. Xem & chỉnh sửa
- F6.1 Player 9:16 phát mp4 local: play/pause, tua, lặp, chọn tốc độ. Bên cạnh: danh sách cảnh, bấm cảnh → nhảy tới thời điểm của cảnh (tính từ `slides[].duration` trong report).
- F6.2 **Chat với Claude** (khung chat theo video): chủ kênh viết yêu cầu ("cảnh 2 ngắn lại", "đổi giọng hào hùng hơn ở hook") → app gửi script hiện tại + notes + lịch sử chat của video + yêu cầu → Claude trả script mới + notes + 1 dòng tóm tắt thay đổi → validate (như F3.6) → phiên bản mới, về `SCRIPT_REVIEW`.
- F6.3 **Sửa tay** trên thẻ cảnh: `title`, `subtitle`, `narration` (và tuỳ chọn ô JSON thô của cảnh cho người rành). Lưu = phiên bản mới.
- F6.4 Thay đổi giữa 2 phiên bản hiện theo cảnh (cảnh nào thêm / xoá / đổi trường nào). Quay lại phiên bản cũ được (tạo phiên bản mới bằng nội dung cũ).
- F6.5 Các render cũ vẫn giữ và xem được (chọn phiên bản trong player).

## 7. Dữ liệu & thư mục

```
~/ClipPublisher/
  app.db                         # SQLite
  videos/<slug>/
    input.md                     # văn bản gốc chủ kênh dán vào
    script.v1.json, script.v2.json, …
    notes.v1.json, …             # note theo cảnh của Claude
    chat.jsonl                   # lịch sử chat với Claude
    voice/                       # --work: narration .txt, .wav, audio_manifest.json (cache voice dùng chung giữa các phiên bản)
    renders/v2.mp4, v2.report.json, slides/, render.v2.log
```

Bảng SQLite tối thiểu: `videos` (id, slug, title, origin=`app`|`import`, source_path, status, status_detail, voice_profile, music, current_version, hidden, created_at, updated_at), `versions` (video_id, n, script_path, notes_path, author=`claude`|`manual`, summary, created_at), `renders` (video_id, version, mp4_path, report_path, log_path, state, duration, started_at, finished_at), `jobs` (id, video_id, kind=`script`|`render`|`preview`, state, pid, started_at, finished_at, error), `settings` (key, value).

## 8. Yêu cầu phi chức năng
- Không mất dữ liệu: mọi file ghi theo kiểu ghi tạm rồi đổi tên; phiên bản không bao giờ bị ghi đè.
- UI không đơ khi đang render; trạng thái cập nhật ≤ 1 s sau khi có dòng log mới.
- Lỗi luôn hiện bằng tiếng Việt, kèm bước và cách xử lý (vd: "Không tìm thấy VoiceStudio → kiểm tra Settings").
- Log app xoay vòng trong `~/ClipPublisher/logs/`.
- Kiểm thử: Go unit test cho phân tích stdout, ghép import theo sha, quản lý phiên bản, state machine; test tích hợp dùng script giả thay `make_video.py` và `claude`.

## 9. Phase 2 (ngoài MVP, để thiết kế không chặn đường)
- **Ảnh thật**: thêm layer `photo` vào make-video-free (ảnh trong khung vàng sơn mài, zoom chậm, dòng ghi nguồn) — spec riêng ở repo đó. Trong app: với cảnh có note "cần ảnh thật", Claude đề xuất từ khoá → app tìm Wikimedia Commons → hiện 3–5 ảnh kèm tác giả + giấy phép → chủ kênh chọn hoặc kéo thả ảnh của mình → ghi nguồn vào `sources`. Không dùng ảnh AI làm bằng chứng.
- Caption theo nền tảng, rồi đăng bài (raw requirement mục 5–11).

## 10. Phần để người build quyết định
- Cấu trúc package Go, thư viện SQLite (khuyến nghị `modernc.org/sqlite`, không cần CGO), migration, router/state UI.
- Prompt cụ thể cho Claude và cờ của `claude -p` (khuyến nghị `--output-format json`, tắt công cụ để Claude không tự ghi file), miễn đáp ứng F3.2–F3.6 và C3.
- Cách phục vụ file mp4 local cho player (khuyến nghị asset handler của Wails, hỗ trợ Range request để tua).
- Thiết kế giao diện chi tiết.

## 11. Các quyết định do Claude đề xuất (chủ kênh kiểm tra lại)
- "Caption" ở danh sách video = **tiêu đề** (`title`) của script — câu hỏi 7 chưa được trả lời rõ.
- Workspace mặc định `~/ClipPublisher`; video import chỉ sửa được khi tìm thấy script nguồn trong `examples/`.
- Ngưỡng cảnh báo lệch thời gian 2 s; Claude sửa lỗi validate tối đa 3 vòng; timeout Claude 5 phút.
- Thêm nút **Preview khung hình** (F4.3) ở bước duyệt script — rẻ, giúp thấy bố cục trước khi chờ voice.
- Thông báo macOS khi render xong (F5.4) đưa vào MVP vì rẻ và giải quyết đúng chuyện "chờ".
- **Lệch so với F5.4**: thông báo gửi qua `osascript` nên bấm vào **không** mở được đúng video (macOS mở Script Editor). Video vừa xong được đưa lên đầu thư viện thay thế. Muốn bấm-để-mở cần app có chữ ký + UNUserNotificationCenter (sau MVP).
- **Workspace cố định** `~/ClipPublisher` trong MVP (database nằm ở đó); ô Workspace trong Settings chỉ hiển thị.
