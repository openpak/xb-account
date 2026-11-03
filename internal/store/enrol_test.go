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
