# Ảnh thật khớp lời đọc (hướng B) — thiết kế

Trạng thái: **chờ chủ kênh duyệt** (chưa viết code sản phẩm).
Ngày: 2026-11-25. Liên quan: `docs/prd.md` F3.3, F3.5, mục 9 (phase 2 "Ảnh thật").

## 1. Vấn đề

Mục "Hình ảnh" trong kịch bản đang được ánh xạ sang catalog layer vẽ thủ tục của
`make-video-free` (~28 layer: `sun`, `clouds`, `peaks`, `drum`, `map`, …). Catalog đó
chỉ có hình vẽ gốc (C-06 của repo đó), nên khi lời đọc nói tới một hiện vật, nhân vật
hay địa danh cụ thể thì Claude chỉ chọn được layer gần nhất. Kết quả: nhiều cảnh lặp
lại cùng vài hình, không khớp nội dung.

## 2. Mục tiêu

- Mỗi cảnh cần ảnh cụ thể (theo lời đọc) có một ảnh thật, được chọn có chủ đích.
- Mỗi ảnh có nguồn: tác giả, giấy phép, link trang gốc. Ảnh đó được ghi vào `sources`.
- Không dùng ảnh AI làm bằng chứng lịch sử (PRD, mục 9).

## 3. Không làm (non-goals)

- Không sửa `make-video-free` trong thiết kế này. Layer `photo` là việc của repo đó
  (xem mục 6).
- Không tự tải ảnh mà không có chủ kênh chọn.
- Không sinh ảnh bằng AI.
- Không gỡ nhãn `[CẦN KIỂM TRA]` hay cờ `claim` (CLAUDE.md, quy tắc 4).

## 4. Hành vi

1. **Đề xuất từ khóa.** Với cảnh có note "cần ảnh thật" (hoặc cảnh mà chủ kênh bấm
   "Tìm ảnh"), Claude trả thêm `photo_queries` trong JSON phản hồi: 1–3 từ khóa tiếng
   Anh và tiếng Việt, kèm lý do ngắn bám vào lời đọc. Ví dụ: lời đọc nhắc "trống đồng
   Ngọc Lũ" → `Ngoc Lu bronze drum`.
2. **Tìm ảnh.** App gọi API của Wikimedia Commons (HTTPS, `net/http` của Go stdlib,
   không thêm dependency). Lấy 3–5 kết quả có ảnh xem trước, kèm tác giả, giấy phép,
   link trang gốc.
3. **Chọn.** Chủ kênh bấm một ảnh, hoặc kéo thả ảnh của mình. Không có ảnh nào được
   tải về khi chưa chọn.
4. **Lưu.** Ảnh tải về vào `~/ClipPublisher/videos/<slug>/photos/<sha256>.<ext>`, ghi
   bằng `WriteFileAtomic`. Mỗi lần chọn tạo một bản ghi nguồn (xem mục 5).
5. **Ghi nguồn.** Thêm vào `sources` của script một dòng: tác giả, tên tệp, giấy phép,
   URL trang gốc. Cảnh đó nhận `claim: true` và `source` trỏ tới dòng này. Nếu phiên
   bản script mới được tạo thì ghi như mọi phiên bản khác (không ghi đè).
6. **Giấy phép chấp nhận.** Chỉ CC0, public domain, CC BY, CC BY-SA. Loại NC và ND
   (xem mục 7).

## 5. Dữ liệu

- File trong workspace: `videos/<slug>/photos/<sha256>.<ext>` và
  `videos/<slug>/photos/index.json` (danh sách ảnh đã chọn: sha256, nguồn, tác giả,
  giấy phép, URL gốc, ngày tải).
- Không thêm bảng SQLite. `index.json` đủ cho MVP này.
- Hợp đồng với renderer: cảnh dùng ảnh sẽ có `scene.layers` chứa một layer `photo`
  với đường dẫn tới file ảnh. **Tên trường và định dạng chưa xác định**, phụ thuộc
  spec `photo` ở `make-video-free`.

## 6. Phụ thuộc ngoài

- **Blocker: layer `photo` ở `make-video-free`.** Không có layer này thì ảnh đã chọn
  không hiện được trong video. Phần app có thể làm trước (tìm, chọn, tải, ghi nguồn,
  hiện ảnh trong thẻ cảnh), nhưng render thật cần layer đó. Việc này cần chủ kênh
  quyết định: tự làm ở repo đó, hoặc duyệt spec để một phiên khác làm.
- Wikimedia Commons: API công khai, cần gửi User-Agent chung chung (không dùng tên
  hay email của chủ kênh, theo quy tắc riêng tư).

## 7. Các quyết định tôi đã chọn (chủ kênh có thể đổi)

- Giấy phép: chỉ CC0, public domain, CC BY, CC BY-SA. Loại NC và ND vì kênh có thể
  kiếm tiền và cần cắt/zoom ảnh.
- Số kết quả: 5 mỗi lần tìm.
- Ảnh tải về không vượt 10 MB; quá thì báo lỗi, không tải.
- Tìm ảnh chạy trong job riêng, không chặn UI. Lỗi mạng hiện bằng tiếng Việt.
- Thử lại, không tự chọn thay chủ kênh.

## 8. Các phương án đã cân nhắc

- **A. Tự kéo thả ảnh của mình.** Chính xác nhất, nhưng chủ kênh tự tìm ảnh cho mỗi cảnh.
  Được giữ làm đường phụ trong thiết kế (mục 4 bước 3).
- **B. Wikimedia với từ khóa do Claude đề xuất (chọn).** Gắn ảnh với lời đọc, có nguồn
  và giấy phép rõ ràng, đúng hướng PRD phase 2.
- **C. Giữ hình vẽ thủ tục, chỉ đánh dấu cảnh không khớp.** Không sửa được việc khớp
  nội dung.

## 9. Kiểm thử

- Go unit test: phân tích phản hồi tìm kiếm (fake HTTP server), tải có giới hạn kích
  thước, ghi `index.json` atomic, thêm dòng `sources` không đụng cảnh khác.
- Không gọi Wikimedia thật trong test tự động.
- Kiểm tra tay: một cảnh có từ khóa, chọn ảnh, xem thẻ cảnh và nguồn.

## 10. Câu hỏi còn mở cho chủ kênh

1. Layer `photo` ở `make-video-free`: bạn tự làm hay duyệt spec để làm riêng?
2. Danh sách giấy phép ở mục 7 có đúng với bạn không?
3. Có cần kéo thả ảnh của chính mình ngay trong MVP này (đường A), hay để sau?
