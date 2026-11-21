package store

// GetSetting reads a key; ok is false when it is not set.
func (s *Store) GetSetting(key string) (value string, ok bool, err error) {
	err = s.db.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&value)
	if notFound(err) == ErrNotFound {
		return "", false, nil
	}
	return value, err == nil, err
}

// SetSetting writes a key.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}
