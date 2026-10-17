// SPDX-License-Identifier: AGPL-3.0-only

package store

import (
	"context"
	"database/sql"
	"errors"
)

// SetUserSettings stores the 16-bit settings word the dashboard's ACCOUNT
// MANAGEMENT screens (account settings, information sharing) send with
// setusersettings. The word is opaque to the server: it is kept as sent and
// handed back unchanged, never interpreted as a grant of anything.
func (s *Store) SetUserSettings(ctx context.Context, xuid string, word uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `INSERT INTO user_settings(xuid,word,updated) VALUES(?,?,?)
 ON CONFLICT(xuid) DO UPDATE SET word=excluded.word, updated=excluded.updated`, xuid, int64(word), s.unix())
	return err
}

// UserSettings returns the stored word; ok is false when none was ever sent.
func (s *Store) UserSettings(ctx context.Context, xuid string) (word uint16, ok bool, err error) {
	var w int64
	err = s.db.QueryRowContext(ctx, `SELECT word FROM user_settings WHERE xuid=?`, xuid).Scan(&w)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return uint16(w), err == nil, err
}
