// SPDX-License-Identifier: AGPL-3.0-only

package store

import (
	"context"

	"github.com/openpak/xb-account/internal/pgdb"
)

// eraseTx deletes every row of one core account: Xbox identities (users,
// gamertag aliases and audit, user settings, auto identity), consoles
// (devices, provisions, enrolments, vouchers) and the gamertag reservation.
// With keepRevocations the account's revocation entries stay: the gateways
// still have to read them to drop live sessions (the event path appends one
// just after this).
func eraseTx(ctx context.Context, t *pgdb.Tx, account string, keepRevocations bool) (int, error) {
	const xuids = `SELECT xuid FROM users WHERE core_account_id=? UNION SELECT xuid FROM auto_identities WHERE core_account_id=?`
	stmts := []struct {
		q    string
		args []any
	}{
		{`DELETE FROM gamertag_aliases WHERE xuid IN (` + xuids + `)`, []any{account, account}},
		{`DELETE FROM user_settings WHERE xuid IN (` + xuids + `)`, []any{account, account}},
		{`DELETE FROM users WHERE core_account_id=?`, []any{account}},
		{`DELETE FROM auto_identities WHERE core_account_id=?`, []any{account}},
		{`DELETE FROM gamertag_audit WHERE core_account_id=?`, []any{account}},
		{`DELETE FROM name_reservations WHERE core_account_id=?`, []any{account}},
		{`DELETE FROM devices WHERE core_account_id=?`, []any{account}},
		{`DELETE FROM provisions WHERE core_account_id=?`, []any{account}},
		{`DELETE FROM enrolments WHERE core_account_id=?`, []any{account}},
		{`DELETE FROM vouchers WHERE core_account_id=?`, []any{account}},
	}
	if !keepRevocations {
		stmts = append(stmts, struct {
			q    string
			args []any
		}{`DELETE FROM revocations WHERE core_account_id=?`, []any{account}})
	}
	total := 0
	for _, s := range stmts {
		r, err := t.ExecContext(ctx, s.q, s.args...)
		if err != nil {
			return 0, err
		}
		n, _ := r.RowsAffected()
		total += int(n)
	}
	return total, nil
}

// EraseAccount deletes everything xb-account holds for the core account,
// including its revocation entries, in one transaction. Idempotent.
func (s *Store) EraseAccount(ctx context.Context, account string) (int, error) {
	var n int
	err := s.tx(ctx, func(t *pgdb.Tx) (err error) {
		n, err = eraseTx(ctx, t, account, false)
		return
	})
	return n, err
}
