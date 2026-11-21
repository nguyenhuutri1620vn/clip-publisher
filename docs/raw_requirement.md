# Raw Requirement — Auto Publisher cho short video (TikTok / YouTube Shorts / Facebook Reels)

> Tên tạm: **clip-publisher**
> Owner: Tri Nguyen
> Trạng thái: raw requirement, đã chốt phạm vi MVP ngày 2026-11-21 (xem **mục 0.1**). PRD của MVP: `docs/prd.md`.
> Ngày: 2026-11-21
> Hình thức: **App desktop** (chạy trên máy cá nhân, không phải website, không phải app mobile)

---

## 0. Kiến trúc tổng quan

App desktop **all-in-one**: giao diện + job worker + publisher chạy chung trên máy cá nhân, gọi trực tiếp 2 project make-voice-free / make-video-free có sẵn trên máy.

```
┌──────────────────────────────── App desktop (clip-publisher) ────────────────────────────────┐
│                                                                                              │
│  UI (cửa sổ chính)                      Core (chạy nền, kể cả khi đóng cửa sổ → system tray)  │
│  - Viết / quản lý kịch bản              - Job worker: gọi make-voice-free, make-video-free    │
│  - Duyệt video (player 9:16)            - Caption generator (LLM API)                         │
│  - Sửa caption theo nền tảng            - Publisher adapters: TikTok / YouTube / Facebook     │
│  - Lịch đăng (calendar)                 - Status tracker (poll theo backoff)                  │
│  - Trạng thái 3 nền tảng                - Scheduler (hẹn giờ đăng)                            │
│  - Kết nối tài khoản                    - SQLite + thư mục lưu video                          │
│                                                                                              │
└───────────────┬──────────────────────────────────────────────┬───────────────────────────────┘
                │ chạy process / CLI                           │ HTTPS
                ▼                                              ▼
     make-voice-free, make-video-free              TikTok / YouTube / Facebook API, LLM API
         (trên cùng máy)
```

Hệ quả của việc chạy trên máy cá nhân (cần thiết kế cho đúng):
- **Máy phải bật** thì mới đăng đúng giờ và theo dõi được trạng thái → app cần chạy nền ở system tray, tự khởi động cùng máy, và **xử lý bù** khi máy bật lại (job quá giờ thì đăng ngay hoặc hỏi lại).
- Ưu tiên **đẩy việc chờ đợi sang phía nền tảng**: YouTube `publishAt`, Facebook `SCHEDULED` → upload sớm khi máy đang bật, nền tảng tự public đúng giờ kể cả khi máy tắt.
- Upload video **trực tiếp từ file trên máy** (TikTok FILE_UPLOAD, YouTube resumable upload, Facebook rupload). Không cần server có URL public.
- Webhook (TikTok) cần endpoint public → app desktop **dùng polling là chính**, webhook để sau (nếu có).
- Token lưu trong **kho bảo mật của hệ điều hành** (Windows Credential Manager / macOS Keychain).

---

## 0.1 Quyết định đã chốt (2026-11-21)

**Phạm vi MVP được thu hẹp**: trước mắt chỉ làm **app desktop quản lý và tạo video**. Đăng bài lên TikTok / YouTube / Facebook (mục 4, FR-5 → FR-8) để các phase sau.

MVP gồm:
- **Thư viện video**: liệt kê video đã có (import từ `make-video-free/output/`) và video tạo mới, xem được ngay trong app.
- **New video**: dán kịch bản dạng văn bản tự do ("CẢNH 1 / Thời gian / Hình ảnh / Chữ trên hình / Lời đọc") → Claude chuyển thành `script.json` → duyệt script → render (voice + video), có màn hình chờ hiện tiến độ.
- **Chỉnh sửa lặp**: chat với Claude hoặc sửa tay từng cảnh → render lại.

