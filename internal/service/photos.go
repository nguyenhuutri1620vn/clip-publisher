package service

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"time"

	"clip-publisher/internal/photos"
)

// photoTimeout bounds one Commons request (search or download).
const photoTimeout = 15 * time.Second

// FindPhotos searches Wikimedia Commons for query. No result is an empty list, not an error.
func (s *Service) FindPhotos(videoID int64, query string) ([]photos.Candidate, error) {
	if _, err := s.d.Store.GetVideo(videoID); err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("Hãy nhập từ khóa")
	}
	ctx, cancel := context.WithTimeout(context.Background(), photoTimeout)
	defer cancel()
	return s.d.Photos.Search(ctx, query, 5)
}

// PickPhoto downloads cand into the video folder, cites it in the script, saves a new version and
// records the photo for slideID. The file is written before the version is saved, so a failed save
// leaves a stray file but never a version without its file.
func (s *Service) PickPhoto(videoID int64, slideID string, cand photos.Candidate) error {
	defer s.lockVideo(videoID)()
	v, err := s.editable(videoID)
	if err != nil {
		return err
	}
	raw, sc, _, err := s.current(v)
	if err != nil {
		return err
	}
	dir := s.videoDir(v)
	ctx, cancel := context.WithTimeout(context.Background(), photoTimeout)
	defer cancel()
	file, err := s.d.Photos.Download(ctx, cand, dir)
	if err != nil {
		return err
	}
	out, err := sc.WithSlideSource(slideID, file.Credit)
	if err != nil {
		return err
	}
	if !bytes.Equal(out, raw) {
		if err := s.saveManual(v, out, "manual", "Thêm ảnh cho cảnh "+slideID); err != nil {
			return err
		}
	}
	return photos.AddRef(dir, slideID, photos.Ref{SHA256: file.SHA256, File: file.Rel, Credit: file.Credit, PageURL: cand.PageURL})
}
