package store

import (
	"database/sql"
	"time"
)

// Render is one run of the pipeline (full render or frame preview).
type Render struct {
	ID         int64
	VideoID    int64
	Version    int
	Kind       string // "render" | "preview"
	MP4Path    string
	ReportPath string
	LogPath    string
	State      string // "RUNNING" | "DONE" | "FAILED" | "CANCELED"
	Error      string
	Seconds    float64
	StartedAt  time.Time
	FinishedAt *time.Time
}

const renderCols = `id, video_id, version, kind, mp4_path, report_path, log_path, state, error, seconds, started_at, finished_at`

func scanRender(r scanner) (Render, error) {
	var x Render
	var started string
	var finished sql.NullString
	err := r.Scan(&x.ID, &x.VideoID, &x.Version, &x.Kind, &x.MP4Path, &x.ReportPath, &x.LogPath, &x.State,
		&x.Error, &x.Seconds, &started, &finished)
	x.StartedAt = parseTime(started)
	if finished.Valid {
		t := parseTime(finished.String)
		x.FinishedAt = &t
	}
	return x, err
}

// AddRender inserts r and sets its ID and StartedAt (if zero).
func (s *Store) AddRender(r *Render) error {
	if r.StartedAt.IsZero() {
		r.StartedAt = now()
	}
	var finished any
	if r.FinishedAt != nil {
		finished = fmtTime(*r.FinishedAt)
	}
	res, err := s.db.Exec(`INSERT INTO renders (video_id, version, kind, mp4_path, report_path, log_path, state,
		error, seconds, started_at, finished_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		r.VideoID, r.Version, r.Kind, r.MP4Path, r.ReportPath, r.LogPath, r.State, r.Error, r.Seconds,
		fmtTime(r.StartedAt), finished)
	if err != nil {
		return err
	}
	r.ID, err = res.LastInsertId()
	return err
}

// FinishRender sets the final state of a render.
func (s *Store) FinishRender(id int64, state, errMsg string, seconds float64) error {
	_, err := s.db.Exec(`UPDATE renders SET state=?, error=?, seconds=?, finished_at=? WHERE id=?`,
		state, errMsg, seconds, fmtTime(now()), id)
	return err
}

// SetRenderPaths sets the files of a render (they are named after its ID).
func (s *Store) SetRenderPaths(id int64, mp4, report, log string) error {
	_, err := s.db.Exec(`UPDATE renders SET mp4_path=?, report_path=?, log_path=? WHERE id=?`, mp4, report, log, id)
	return err
}

// GetRender loads one render.
func (s *Store) GetRender(id int64) (Render, error) {
	r, err := scanRender(s.db.QueryRow(`SELECT `+renderCols+` FROM renders WHERE id=?`, id))
	return r, notFound(err)
}

// ListRenders returns a video's renders, newest first.
func (s *Store) ListRenders(videoID int64) ([]Render, error) {
	rows, err := s.db.Query(`SELECT `+renderCols+` FROM renders WHERE video_id=? ORDER BY started_at DESC, id DESC`, videoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Render{}
	for rows.Next() {
		r, err := scanRender(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// LatestRender returns the newest render of the given kind.
func (s *Store) LatestRender(videoID int64, kind string) (Render, error) {
	r, err := scanRender(s.db.QueryRow(`SELECT `+renderCols+` FROM renders WHERE video_id=? AND kind=?
		ORDER BY started_at DESC, id DESC LIMIT 1`, videoID, kind))
	return r, notFound(err)
}
