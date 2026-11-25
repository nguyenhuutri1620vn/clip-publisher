# Ảnh thật khớp lời đọc (phần app) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cho chủ kênh tìm ảnh Wikimedia Commons theo từ khóa Claude đề xuất cho từng cảnh, chọn một ảnh, và ghi nguồn vào script, theo thiết kế.

**Architecture:** Package mới `internal/photos` (tìm, lọc giấy phép, tải, lưu vào `photos/`), `scriptmodel` thêm `WithSlideSource`, `scriptgen` và `notes` nhận `photo_queries`, `service` nối các bước và đi qua đường lưu phiên bản có sẵn (`saveManual`), frontend thêm nút "Tìm ảnh" trên thẻ cảnh.

**Tech Stack:** Go 1.23 (stdlib `net/http`, `encoding/json`, `crypto/sha256`), React/TypeScript, Wails v2 bindings.

**Spec:** `docs/superpowers/specs/2026-11-25-real-photo-scenes-design.md`

**Out of scope:** layer `photo` trong `make-video-free` (repo khác, không sửa). Sau khi làm xong, ảnh đã chọn được ghi nguồn và lưu, nhưng chưa hiện trong video cho tới khi layer đó có.

## Global Constraints

- Không thêm dependency mới (CLAUDE.md, quy tắc 7). Chỉ dùng Go stdlib và package đã có.
- Không `exec` và không shell. Phần này không gọi tiến trình ngoài.
- Không ghi đè file. Mọi thay đổi script đi qua `saveManual` (phiên bản mới), file ghi bằng `workspace.WriteFileAtomic`.
- Giấy phép cho phép, đúng chuỗi `LicenseShortName` của Commons: `CC0`, `Public domain`, `CC BY x.x`, `CC BY-SA x.x`. Loại mọi giá trị có `NC` hoặc `ND`.
- Giới hạn tải: 10 * 1024 * 1024 byte. Chỉ chấp nhận `image/jpeg` và `image/png`.
- Timeout HTTP: 15 s cho mỗi request. User-Agent cố định: `clip-publisher/1.0 (desktop; no-contact)`, không chứa thông tin người dùng.
- Lỗi hiển thị cho người dùng viết tiếng Việt. Code, tên biến, commit message tiếng Anh.
- Thẻ cảnh không được đổi `title`, `subtitle`, `narration`, `id` khi thêm nguồn.

## Review Focus

1. Commons không trả kết quả nào: trả danh sách rỗng và thông báo "Không tìm thấy ảnh cho “…”. Thử từ khóa khác." (không phải lỗi).
2. Ảnh có giấy phép NC hoặc ND trong kết quả: bị loại, không hiện trong danh sách.
3. Ảnh vượt 10 MB hoặc không phải JPEG/PNG (ví dụ SVG, OGG): báo lỗi tiếng Việt, không ghi file nào.
4. Mất mạng hoặc Commons trả HTTP 5xx: báo "Không kết nối được Wikimedia Commons…", không làm hỏng script.
5. Chọn lại cùng một ảnh cho cùng cảnh hoặc cảnh khác: không thêm dòng nguồn trùng, không tạo file trùng.
6. Tác giả trong metadata Commons có HTML (`<a href=…>`): loại thẻ HTML trước khi ghi vào `sources`.

Mỗi dòng trên có test ở task sở hữu code (xem Task 2, 4).

---

### Task 1: `scriptmodel.WithSlideSource`

**Files:**
- Modify: `internal/scriptmodel/script.go` (thêm hàm, đổi nhỏ `render` để nhận `top` đã sửa)
- Test: `internal/scriptmodel/script_test.go`

**Interfaces:**
- Consumes: `Script`, `object`, `parseObject` (đã có)
- Produces: `func (s *Script) WithSlideSource(id, credit string) ([]byte, error)`. Thêm `credit` vào cuối `sources` (bỏ qua nếu đã có chuỗi y hệt, trả về script không đổi), và nếu cảnh `id` chưa có `source` thì đặt `claim: true` và `source` = chỉ số của `credit`. Nếu cảnh đã có `source`, giữ nguyên.

