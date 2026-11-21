package store

import (
	"time"
)

// Status is the lifecycle state of a video.
type Status string

const (
	StatusDraft            Status = "DRAFT"
	StatusGeneratingScript Status = "GENERATING_SCRIPT"
	StatusScriptReview     Status = "SCRIPT_REVIEW"
	StatusRendering        Status = "RENDERING"
	StatusReady            Status = "READY"
	StatusFailed           Status = "FAILED"
)

// Video is one library entry.
type Video struct {
	ID             int64
	Slug           string
	Title          string
	Origin         string // "app" | "import"
	SourcePath     string // original mp4 for imports
	Status         Status
	StatusDetail   string
	VoiceProfile   string
	Music          string
	CurrentVersion int
	Editable       bool
	Hidden         bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

const videoCols = `id, slug, title, origin, source_path, status, status_detail, voice_profile, music,
	current_version, editable, hidden, created_at, updated_at`

type scanner interface{ Scan(...any) error }

func scanVideo(r scanner) (Video, error) {
	var v Video
	var created, updated string
	err := r.Scan(&v.ID, &v.Slug, &v.Title, &v.Origin, &v.SourcePath, &v.Status, &v.StatusDetail,
		&v.VoiceProfile, &v.Music, &v.CurrentVersion, &v.Editable, &v.Hidden, &created, &updated)
	v.CreatedAt, v.UpdatedAt = parseTime(created), parseTime(updated)
	return v, err
}

// CreateVideo inserts v and sets its ID and timestamps.
func (s *Store) CreateVideo(v *Video) error {
	t := now()
	v.CreatedAt, v.UpdatedAt = t, t
	res, err := s.db.Exec(`INSERT INTO videos (slug, title, origin, source_path, status, status_detail,
		voice_profile, music, current_version, editable, hidden, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		v.Slug, v.Title, v.Origin, v.SourcePath, v.Status, v.StatusDetail, v.VoiceProfile, v.Music,
		v.CurrentVersion, v.Editable, v.Hidden, fmtTime(t), fmtTime(t))
	if err != nil {
		return err
	}
	v.ID, err = res.LastInsertId()
	return err
}

// GetVideo loads one video.
func (s *Store) GetVideo(id int64) (Video, error) {
	v, err := scanVideo(s.db.QueryRow(`SELECT `+videoCols+` FROM videos WHERE id = ?`, id))
	return v, notFound(err)
}

// FindVideoBySource finds an imported video by its original mp4 path.
func (s *Store) FindVideoBySource(path string) (Video, error) {
	v, err := scanVideo(s.db.QueryRow(`SELECT `+videoCols+` FROM videos WHERE source_path = ? LIMIT 1`, path))
	return v, notFound(err)
}

// ListVideos returns videos, most recently updated first.
func (s *Store) ListVideos(includeHidden bool) ([]Video, error) {
	q := `SELECT ` + videoCols + ` FROM videos`
	if !includeHidden {
		q += ` WHERE hidden = 0`
	}
	rows, err := s.db.Query(q + ` ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Video{}
	for rows.Next() {
		v, err := scanVideo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// UpdateVideo saves all fields of v and bumps updated_at.
func (s *Store) UpdateVideo(v Video) error {
	_, err := s.db.Exec(`UPDATE videos SET slug=?, title=?, origin=?, source_path=?, status=?, status_detail=?,
		voice_profile=?, music=?, current_version=?, editable=?, hidden=?, updated_at=? WHERE id=?`,
		v.Slug, v.Title, v.Origin, v.SourcePath, v.Status, v.StatusDetail, v.VoiceProfile, v.Music,
		v.CurrentVersion, v.Editable, v.Hidden, fmtTime(now()), v.ID)
	return err
}

// SetStatus changes the status and its detail text.
func (s *Store) SetStatus(id int64, st Status, detail string) error {
	_, err := s.db.Exec(`UPDATE videos SET status=?, status_detail=?, updated_at=? WHERE id=?`,
		st, detail, fmtTime(now()), id)
	return err
}

// DeleteVideo removes a video and (by cascade) its versions, renders and jobs.
func (s *Store) DeleteVideo(id int64) error {
	_, err := s.db.Exec(`DELETE FROM videos WHERE id=?`, id)
	return err
}
