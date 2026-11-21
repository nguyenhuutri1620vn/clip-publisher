package store

import "time"

// Version is one saved script.json of a video.
type Version struct {
	VideoID    int64
	N          int
	ScriptPath string
	NotesPath  string
	Author     string // "claude" | "manual" | "import" | "restore"
	Summary    string
	CreatedAt  time.Time
}

// AddVersion inserts a version; (video, N) must be unique.
func (s *Store) AddVersion(v Version) error {
	if v.CreatedAt.IsZero() {
		v.CreatedAt = now()
	}
	_, err := s.db.Exec(`INSERT INTO versions (video_id, n, script_path, notes_path, author, summary, created_at)
		VALUES (?,?,?,?,?,?,?)`, v.VideoID, v.N, v.ScriptPath, v.NotesPath, v.Author, v.Summary, fmtTime(v.CreatedAt))
	return err
}

func scanVersion(r scanner) (Version, error) {
	var v Version
	var created string
	err := r.Scan(&v.VideoID, &v.N, &v.ScriptPath, &v.NotesPath, &v.Author, &v.Summary, &created)
	v.CreatedAt = parseTime(created)
	return v, err
}

const versionCols = `video_id, n, script_path, notes_path, author, summary, created_at`

// ListVersions returns versions in ascending N.
func (s *Store) ListVersions(videoID int64) ([]Version, error) {
	rows, err := s.db.Query(`SELECT `+versionCols+` FROM versions WHERE video_id=? ORDER BY n`, videoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Version{}
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// GetVersion loads version n of a video.
func (s *Store) GetVersion(videoID int64, n int) (Version, error) {
	v, err := scanVersion(s.db.QueryRow(`SELECT `+versionCols+` FROM versions WHERE video_id=? AND n=?`, videoID, n))
	return v, notFound(err)
}
