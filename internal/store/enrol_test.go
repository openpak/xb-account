// SPDX-License-Identifier: AGPL-3.0-only

package store

import (
	"bytes"
	"context"
	"testing"

	"github.com/openpak/xb-account/internal/pgdb"
	"github.com/openpak/xb-common/xbaccountapi"
)

// State written by one instance is read by a new instance on the same database
// (restart), and opening twice is idempotent.
func TestStateSurvivesRestart(t *testing.T) {
	dsn := pgdb.TestDSN(t)
	ctx := context.Background()
	pepper := bytes.Repeat([]byte{1}, 32)
	s1, err := Open(dsn, pepper)
	if err != nil {
		t.Fatal(err)
	}
	p := Provision{ID: "p1", CoreAccountID: "acct", MachineID: "0000001cbe991a14", XUID: "0009000000000001", LinkID: "l1",
		Gamertag: "Tag One", ProofMAC: []byte{1, 2}, ProofSealed: []byte{3, 4}}
	if err := s1.CreateProvision(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := s1.CommitProvision(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := s1.SetUserSettings(ctx, p.XUID, 0x1234); err != nil {
		t.Fatal(err)
	}
	if err := s1.ApplyChange(ctx, Change{EventID: "e1", Version: 7,
		Emit: &xbaccountapi.Revocation{Scope: "user", PrincipalID: p.XUID, CoreAccountID: "acct", Reason: "banned"}}); err != nil {
		t.Fatal(err)
	}
	s1.Close()

	s2, err := Open(dsn, pepper)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	d, err := s2.GetDevice(ctx, p.MachineID)
	if err != nil || d.Signup != SignupComplete || d.CoreAccountID != "acct" || d.Status != xbaccountapi.StatusActive {
		t.Fatalf("device after restart: %+v %v", d, err)
	}
	u, err := s2.GetUserByGamertag(ctx, "tag one")
	if err != nil || u.XUID != p.XUID {
		t.Fatalf("user after restart: %+v %v", u, err)
	}
	if ok, err := s2.VerifyDeviceProof(ctx, p.MachineID, nil); err != nil || ok {
		t.Fatalf("proof %v %v", ok, err)
	}
	if w, ok, _ := s2.UserSettings(ctx, p.XUID); !ok || w != 0x1234 {
		t.Fatalf("settings %v %v", w, ok)
	}
	if v, _ := s2.Cursor(ctx); v != 7 {
		t.Fatalf("cursor %d", v)
	}
	if revs, err := s2.Revocations(ctx, 0, 10); err != nil || len(revs) != 1 || revs[0].EventID != "e1" {
		t.Fatalf("revocations %+v %v", revs, err)
	}
}

// A leftover alias of an older tag is removed, the current one is never touched, and a user whose
// current alias is missing keeps what it has.
func TestPruneStaleAliases(t *testing.T) {
	ctx := context.Background()
	s, err := Open(pgdb.TestDSN(t), bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := Provision{ID: "p9", CoreAccountID: "acct9", MachineID: "0000001cbe991a19", XUID: "0009000000000009", LinkID: "l9",
		Gamertag: "Current", ProofMAC: []byte{1}, ProofSealed: []byte{2}}
	if err := s.CreateProvision(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := s.CommitProvision(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO gamertag_aliases(alias_lower,xuid,created) VALUES('old tag',?,1)`, p.XUID); err != nil {
		t.Fatal(err)
	}
	if n, err := s.PruneStaleAliases(ctx, p.XUID, "Current"); err != nil || n != 1 {
		t.Fatalf("pruned %d %v", n, err)
	}
	if _, err := s.GetUserByGamertag(ctx, "Current"); err != nil {
		t.Fatal("current alias lost")
	}
	if n, _ := s.PruneStaleAliases(ctx, p.XUID, "Current"); n != 0 {
		t.Fatalf("second prune removed %d", n)
	}
	if n, _ := s.PruneStaleAliases(ctx, p.XUID, "Missing"); n != 0 {
		t.Fatalf("pruned %d with no current alias", n)
	}
}
