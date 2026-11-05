package gamertag

import (
	"bytes"
	"context"
	"testing"

	"github.com/openpak/xb-account/internal/autoid"
	"github.com/openpak/xb-account/internal/core"
	pb "github.com/openpak/xb-account/internal/corepb"
	"github.com/openpak/xb-account/internal/coretest"
	"github.com/openpak/xb-account/internal/pgdb"
	"github.com/openpak/xb-account/internal/store"
)

func TestSyncCoreHandleWins(t *testing.T) {
	db, err := store.Open(pgdb.TestDSN(t), bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	c := coretest.New()
	c.Handles = map[string]string{"a1": "Alpha", "a2": "Beta"}
	c.AddAccount("a1", pb.AccountStatus_ACCOUNT_STATUS_ACTIVE)
	c.AddAccount("a2", pb.AccountStatus_ACCOUNT_STATUS_ACTIVE)
	au := &autoid.Service{Core: c, Store: db}
	au.Ensure(ctx, "a1")
	au.Ensure(ctx, "a2")
	s := &Syncer{Core: c, Store: db}

	if r, err := s.Pass(ctx); err != nil || r.Repaired != 0 || r.Checked != 2 {
		t.Fatalf("%+v %v", r, err)
	}
	// The core handle changes after signup: the cache, alias and link follow, with an audit row.
	c.Handles["a1"] = "Gamma"
	r, err := s.Pass(ctx)
	if err != nil || r.Repaired != 1 {
		t.Fatalf("%+v %v", r, err)
	}
	u, _, _ := db.ActiveUserOf(ctx, "a1")
	if u.Gamertag != "Gamma" {
		t.Fatalf("cache %q", u.Gamertag)
	}
	if _, err := db.GetUserByGamertag(ctx, "Alpha"); err == nil {
		t.Fatal("old alias must go")
	}
	if o, err := db.GetUserByGamertag(ctx, "Gamma"); err != nil || o.XUID != u.XUID {
		t.Fatal("new alias missing")
	}
	if l, _ := c.GetActiveLinkByAccount(ctx, "a1"); l.PublicCode != "Gamma" {
		t.Fatalf("link %q", l.PublicCode)
	}
	if a, err := db.AuditCount(ctx, "a1"); err != nil || a != 1 {
		t.Fatalf("audit %d %v", a, err)
	}
	// Stable afterwards; core down serves the cache.
	if r, _ := s.Pass(ctx); r.Repaired != 0 {
		t.Fatalf("%+v", r)
	}
	c.Handles["a2"] = "Delta"
	c.Fail = func(string) error { return coreUnavailable() }
	if _, err := s.Pass(ctx); err == nil {
		t.Fatal("want error")
	}
	if u, _, _ := db.ActiveUserOf(ctx, "a2"); u.Gamertag != "Beta" {
		t.Fatal("cache must be served unchanged")
	}
	// A handle held by another identity is never stolen.
	c.Fail = nil
	c.Handles["a2"] = "Gamma"
	if r, _ := s.Pass(ctx); r.Failed != 1 || r.Repaired != 0 {
		t.Fatalf("%+v", r)
	}
	// Erasure still covers the audit rows the sync wrote.
	if _, err := db.EraseAccount(ctx, "a1"); err != nil {
		t.Fatal(err)
	}
	if a, _ := db.AuditCount(ctx, "a1"); a != 0 {
		t.Fatal("audit survived erasure")
	}
}

func coreUnavailable() error { return core.ErrUnavailable }
