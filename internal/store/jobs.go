package store

import "time"

// Job is one background task (script generation, chat edit, render, preview).
type Job struct {
	ID        int64
	VideoID   int64
	Kind      string // "script" | "chat" | "render" | "preview"
	State     string // "QUEUED" | "RUNNING" | "DONE" | "FAILED" | "CANCELED"
	Error     string
	CreatedAt time.Time
}

const jobCols = `id, video_id, kind, state, error, created_at`

func scanJob(r scanner) (Job, error) {
	var j Job
	var created string
	err := r.Scan(&j.ID, &j.VideoID, &j.Kind, &j.State, &j.Error, &created)
	j.CreatedAt = parseTime(created)
	return j, err
}

// AddJob inserts j and sets its ID.
func (s *Store) AddJob(j *Job) error {
	j.CreatedAt = now()
	res, err := s.db.Exec(`INSERT INTO jobs (video_id, kind, state, error, created_at) VALUES (?,?,?,?,?)`,
		j.VideoID, j.Kind, j.State, j.Error, fmtTime(j.CreatedAt))
	if err != nil {
		return err
	}
	j.ID, err = res.LastInsertId()
	return err
}

// SetJobState updates a job's state and error.
func (s *Store) SetJobState(id int64, state, errMsg string) error {
	_, err := s.db.Exec(`UPDATE jobs SET state=?, error=? WHERE id=?`, state, errMsg, id)
	return err
}

// LastJob returns the newest job of a video.
func (s *Store) LastJob(videoID int64) (Job, error) {
	j, err := scanJob(s.db.QueryRow(`SELECT `+jobCols+` FROM jobs WHERE video_id=? ORDER BY id DESC LIMIT 1`, videoID))
	return j, notFound(err)
}

// MarkInterrupted fails every QUEUED/RUNNING job and RUNNING render (app quit mid-run).
func (s *Store) MarkInterrupted() ([]Job, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT ` + jobCols + ` FROM jobs WHERE state IN ('QUEUED','RUNNING') ORDER BY id`)
	if err != nil {
		return nil, err
	}
	out := []Job{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		j.State, j.Error = "FAILED", "interrupted"
		out = append(out, j)
	}
	rows.Close()
	if _, err := tx.Exec(`UPDATE jobs SET state='FAILED', error='interrupted' WHERE state IN ('QUEUED','RUNNING')`); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`UPDATE renders SET state='FAILED', error='interrupted', finished_at=? WHERE state='RUNNING'`, fmtTime(now())); err != nil {
		return nil, err
	}
	return out, tx.Commit()
}