| # | Câu hỏi | Quyết định |
|---|---|---|
| Q1 | 2 project viết bằng gì, input/output? | Python CLI, giao tiếp qua file: `script.json` → narration `.txt` → `.wav` + `audio_manifest.json` → `.mp4` + `.report.json`. Có sẵn pipeline 1 lệnh `make-video-free/tools/make_video.py --voice-cmd …`. |
| Q2 | Generate mất bao lâu? | Voice ≈ 30 s / cảnh trên M1 Pro (có cache theo đoạn); 1 video vài phút. |
| Q3 | Hệ điều hành? | macOS (máy hiện tại). |
| Q8 | Duyệt kịch bản (cổng #1)? | **Có**: xem script.json dạng từng cảnh trước khi chạy voice. |
| Q10 | Framework? | **Wails (Go + React/TS)**. |
| Q5, Q6, Q7, Q9, Q11 | Đăng bài, thumbnail, Telegram | Để sau MVP. |
| — | Lưu video ở đâu? | Thư mục làm việc riêng của app, mỗi video 1 thư mục. Video cũ trong `make-video-free/output/` được import, không di chuyển. |
| — | Giọng đọc | Dùng profile của make-voice-free (`voices/*.json`); mặc định `narrator_mid` (profile dùng gần nhất), đổi được theo video. |
| — | Cột "Thời gian" trong kịch bản | Chỉ để tham khảo; độ dài cảnh do voice quyết định; app cảnh báo khi lệch nhiều. |
| — | Nguồn trích dẫn | Claude tự điền `sources` + cờ `claim`, gắn nhãn `[CẦN KIỂM TRA]` để chủ kênh soát. |
| — | Chỉnh sửa | Cả 2: chat với Claude và sửa tay từng trường của cảnh. |
| — | Gọi Claude thế nào? | Claude Code CLI (`claude -p`) có sẵn trên máy, dùng gói Claude hiện có, không cần API key. |
| — | Ảnh thật (hiện vật, bằng chứng) | Làm **sau MVP**. Nguồn: **Wikimedia Commons** (Claude đề xuất, chủ kênh chọn, ghi tác giả + giấy phép); không có thì chủ kênh tự kéo thả ảnh. Không dùng ảnh AI làm "bằng chứng". Cần thêm loại layer ảnh vào make-video-free (repo riêng). |

---

## 1. Bối cảnh

Hiện có 2 project:

- **make-voice-free**: từ kịch bản → tạo file giọng đọc (voice-over).
- **make-video-free**: từ kịch bản + voice → dựng ra 1 short video hoàn chỉnh (dọc 9:16).

Quy trình hằng ngày đang làm **thủ công**:

1. Viết kịch bản.
2. Chạy make-voice-free → ra voice.
3. Chạy make-video-free → ra video.
4. Xem lại video.
5. Nghĩ caption / tiêu đề / hashtag cho từng nền tảng.
6. Mở từng nền tảng (TikTok, YouTube, Facebook) → upload → điền caption → đăng.
7. Thỉnh thoảng quay lại xem video đã lên chưa, có bị lỗi / kẹt kiểm duyệt không.

## 2. Pain point

| # | Vấn đề | Ảnh hưởng |
|---|---|---|
| P1 | Đăng thủ công trên 3 nền tảng, mỗi nơi một form, một kiểu caption | Tốn thời gian nhất trong ngày, lặp đi lặp lại |
| P2 | TikTok & YouTube có giai đoạn xử lý / kiểm duyệt sau khi upload, **không biết khi nào xong** | Phải mở nền tảng kiểm tra nhiều lần, không biết video đã public hay bị kẹt / bị từ chối |
| P3 | Không có nơi quản lý tập trung: video nào đã làm, đã đăng ở đâu, link bài đăng, trạng thái | Dễ sót, dễ đăng trùng, khó theo dõi |
| P4 | Chạy 2 project generate rời rạc bằng tay | Mất công nối output của bước này sang bước kia |

## 3. Mục tiêu (Expected)

Việc của mình **chỉ còn 2 việc**:

1. **Viết content** (kịch bản).
2. **Duyệt** (video thành phẩm + caption).

Mọi thứ còn lại chạy tự động: generate voice → generate video → sinh caption theo từng nền tảng → upload → theo dõi trạng thái xử lý / kiểm duyệt → báo kết quả.

### Tiêu chí thành công

- Thời gian thao tác tay cho 1 clip (sau khi viết xong kịch bản) **≤ 3 phút** (chỉ duyệt + bấm approve).
- 100% bài đăng có **trạng thái rõ ràng** trong app cho từng nền tảng (đang upload / đang xử lý / đã public / lỗi + lý do).
- Có **thông báo desktop** (và tuỳ chọn Telegram để biết khi không ngồi máy) khi: video sẵn sàng để duyệt, bài đã lên thành công, bài lỗi / bị từ chối.
- Không phải mở TikTok / YouTube / Facebook để kiểm tra trạng thái.

## 4. Khảo sát thị trường — các app đã giải quyết thế nào

### 4.1 Các nhóm sản phẩm

| Nhóm | Ví dụ | Cách làm | Ghi chú |
|---|---|---|---|
| Social media scheduler (SaaS) | Buffer, Metricool, Later, Publer, Hootsuite | Calendar + queue, đăng tự động hoặc "notification publishing" | App của họ đã được các nền tảng duyệt (audited) nên đăng public được ngay |
| Scheduler open-source tự host | **Postiz** (AGPL-3.0), Mixpost | Giống SaaS nhưng tự chạy | Tự host thì **phải tự tạo developer app và tự xin duyệt** ở Meta / YouTube / TikTok (có thể mất vài tuần) |
| Unified posting API | Ayrshare, Blotato, upload-post, PostProxy | 1 API → nhiều nền tảng | Trả phí, nhưng dùng app đã audit của họ → né được khâu xin duyệt |
| Workflow automation | n8n, Make, Workato | Ghép node gọi API từng nền tảng | Linh hoạt, vẫn phải tự lo app / token |

### 4.2 Các nền tảng xử lý "kiểm duyệt không biết khi nào xong" thế nào

Điểm chung: **đăng bài là bất đồng bộ (async)**. Không app nào "chờ" kiểm duyệt trong 1 request; tất cả đều làm theo mẫu:

```
upload / init  →  nhận ID (publish_id / video_id)  →  theo dõi trạng thái (poll hoặc webhook)  →  trạng thái cuối (thành công / lỗi)
```

**TikTok (Content Posting API)**
- Init publish (FILE_UPLOAD từ máy hoặc PULL_FROM_URL) → nhận `publish_id` → TikTok xử lý + kiểm duyệt.
- Theo dõi bằng 2 cách: endpoint **Fetch Status** (`/v2/post/publish/status/fetch/`) để poll, hoặc **webhook** báo kết quả cuối.
- Theo tài liệu TikTok: kiểm duyệt **thường xong trong khoảng 1 phút, đôi khi mất vài giờ**.
- Giới hạn poll: 30 request/phút cho mỗi access token.
- Trạng thái cuối: `PUBLISH_COMPLETE`, `FAILED` (kèm `fail_reason`), `SEND_TO_USER_INBOX` (nếu dùng chế độ Upload vào inbox).
- Ví dụ thực tế: Workato poll mỗi 15 giây, tối đa 10 phút rồi timeout. Điều này cho thấy **không nên poll đồng bộ** mà phải có job nền chạy dài hơn.
- ⚠️ **App chưa audit**: mọi bài đăng bị ép về **riêng tư (SELF_ONLY)**, tối đa 5 user / 24 giờ, tài khoản phải để private. Muốn public thì phải qua audit.
- Có giới hạn số bài / ngày cho mỗi creator (khoảng 15 bài/ngày, dùng chung cho mọi app).
- TikTok **không có hẹn giờ qua API** → app phải tự giữ lịch và gọi đăng đúng giờ (máy phải bật).
- Có 2 chế độ: **Direct Post** (đăng thẳng) và **Upload** (gửi vào inbox TikTok, người dùng bấm thông báo trong app TikTok để hoàn tất). Chế độ Upload có thể dùng làm phương án dự phòng khi chưa audit.

**YouTube (Data API v3)**
- `videos.insert` (upload resumable, hỗ trợ tiếp tục khi rớt mạng) → nhận `video_id` ngay → video **chưa hiển thị cho tới khi xử lý xong**.
- Theo dõi bằng `videos.list` với `part=status,processingDetails`: `status.uploadStatus` (uploaded / processed / failed / rejected / deleted), `failureReason`, `rejectionReason`, `processingDetails.processingStatus`.
- Hẹn giờ: upload sớm ở trạng thái `private` + đặt `status.publishAt` → YouTube tự public đúng giờ. **Đây là cách các tool né chuyện "không biết khi nào xử lý xong": upload trước, xử lý xong từ sớm, đến giờ thì tự public.** Rất hợp với app desktop vì không cần máy bật lúc public.
- ⚠️ Project API **chưa verify / audit**: video upload qua API bị khoá ở **private**. Có quota mặc định theo ngày.
- Shorts: video dọc, ngắn → YouTube tự nhận là Short (không có API flag riêng).

**Facebook (Reels Publishing API cho Page)**
- 3 bước: `POST /{page_id}/video_reels` (upload_phase=start) → upload file lên `rupload.facebook.com` (từ file local được) → `upload_phase=finish` (+ description, title).
- Theo dõi bằng `GET /{video_id}?fields=status`: có `uploading_phase`, `processing_phase`, `publishing_phase`, `copyright_check_status`, kèm thông báo lỗi (vd: "Resolution too low… minimum 540p").
- Hỗ trợ `video_state` = `DRAFT | PUBLISHED | SCHEDULED` → hẹn giờ được phía nền tảng.
- Chỉ đăng Reels lên **Facebook Page** (không đăng lên trang cá nhân qua API). Giới hạn 30 reels đăng qua API trong 24 giờ.
- Có cờ khai báo nội dung tạo bằng AI (`is_ai_generated`) ở một số endpoint video.

### 4.3 Các tool scheduler xử lý lỗi & trạng thái thế nào

- **Buffer / Metricool / Later**: có **"notification publishing"**: đến giờ thì đẩy thông báo, người dùng bấm vào để hoàn tất đăng trong app gốc. Dùng khi không đăng tự động được (chưa có quyền, loại nội dung không hỗ trợ).
- **Buffer**: không tự retry bài lỗi. Người dùng bấm "Retry now" hoặc "Re-add to queue". Với YouTube thì hướng dẫn kiểm tra kênh trước, vì có trường hợp báo lỗi nhưng video thực ra đã lên → **cần chống đăng trùng**.
- **Metricool**: phân biệt trạng thái "Sent" (đã gửi thông báo, chưa thật sự lên TikTok) với "Published"; có thư viện mã lỗi + hướng dẫn xử lý; một số lỗi (token hết hạn) thì chỉ cần kết nối lại tài khoản.
- **Postiz**: đánh dấu bài lỗi bằng "vòng đỏ" + tooltip lỗi từ nền tảng, người dùng tự reschedule.
- Sự cố thực tế (Buffer, TikTok): có bài kẹt ở trạng thái sending / processing hơn 1 giờ, phải retry tay → **cần timeout + cảnh báo "kẹt"**.

### 4.4 Bài học rút ra cho project

1. **Mỗi lần đăng (1 video × 1 nền tảng) là 1 job riêng có state machine riêng**: 1 video có 3 "publication" độc lập, lỗi ở TikTok không ảnh hưởng YouTube.
2. **Không chờ đồng bộ**: worker nền poll theo backoff (vd: 15s → 30s → 1m → 5m → 15m…, tối đa ~6–24h).
3. **Upload sớm, public sau** (YouTube `publishAt`, Facebook `SCHEDULED`) để khâu xử lý xong từ trước giờ đăng, và không phụ thuộc máy có bật lúc đó hay không. TikTok không hẹn giờ qua API → app tự giữ lịch.
4. **Trạng thái "kẹt"** là trạng thái hạng nhất: quá X phút vẫn processing → báo cho người dùng.
5. **Idempotency / chống đăng trùng**: lưu ID phía nền tảng ngay khi có; trước khi retry phải kiểm tra bài đã lên chưa. Đặc biệt quan trọng với app desktop vì máy có thể tắt / sleep giữa chừng.
6. **Có phương án dự phòng thủ công**: nếu nền tảng chưa cho đăng public, app vẫn chuẩn bị sẵn file + caption để mình đăng tay trong ~1 phút.
7. **Rào cản lớn nhất không phải code mà là audit / app review** của TikTok và YouTube → cần quyết định chiến lược sớm (xem mục 9).

## 5. Phạm vi (Scope)

### In scope (MVP)
- App desktop: quản lý kịch bản → job generate (voice + video) → duyệt → sinh caption → đăng 3 nền tảng → theo dõi trạng thái → thông báo.
- 1 người dùng (chính mình), 1 tài khoản mỗi nền tảng, 1 máy.
- Nền tảng: TikTok, YouTube (Shorts), Facebook Page (Reels).

### Out of scope (giai đoạn đầu)
- Nhiều user / team / phân quyền, đồng bộ nhiều máy.
- App mobile, web dashboard.
- Analytics chi tiết (view, like, retention). Có thể thêm sau.
- Trả lời comment, inbox.
- Instagram, Threads… (nhưng thiết kế nên dễ thêm nền tảng mới).

## 6. Luồng nghiệp vụ chính

```
[Mình] Viết kịch bản trong app
   │
   ▼
(1) SCRIPT_DRAFT ──(tuỳ chọn) AI chuẩn hoá / chia cảnh──► SCRIPT_READY
   │
   ▼ [Mình] Bấm "Generate" (hoặc duyệt kịch bản ← Cổng duyệt #1, có thể tắt)
(2) GENERATING_VOICE  → gọi make-voice-free (process local)
(3) GENERATING_VIDEO  → gọi make-video-free (process local)
(4) CAPTIONING        → AI sinh caption/tiêu đề/hashtag cho TỪNG nền tảng
   │
   ▼ Thông báo desktop "Video sẵn sàng duyệt"
(5) PENDING_REVIEW    ← Cổng duyệt #2: xem video + sửa caption + chọn nền tảng + chọn giờ đăng
   │  ├─ Reject → quay lại (2)/(3)/(4) kèm ghi chú
   │  └─ Approve
   ▼
(6) SCHEDULED → tạo 3 Publication (TikTok / YouTube / Facebook), mỗi cái chạy độc lập
   │
   ▼ Thông báo kết quả từng nền tảng (thành công + link / lỗi + lý do / kẹt)
(7) DONE (khi mọi publication ở trạng thái cuối)
```

### State machine cho 1 Publication (video × nền tảng)

```
QUEUED → UPLOADING → UPLOADED → PROCESSING → PUBLISHED
                │           │            ├──► SCHEDULED_ON_PLATFORM → PUBLISHED   (YouTube publishAt / FB SCHEDULED)
                │           │            ├──► REJECTED      (bị từ chối / vi phạm, có lý do)
                │           │            └──► STUCK         (quá timeout, vẫn theo dõi tiếp + cảnh báo)
                └───────────┴──────────────► FAILED        (lỗi kỹ thuật, có mã lỗi, retry được)
MANUAL_REQUIRED  (nền tảng chưa cho đăng tự động → đã chuẩn bị file + caption, chờ mình đăng tay rồi dán link)
MISSED           (quá giờ hẹn vì máy tắt → khi app mở lại: đăng ngay / dời giờ / huỷ theo cấu hình)
```

## 7. Yêu cầu chức năng

### FR-1. Quản lý kịch bản
- FR-1.1 Tạo / sửa / xoá kịch bản (text, markdown), lưu nháp tự động. Có tiêu đề, chủ đề / series, tag.
- FR-1.2 (Tuỳ chọn) AI hỗ trợ: chuẩn hoá kịch bản theo template của kênh, chia cảnh, kiểm tra độ dài (vd: ≤ 60s / ≤ 90s).
- FR-1.3 Nhập nhiều kịch bản một lúc (batch, hoặc import từ thư mục file .md/.txt) để làm trước cả tuần.

### FR-2. Điều phối generate (local)
- FR-2.1 Cấu hình đường dẫn tới make-voice-free và make-video-free (thư mục project, lệnh chạy, môi trường Python/Node… nếu có).
- FR-2.2 Gọi make-voice-free với input kịch bản → nhận file audio.
- FR-2.3 Gọi make-video-free với input kịch bản + audio → nhận file video.
- FR-2.4 Mỗi bước là 1 job nền có trạng thái, log (stdout/stderr), thời gian chạy; retry được từng bước mà không chạy lại từ đầu; huỷ được job đang chạy.
- FR-2.5 Giới hạn số job generate chạy song song (vd: 1) để máy không quá tải; các job còn lại xếp hàng.
- FR-2.6 Lưu artifact (audio, video, thumbnail nếu có) vào thư mục làm việc cố định, mở được thư mục từ app.
- FR-2.7 Kiểm tra video trước khi duyệt: tỉ lệ 9:16, độ phân giải (≥ 720p, Facebook tối thiểu 540p), thời lượng, dung lượng, codec (dùng ffprobe). Không đạt thì báo ngay, không đợi tới lúc nền tảng từ chối.

### FR-3. Sinh caption theo nền tảng
- FR-3.1 Từ kịch bản, AI sinh:
  - TikTok: caption ngắn có hook + hashtag.
  - YouTube: tiêu đề (≤ 100 ký tự) + mô tả + tags.
  - Facebook: mô tả dài hơn + hashtag.
- FR-3.2 Có template / giọng văn riêng cho kênh (lưu được, chỉnh được).
- FR-3.3 Sửa tay trước khi duyệt.
- FR-3.4 Cờ khai báo nội dung AI (AI-generated) cho từng nền tảng nếu API hỗ trợ.

### FR-4. Màn hình duyệt (Review)
- FR-4.1 Player video dọc 9:16 trong app (phát file local), có tua, phát lặp.
- FR-4.2 Bên cạnh player: 3 tab caption (TikTok / YouTube / Facebook), sửa trực tiếp, đếm ký tự theo giới hạn từng nền tảng.
- FR-4.3 Bật / tắt từng nền tảng; chọn đăng ngay hoặc hẹn giờ (theo từng nền tảng hoặc chung).
- FR-4.4 Approve / Reject (kèm ghi chú → quay lại bước tương ứng).
- FR-4.5 Hàng chờ duyệt: duyệt liên tiếp nhiều video, có phím tắt (vd: A = approve, R = reject, ←/→ = video trước/sau).

### FR-5. Đăng bài (Publisher)
- FR-5.1 Mỗi nền tảng là 1 adapter riêng, cùng interface (`Upload`, `Publish`, `GetStatus`, `Cancel`), để dễ thêm nền tảng mới hoặc đổi sang unified API (Ayrshare / Blotato / Postiz API).
- FR-5.2 TikTok: Direct Post với **FILE_UPLOAD** (upload từ file local theo chunk) → lưu `publish_id`.
- FR-5.3 YouTube: resumable upload; nếu hẹn giờ thì `privacyStatus=private` + `publishAt` → lưu `video_id`.
- FR-5.4 Facebook: Reels flow start → upload → finish (`PUBLISHED` hoặc `SCHEDULED`) → lưu `video_id`.
- FR-5.5 Upload chịu được rớt mạng / máy sleep: tiếp tục (resume) được nếu nền tảng hỗ trợ, nếu không thì upload lại sau khi đã kiểm tra chống trùng.
- FR-5.6 Chế độ **MANUAL_REQUIRED**: nếu nền tảng chưa cấu hình / chưa có quyền public → app hiện "gói đăng": nút mở thư mục chứa video, nút copy caption, nút mở trang upload của nền tảng trên trình duyệt (TikTok Studio / YouTube Studio / Meta Business Suite). Đăng xong thì dán link bài (hoặc bấm "Đã đăng") để đóng job.
- FR-5.7 Scheduler cho TikTok (và mọi nền tảng không hẹn giờ phía server): app tự giữ lịch; nếu lỡ giờ do máy tắt → trạng thái MISSED, xử lý theo cấu hình (đăng ngay / hỏi / dời sang khung giờ kế tiếp).

### FR-6. Theo dõi trạng thái (Status Tracker): giải quyết P2
- FR-6.1 Worker nền poll trạng thái theo **exponential backoff**, tôn trọng rate limit từng nền tảng (TikTok 30 req/phút/token).
- FR-6.2 Map trạng thái riêng của từng nền tảng về state machine chung (mục 6).
- FR-6.3 Ngưỡng **STUCK** cấu hình được (vd: TikTok > 30 phút, YouTube > 2 giờ, Facebook > 1 giờ) → cảnh báo, nhưng vẫn tiếp tục theo dõi tới ngưỡng bỏ cuộc (vd 24h).
- FR-6.4 Khi app khởi động lại (máy vừa bật / wake), **tiếp tục theo dõi** mọi publication chưa ở trạng thái cuối.
- FR-6.5 Lưu link bài đăng public khi thành công (TikTok post id, YouTube URL, Facebook URL).
- FR-6.6 Lưu nguyên văn lý do lỗi / từ chối từ nền tảng + gợi ý xử lý (vd: token hết hạn → kết nối lại tài khoản).
- FR-6.7 (Sau này) Webhook TikTok nếu có endpoint public (qua tunnel); MVP dùng polling.

### FR-7. Thông báo
- FR-7.1 Kênh chính: **thông báo desktop của hệ điều hành** (Windows toast / macOS notification), bấm vào mở đúng màn hình video / publication.
- FR-7.2 Kênh phụ (tuỳ chọn): Telegram bot, để biết kết quả khi không ngồi máy.
- FR-7.3 Sự kiện: video sẵn sàng duyệt; đăng thành công (kèm link); lỗi / bị từ chối (kèm lý do); kẹt quá ngưỡng; lỡ giờ đăng (MISSED); token sắp hết hạn / đã hết hạn.
- FR-7.4 Màn hình "Hoạt động" trong app lưu lịch sử sự kiện.
- FR-7.5 Tóm tắt cuối ngày (tuỳ chọn): hôm nay đăng bao nhiêu, lỗi bao nhiêu.

### FR-8. Quản lý tài khoản nền tảng
- FR-8.1 Kết nối tài khoản qua OAuth (TikTok Login Kit, Google OAuth, Facebook Login → Page token). Luồng desktop: app mở trình duyệt mặc định → đăng nhập → redirect về **loopback** `http://127.0.0.1:<port>/callback` do app lắng nghe tạm thời (hoặc custom URL scheme) → app nhận code, đổi token.
- FR-8.2 Tự refresh token; báo trước khi token hết hạn.
- FR-8.3 Lưu token / secret trong kho bảo mật của OS (Windows Credential Manager / macOS Keychain), không lưu plain text trong file config.

### FR-9. Màn hình chính / Library
- FR-9.1 Danh sách video với trạng thái tổng + trạng thái từng nền tảng (dạng 3 icon xanh / vàng / đỏ), cập nhật realtime.
- FR-9.2 Calendar view: lịch đăng theo ngày / tuần, kéo thả để đổi giờ.
- FR-9.3 Lọc theo trạng thái (đang generate, chờ duyệt, đang xử lý, lỗi, đã đăng).
- FR-9.4 Retry / Cancel từng publication. Retry phải **kiểm tra chống trùng** trước.
- FR-9.5 Màn hình theo dõi job generate đang chạy: tiến độ, log, nút chạy lại / huỷ.
- FR-9.6 Bấm vào publication đã đăng → mở bài trên trình duyệt.

### FR-10. Yêu cầu riêng cho app desktop
- FR-10.1 **Chạy nền ở system tray** khi đóng cửa sổ; menu tray: mở app, tạm dừng đăng, thoát.
- FR-10.2 **Tự khởi động cùng hệ điều hành** (bật/tắt trong Settings).
- FR-10.3 (Tuỳ chọn) Ngăn máy sleep khi đang upload / đang có job generate.
- FR-10.4 Màn hình Settings: đường dẫn make-voice-free / make-video-free, thư mục làm việc, API key LLM, ngưỡng STUCK, khung giờ đăng mặc định, số job song song.
- FR-10.5 Dọn dẹp: tự xoá / lưu trữ file trung gian (audio, bản render cũ) sau X ngày để không đầy ổ cứng.
- FR-10.6 Chỉ 1 instance chạy cùng lúc (single instance lock) để tránh 2 worker cùng đăng 1 bài.
- FR-10.7 Hệ điều hành mục tiêu: Windows trước (hoặc theo máy đang dùng), macOS sau.

## 8. Yêu cầu phi chức năng

- **Độ tin cậy**: job lưu trong DB (SQLite) → tắt app / tắt máy không mất job; khởi động lại thì chạy tiếp; mỗi bước idempotent.
- **Chống đăng trùng**: khoá theo (video_id, platform); không gọi Publish lần 2 nếu đã có ID phía nền tảng mà chưa xác minh trạng thái.
- **Quan sát được**: log theo job (file log xoay vòng), lưu response lỗi thô của nền tảng.
- **Bảo mật**: token / secret trong OS keychain, không commit vào git.
- **Tài nguyên**: app nhẹ khi chạy nền (worker idle không ngốn CPU); generate chạy giới hạn song song.
- **Chi phí**: không cần server; chỉ tốn chi phí LLM API (và unified API nếu chọn option B).
- **Mở rộng**: thêm nền tảng mới = thêm 1 adapter, không sửa core.
- **Tuân thủ**: chỉ đăng nội dung gốc của mình, có sự đồng ý rõ ràng trước khi upload (yêu cầu của TikTok), tôn trọng giới hạn số bài / ngày.

## 9. Ràng buộc & rủi ro chính

| Rủi ro | Mô tả | Hướng xử lý |
|---|---|---|
| R1. TikTok audit | App chưa audit → bài chỉ ở chế độ riêng tư | (a) Xin audit Content Posting API; (b) trong lúc chờ dùng chế độ Upload-to-inbox hoặc MANUAL_REQUIRED; (c) hoặc dùng unified API đã audit (Blotato / Ayrshare…) |
| R2. YouTube audit / verify | Project chưa verify → video upload qua API bị khoá private | Xin audit + quota extension; trong lúc chờ: upload private rồi public tay 1 click, hoặc dùng unified API |
| R3. Facebook | Chỉ đăng Reels lên **Page**, cần Page access token | Đảm bảo đang dùng Facebook Page (không phải profile cá nhân) |
| R4. Kiểm duyệt không đoán được | TikTok thường ~1 phút nhưng có thể vài giờ; YouTube tuỳ độ dài / tải | State machine + poll backoff + STUCK alert + upload sớm / hẹn giờ phía nền tảng |
| R5. Máy tắt / sleep | App desktop chỉ chạy khi máy bật → lỡ giờ đăng TikTok, gián đoạn upload / theo dõi | Tray + auto-start + trạng thái MISSED + hẹn giờ phía nền tảng cho YouTube/Facebook + resume khi khởi động |
| R6. API thay đổi | Endpoint / chính sách nền tảng đổi thường xuyên | Adapter pattern, lưu raw response, test định kỳ |
| R7. Nội dung AI | Nền tảng yêu cầu khai báo nội dung AI / chính sách nội dung tái sử dụng | Bật cờ AI-generated khi có; giữ nội dung gốc |
| R8. OAuth trên desktop | Một số nền tảng đòi redirect URI cố định / HTTPS | Dùng loopback 127.0.0.1 nếu được phép; nếu không thì dùng custom URL scheme hoặc một trang redirect nhỏ |

### Quyết định chiến lược cần chốt (chọn 1 hoặc kết hợp)

- **Option A: Tự tích hợp API chính chủ** (+ xin audit TikTok, YouTube). Free, kiểm soát hoàn toàn, nhưng mất thời gian thủ tục.
- **Option B: Dùng unified posting API trả phí** (Blotato / Ayrshare / upload-post…). Đăng public ngay, ít code, nhưng tốn phí hằng tháng và phụ thuộc bên thứ 3. Lưu ý: một số dịch vụ cần URL video public → phải upload file lên storage của họ / S3 trước.
- **Option C: Dùng Postiz làm lớp đăng bài**, app desktop chỉ lo generate + duyệt rồi gọi Postiz API. Cần chạy Postiz (Docker) hoặc dùng Postiz Cloud; tự host thì vẫn phải xin duyệt app như option A.
- **Gợi ý**: thiết kế theo adapter để bắt đầu bằng **MANUAL_REQUIRED + Facebook API** (dễ nhất), song song xin audit TikTok / YouTube; khi có quyền thì bật adapter tự động. Nếu muốn nhanh thì cắm adapter unified API.

## 10. Gợi ý kỹ thuật (để tham khảo, chưa chốt)

| Lựa chọn | Ưu | Nhược |
|---|---|---|
| **Wails** (Go + webview, UI bằng React/TS) | Backend viết bằng **Go** (thế mạnh hiện tại), UI React/TS (đã biết), file cài nhẹ | Cộng đồng nhỏ hơn Electron |
| **Tauri** (Rust + webview) | Rất nhẹ, có sẵn plugin tray / notification / autostart / updater | Core viết Rust (hoặc chạy Go/Python làm sidecar) |
| **Electron** (Node + Chromium) | Hệ sinh thái lớn, nhiều thư viện | Nặng RAM / dung lượng |

- Gợi ý: **Wails (Go + React/TS)**. Core (worker, scheduler, adapters) viết bằng Go, UI React.
- DB: **SQLite** (file local), migration bằng goose / golang-migrate.
- Job queue: bảng `jobs` trong SQLite + worker goroutine (đơn giản, đủ dùng cho 1 máy).
- Gọi make-voice-free / make-video-free: `exec` process (CLI) với input/output là file + JSON manifest; hoặc gọi HTTP nếu 2 project đó có API.
- Kiểm tra video: ffprobe.
- Thông báo: notification API của OS (qua thư viện của framework); Telegram Bot API tuỳ chọn.
- Lưu token: thư viện keyring (vd `zalando/go-keyring`).
- AI caption: LLM API (Claude / …) với prompt template theo kênh.

### Mô hình dữ liệu sơ bộ

- `scripts` (id, title, content, series, status, created_at)
- `renders` (id, script_id, voice_path, video_path, duration, resolution, status, log_path)
- `posts` (id, render_id, review_status, approved_at, scheduled_at)
- `captions` (post_id, platform, title, body, hashtags, ai_generated_flag)
- `publications` (id, post_id, platform, state, platform_ref_id, public_url, error_code, error_message, attempts, next_poll_at, stuck_since, scheduled_at, updated_at)
- `accounts` (platform, account_name, keyring_ref, expires_at)
- `jobs` (id, type, payload, state, attempts, run_after, locked_at, last_error)
- `events` (publication_id, from_state, to_state, raw_response, at): audit log
- `settings` (key, value)

## 11. Lộ trình đề xuất

| Phase | Nội dung | Kết quả |
|---|---|---|
| P0 | Chuẩn hoá make-voice-free & make-video-free thành CLI có input/output rõ ràng (vd: `--script file.md --out dir/` + manifest JSON) | Gọi được 2 project từ app |
| P1 | Khung app desktop (tray, settings, SQLite, job worker) + script → generate → caption → màn hình duyệt → "gói đăng" thủ công (MANUAL_REQUIRED) + thông báo desktop | Đã giảm phần lớn thời gian, chưa cần audit |
| P2 | Adapter Facebook Reels (tự động, có SCHEDULED) + status tracker + state machine | Facebook chạy tự động hoàn toàn |
| P3 | Adapter YouTube (upload + publishAt + poll status) | YouTube tự động (sau khi verify / audit) |
| P4 | Adapter TikTok (Direct Post FILE_UPLOAD + status fetch) + scheduler local + MISSED | TikTok tự động (sau khi audit) |
| P5 | Calendar kéo thả, batch import, tóm tắt ngày, auto-update app, (sau này) analytics | Vận hành ổn định |

## 12. Câu hỏi mở (cần trả lời trước khi viết spec chi tiết)

1. make-voice-free và make-video-free viết bằng ngôn ngữ gì? Đang chạy bằng CLI, script hay có API? Input/output hiện tại là gì?
2. Mỗi video generate mất bao lâu? Máy có GPU không?
3. Máy chạy app là Windows hay macOS? Máy có bật 24/7 không, hay chỉ bật giờ làm việc?
4. Một ngày đăng bao nhiêu clip? Có muốn đăng cùng giờ trên cả 3 nền tảng không?
5. Facebook đang đăng lên **Page** hay trang cá nhân?
6. YouTube: chỉ Shorts hay cả video dài?
7. Chọn Option A / B / C (mục 9)? Có chấp nhận trả phí unified API không?
8. Có cần duyệt kịch bản (cổng #1) không, hay chỉ duyệt video thành phẩm?
9. Có cần thumbnail / cover riêng cho từng nền tảng không?
10. Chọn framework desktop: Wails (Go) / Tauri / Electron?
11. Có cần nhận thông báo khi không ngồi máy (Telegram) không?

## 13. Tài liệu tham khảo

- TikTok Content Posting API: Direct Post, Get Post Status, Content Sharing Guidelines: https://developers.tiktok.com/doc/content-posting-api-get-started
- TikTok Get Post Status (poll / webhook, thời gian kiểm duyệt): https://developers.tiktok.com/doc/content-posting-api-reference-get-video-status
- YouTube Data API `videos.insert` & `videos` resource (status, processingDetails, publishAt): https://developers.google.com/youtube/v3/docs/videos/insert
- YouTube Quota & Compliance Audits: https://developers.google.com/youtube/data/guides/quota_and_compliance_audits
- Facebook Reels Publishing API: https://developers.facebook.com/docs/video-api/guides/reels-publishing
- Postiz (open-source scheduler): https://github.com/gitroomhq/postiz-app
- Buffer notification publishing: https://support.buffer.com/hc/en-us/articles/360039027053
- Metricool TikTok troubleshooting: https://help.metricool.com/en/article/tiktok-publishing-and-upload-troubleshooting-guide-1umbvdy/