- [ ] **Step 1: Write the failing test**

```go
func TestWithSlideSourceAddsCreditOnce(t *testing.T) {
	s, _ := Parse(sampleScript(t)) // dùng testdata/sample.json có sẵn; slide "s1" chưa có source
	out, err := s.WithSlideSource("s1", "Ảnh: Tác giả X, CC BY-SA 4.0, https://commons.wikimedia.org/wiki/File:A.jpg")
	if err != nil { t.Fatal(err) }
	s2, _ := Parse(out)
	if len(s2.Sources) != len(s.Sources)+1 { t.Fatalf("sources: want +1, got %d", len(s2.Sources)) }
	// lần 2 cùng credit: không thêm nữa
	out2, _ := s2.WithSlideSource("s1", s2.Sources[len(s2.Sources)-1])
	s3, _ := Parse(out2)
	if len(s3.Sources) != len(s2.Sources) { t.Fatalf("duplicate credit added") }
}
```

(`sampleScript(t)` là helper đọc `testdata/sample.json`; nếu chưa có thì viết trong test.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scriptmodel -run TestWithSlideSourceAddsCreditOnce -v`
Expected: FAIL với "undefined: WithSlideSource"

- [ ] **Step 3: Implement `WithSlideSource` in `internal/scriptmodel/script.go`**

Dựa trên `WithSlideText`: lấy `s.index(id)`, copy slides bằng `copySlides`. Đọc `sources` từ `s.top.vals["sources"]` (mảng chuỗi, có thể vắng), kiểm tra trùng, nối `credit`, đặt lại bằng `top.set("sources", list)`. Lưu ý `render` hiện chỉ đặt lại `slides`, nên tách phần ghép `top` thành hàm nhận `top` và `slides` để dùng chung.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scriptmodel -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/scriptmodel/script.go internal/scriptmodel/script_test.go
git commit -m "feat: add WithSlideSource to cite a photo in script sources"
```

---

### Task 2: package `internal/photos` (tìm, lọc giấy phép, tải, lưu)

**Files:**
- Create: `internal/photos/commons.go` (tìm kiếm + lọc giấy phép + làm sạch HTML)
- Create: `internal/photos/fetch.go` (tải có giới hạn, kiểm MIME, lưu file + `index.json`)
- Test: `internal/photos/photos_test.go` (dùng `httptest.Server`, không gọi Commons thật)

**Interfaces:**
- Produces:
  ```go
  type Candidate struct {
      Title, Author, License, LicenseURL, PageURL, ThumbURL, FullURL string
      Width, Height int
  }
  type Client struct { HTTP *http.Client; BaseURL string } // BaseURL mặc định "https://commons.wikimedia.org/w/api.php"
  func (c *Client) Search(ctx context.Context, query string, limit int) ([]Candidate, error)
  func (c *Client) Download(ctx context.Context, cand Candidate, dir string) (File, error)
  type File struct { Rel string; SHA256 string; Credit string } // Rel là đường dẫn tương đối từ thư mục video, ví dụ "photos/<sha>.jpg"
  func AllowedLicense(short string) bool
  func StripHTML(s string) string
  func Credit(c Candidate) string // "Ảnh: <author>, <license>, <pageURL>"
  func Index(dir string) (map[string][]Ref, error)      // đọc photos/index.json, key là slide id
  func AddRef(dir, slideID string, ref Ref) error       // ghi lại index.json bằng WriteFileAtomic
  type Ref struct { SHA256, File, Credit, PageURL string }
  ```
- Consumes: `workspace.WriteFileAtomic` (đã có)

- [ ] **Step 1: Write failing tests**

Các test cần có (tên và assertion chính, viết đầy đủ khi triển khai):
- `TestSearchFiltersLicenses`: server trả 3 kết quả với `LicenseShortName` lần lượt `"CC BY-SA 4.0"`, `"CC BY-NC 2.0"`, `"Public domain"`. Kỳ vọng `len == 2` và không có kết quả NC.
- `TestSearchEmptyIsNotError`: `query` không có kết quả → `err == nil`, `len == 0`.
- `TestSearchHTTPErrorIsVietnamese`: server trả 503 → `err` chứa "Không kết nối được Wikimedia Commons".
- `TestStripHTMLArtist`: `StripHTML("<a href=\"x\">Tác giả</a>")` == `"Tác giả"`.
- `TestDownloadRejectsLargeAndNonImage`: ảnh > 10 MB hoặc `Content-Type: image/svg+xml` → lỗi, không có file trong `dir/photos`.
- `TestDownloadWritesSHAName`: JPEG hợp lệ → file tên `<sha256>.jpg`, `File.SHA256` khớp.
- `TestAddRefIsIdempotentPerSHA`: gọi `AddRef` hai lần cùng `SHA256` cho cùng slide → chỉ một `Ref`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/photos -v`
Expected: FAIL (package chưa có hàm nào)

- [ ] **Step 3: Implement**

- `Search`: gọi `action=query&generator=search&gsrnamespace=6&gsrlimit=<limit>&prop=imageinfo&iiprop=url|size|mime|extmetadata&iiurlwidth=640&format=json` với `gsrsearch=query`. Đọc `extmetadata.LicenseShortName.value`, `Artist.value`, `LicenseUrl.value`. Lọc bằng `AllowedLicense`, và chỉ giữ MIME `image/jpeg` hoặc `image/png`.
- `AllowedLicense`: trả `true` khi chuỗi bắt đầu bằng `CC0`, `Public domain`, hoặc `CC BY ` / `CC BY-SA ` và không chứa `NC`/`ND`.
- `Download`: `GET cand.FullURL` với `io.LimitReader(resp.Body, 10<<20+1)`; nếu đọc được hơn 10 MB thì lỗi. MIME từ `Content-Type` phải là JPEG hoặc PNG, đuôi tương ứng `.jpg`/`.png`. Ghi bằng `workspace.WriteFileAtomic` vào `dir/photos/<sha256>.<ext>` (nếu file đã có thì không ghi lại).
- `AddRef`: đọc `photos/index.json` (map `slideID → []Ref`), thêm ref nếu chưa có cùng `SHA256` cho slide đó, ghi lại atomic.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/photos -v && go vet ./internal/photos`
Expected: PASS, không có cảnh báo vet

- [ ] **Step 5: Commit**

```bash
git add internal/photos
git commit -m "feat: add photos package to search, filter and store Commons images"
```

---

### Task 3: Claude đề xuất `photo_queries` theo cảnh

**Files:**
- Modify: `internal/scriptmodel/notes.go` (thêm field vào `SlideNote`)
- Modify: `internal/scriptgen/prompt.go` (thêm một dòng quy tắc)
- Test: `internal/scriptgen/scriptgen_test.go`, `internal/scriptmodel` (test parse notes)

**Interfaces:**
- Produces: `SlideNote` có thêm `PhotoQueries []string \`json:"photo_queries,omitempty"\``. Prompt yêu cầu: với cảnh có "Hình ảnh" cần ảnh thật, trả 1–3 từ khóa tiếng Anh trong `notes.slides.<id>.photo_queries`.

- [ ] **Step 1: Write failing tests**

- `TestParseNotesPhotoQueries`: JSON `{"slides":{"s1":{"note":"cần ảnh thật","photo_queries":["Ngoc Lu bronze drum"]}}}` → `n.Slides["s1"].PhotoQueries[0] == "Ngoc Lu bronze drum"`.
- `TestPromptAsksForPhotoQueries`: `BuildPrompt("", nil, Request{Input: "x"})` chứa chuỗi `photo_queries`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/scriptmodel ./internal/scriptgen -run 'PhotoQueries' -v`
Expected: FAIL

- [ ] **Step 3: Implement**

Thêm field vào `SlideNote`. Thêm vào `rules` trong `prompt.go` một dòng: `- When a slide needs a real photo, also give notes.slides.<id>.photo_queries: 1–3 short English search terms for Wikimedia Commons that match the narration.` Không đổi phần còn lại của prompt.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/scriptmodel ./internal/scriptgen -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/scriptmodel/notes.go internal/scriptgen/prompt.go internal/scriptgen/scriptgen_test.go
git commit -m "feat: ask Claude for photo search terms per slide"
```

---

### Task 4: service — tìm và chọn ảnh

**Files:**
- Modify: `internal/service/service.go` (`Deps` thêm `Photos *photos.Client`; mặc định dùng `&photos.Client{HTTP: &http.Client{Timeout: 15*time.Second}}` nếu nil)
- Create: `internal/service/photos.go`
- Modify: `internal/service/dto.go` (`SlideDTO` thêm `PhotoQueries []string` và `Photos []photos.Ref`)
- Modify: `internal/service/query.go` (điền `PhotoQueries` và `Photos` trong `Detail`)
- Test: `internal/service/photos_test.go` (dùng `httptest.Server` làm Commons giả, `Deps.Photos.BaseURL` trỏ tới đó)

**Interfaces:**
- Consumes: `photos.Client.Search`, `Download`, `AddRef`, `Credit`; `scriptmodel.Script.WithSlideSource`; `Service.editable`, `Service.current`, `Service.saveManual` (đã có)
- Produces:
  ```go
  func (s *Service) FindPhotos(videoID int64, query string) ([]photos.Candidate, error)
  func (s *Service) PickPhoto(videoID int64, slideID string, cand photos.Candidate) error
  ```
  `FindPhotos` không sửa gì và không cần khóa video. `PickPhoto` giữ `lockVideo`, gọi `editable`, tải và lưu file vào `videoDir`, rồi gọi `WithSlideSource` và `saveManual(v, out, "manual", "Thêm ảnh cho cảnh "+slideID)`. Sau đó gọi `photos.AddRef`. Thứ tự: tải và ghi file trước, rồi mới lưu phiên bản; nếu lưu phiên bản lỗi thì file ảnh vẫn còn nhưng `index.json` không được cập nhật.

- [ ] **Step 1: Write failing tests**

- `TestFindPhotosReturnsOnlyAllowed`: fake Commons trả 2 ảnh (một CC BY-SA, một NC) → `FindPhotos` trả 1 ảnh.
- `TestPickPhotoAddsSourceAndVersion`: tạo video với script mẫu, gọi `PickPhoto` với candidate từ fake server → video có version mới, `Sources` có thêm dòng chứa tác giả và giấy phép, file có trong `photos/`, `photos/index.json` có slide id.
- `TestPickPhotoTwiceNoDuplicateSource`: chọn cùng ảnh hai lần → số dòng `sources` không tăng ở lần 2.
- `TestPickPhotoRejectsNonImage`: fake server trả `image/svg+xml` → lỗi chứa "Ảnh không hợp lệ", không có version mới.
- `TestDetailShowsPhotoQueries`: `notes` có `photo_queries` → `Detail` trả `Slides[i].PhotoQueries`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service -run 'Photo' -v`
Expected: FAIL (chưa có `FindPhotos`, `PickPhoto`)

- [ ] **Step 3: Implement**

- `FindPhotos`: `query` đã trim, rỗng thì lỗi "Hãy nhập từ khóa". Gọi `s.d.Photos.Search(ctx, query, 5)` với ctx timeout 15 s.
- `PickPhoto`: theo thứ tự ở phần Interfaces. Lỗi tải trả về nguyên văn lỗi tiếng Việt của package `photos`. Ảnh tải về dùng `photos.Credit(cand)` làm dòng nguồn.
- `Detail`: với mỗi cảnh, đọc `notes.Slides[sl.ID].PhotoQueries` và `photos.Index(dir)[sl.ID]`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service ./internal/photos ./internal/scriptmodel ./internal/scriptgen -v && go vet ./...`
Expected: PASS, vet sạch

- [ ] **Step 5: Commit**

```bash
git add internal/service
git commit -m "feat: find Commons photos for a slide and cite the chosen one"
```

---

### Task 5: Wails bindings

**Files:**
- Modify: `app.go` (thêm hai method)
- Regenerate: `frontend/wailsjs` bằng `wails generate module`

**Interfaces:**
- Produces (JS): `App.FindPhotos(id: number, query: string): Promise<photos.Candidate[]>`, `App.PickPhoto(id: number, slideID: string, cand: photos.Candidate): Promise<void>`

- [ ] **Step 1: Add methods**

```go
func (a *App) FindPhotos(id int64, query string) ([]photos.Candidate, error) {
	var out []photos.Candidate
	err := a.call(func() error {
		var e error
		out, e = a.svc.FindPhotos(id, query)
		return e
	})
	return out, err
}

func (a *App) PickPhoto(id int64, slideID string, cand photos.Candidate) error {
	return a.call(func() error { return a.svc.PickPhoto(id, slideID, cand) })
}
```

- [ ] **Step 2: Regenerate bindings and build**

Run: `wails generate module && go build ./...`
Expected: không lỗi; `frontend/wailsjs/go/main/App.d.ts` có `FindPhotos` và `PickPhoto`

- [ ] **Step 3: Commit**

```bash
git add app.go frontend/wailsjs
git commit -m "feat: bind FindPhotos and PickPhoto to the UI"
```

---

### Task 6: Frontend — nút "Tìm ảnh" trên thẻ cảnh

**Files:**
- Modify: `frontend/src/components/ScriptReview.tsx` (`SlideCard`)
- Modify: `frontend/src/style.css` (lưới ảnh nhỏ)

**Interfaces:**
- Consumes: `api.FindPhotos`, `api.PickPhoto`, `SlideDTO.photoQueries`, `SlideDTO.photos`
- Produces: một panel trong thẻ cảnh gồm ô từ khóa (điền sẵn `photoQueries[0]`), nút "Tìm", lưới tối đa 5 ảnh (thumb, tác giả, giấy phép, nút "Chọn"). Sau khi chọn thì đóng panel, `act(...)` làm mới chi tiết video.

- [ ] **Step 1: Implement**

- Nút "Tìm ảnh" ở phần meta của thẻ, `disabled={busy}`.
- Khi mở: ô nhập `query` điền sẵn, nút "Tìm" gọi `api.FindPhotos(videoId, query)`. Trạng thái loading "Đang tìm…".
- Kết quả rỗng: hiện "Không tìm thấy ảnh cho “…”. Thử từ khóa khác."
- Chọn ảnh: `act(api.PickPhoto(videoId, s.id, cand))`, lỗi hiện trong `banner error`.
- Dưới thẻ: liệt kê `s.photos` với tác giả và giấy phép.

- [ ] **Step 2: Verify**

Run: `cd frontend && npm run build`
Expected: build thành công, không lỗi TypeScript

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/ScriptReview.tsx frontend/src/style.css
git commit -m "feat: photo search panel on each slide card"
```

---

### Task 7: Kiểm tra tổng

- [ ] **Step 1: Chạy toàn bộ kiểm thử**

Run: `go test ./... && go vet ./... && (cd frontend && npm run build)`
Expected: tất cả PASS, không lỗi vet, build frontend thành công.

- [ ] **Step 2: Kiểm tra tay (tùy chọn, cần Commons thật)**

Mở một video có cảnh ghi "cần ảnh thật", bấm "Tìm ảnh", chọn một ảnh, kiểm tra dòng nguồn mới và thư mục `photos/`. Không gọi Commons trong test tự động.

- [ ] **Step 3: Ghi chú còn lại**

Ảnh đã chọn chưa hiện trong video do thiếu layer `photo` ở `make-video-free`. Ghi điều này vào báo cáo.

---

## Self-review

- Spec coverage: mục 4 (1–6) → Task 3, 4, 6; mục 5 (dữ liệu) → Task 2, 4; mục 6 (blocker) ghi ở Out of scope; mục 7 (quyết định) → Global Constraints; mục 9 (kiểm thử) → Task 1–4, 7.
- Type consistency: `Candidate`, `Ref`, `File`, `Client.Search/Download`, `AddRef`, `WithSlideSource` dùng cùng tên xuyên suốt. `FindPhotos`/`PickPhoto` khớp giữa Task 4 và 5.
