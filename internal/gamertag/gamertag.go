// SPDX-License-Identifier: AGPL-3.0-only

// Package gamertag keeps the local gamertag (users.gamertag and its alias, the
// name-reservation table) a cache of the account core handle. The core handle
// wins: a push on the core profile_updated event re-reads the account, and a
// full pass every TTL (and once at start) repairs anything that drifted.
// Core unreachable means the cache is served and nothing changes.
package gamertag

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/openpak/xb-account/internal/core"
	"github.com/openpak/xb-account/internal/store"
	"github.com/openpak/xb-common/xbaccountapi"
)

// TTL is the backup refresh interval for the push path.
const TTL = 5 * time.Minute

const actor = "core-handle-sync"

type Syncer struct {
	Core  core.API
	Store *store.Store
}

// Counts is the outcome of a pass. No names, only numbers.
type Counts struct{ Checked, Repaired, Skipped, Failed int }

// Sync makes every gamertag row of the account equal the core handle.
// Accounts without a handle (legacy seeds) are left alone.
func (s *Syncer) Sync(ctx context.Context, account string) (Counts, error) {
	var c Counts
	acct, err := s.Core.GetAccount(ctx, account)
	if err != nil {
		return c, err
	}
	users, err := s.Store.UsersOf(ctx, account)
	if err != nil {
		return c, err
	}
	for _, u := range users {
		c.Checked++
		if acct.Handle == "" || !xbaccountapi.ValidGamertag(acct.Handle) || len(acct.Handle) > 15 {
			c.Skipped++
			continue
		}
		if u.Gamertag == acct.Handle {
			continue
		}
		if o, err := s.Store.GetUserByGamertag(ctx, acct.Handle); err == nil && o.XUID != u.XUID {
			c.Failed++ // held by another identity: never steal, report
			continue
		}
		if u.LinkID != "" {
			if err := s.Core.SetLinkPublicCode(ctx, u.LinkID, acct.Handle); err != nil && !errors.Is(err, core.ErrPrecondition) && !errors.Is(err, core.ErrNotFound) {
				c.Failed++
				continue
			}
		}
		if err := s.Store.RenameUserAudited(ctx, account, u.XUID, u.Gamertag, acct.Handle, "", actor); err != nil {
			c.Failed++
			continue
		}
		c.Repaired++
	}
	return c, nil
}

// Pass syncs every account that has a user row.
func (s *Syncer) Pass(ctx context.Context) (Counts, error) {
	var t Counts
	users, err := s.Store.LiveUsers(ctx)
	if err != nil {
		return t, err
	}
	seen := map[string]bool{}
	for _, u := range users {
		if seen[u.CoreAccountID] {
			continue
		}
		seen[u.CoreAccountID] = true
		c, err := s.Sync(ctx, u.CoreAccountID)
		if err != nil {
			if errors.Is(err, core.ErrNotFound) {
				t.Skipped++
				continue
			}
			t.Failed++
			if errors.Is(err, core.ErrUnavailable) {
				return t, err // core down: keep serving the cache
			}
			continue
		}
		t.Checked += c.Checked
		t.Repaired += c.Repaired
		t.Skipped += c.Skipped
		t.Failed += c.Failed
	}
	return t, nil
}

// Run reconciles once at start and then every ttl.
func (s *Syncer) Run(ctx context.Context, ttl time.Duration) {
	first := true
	for ctx.Err() == nil {
		c, err := s.Pass(ctx)
		switch {
		case err != nil:
			log.Printf("xbox gamertag sync: core unavailable, serving cache (checked=%d failed=%d)", c.Checked, c.Failed)
		case first || c.Repaired > 0 || c.Failed > 0:
			log.Printf("xbox gamertag sync: checked=%d repaired=%d skipped=%d failed=%d", c.Checked, c.Repaired, c.Skipped, c.Failed)
		}
		first = false
		select {
		case <-ctx.Done():
		case <-time.After(ttl):
		}
	}
}
