// SPDX-License-Identifier: AGPL-3.0-only

// Package profile applies the canonical core handle to an existing Xbox identity
// during the reviewed alpha migration. It preserves the XUID and credentials;
// the internal provisioner endpoint cannot choose a different populated handle.
package profile

import (
	"context"
	"errors"
	"strings"

	"github.com/openpak/xb-account/internal/core"
	"github.com/openpak/xb-account/internal/projection"
	"github.com/openpak/xb-account/internal/store"
	"github.com/openpak/xb-common/xbaccountapi"
)

var (
	ErrInvalid       = errors.New("profile: invalid gamertag")
	ErrReserved      = errors.New("profile: reserved gamertag")
	ErrTaken         = errors.New("profile: gamertag in use")
	ErrNoIdentity    = errors.New("profile: account has no Xbox identity")
	ErrConsoleBound  = errors.New("profile: identity is bound to a console; the console must re-enrol after a rename")
	ErrRequestReplay = errors.New("profile: request id reused with a different gamertag")
	ErrUnavailable   = errors.New("profile: unavailable")
)

// reserved words (case-insensitive, spaces ignored): impersonation and platform names.
var reserved = map[string]bool{
	"xbox": true, "xboxlive": true, "xboxlivesupport": true, "microsoft": true, "admin": true, "administrator": true,
	"moderator": true, "support": true, "system": true, "openpak": true, "openpakstaff": true, "staff": true,
	"guest": true, "player": true, "null": true, "undefined": true, "root": true,
}

// Reserved reports whether a gamertag is on the reserved list.
func Reserved(tag string) bool {
	k := strings.ToLower(strings.ReplaceAll(tag, " ", ""))
	return reserved[k] || strings.HasPrefix(k, "xboxlive") || strings.HasPrefix(k, "openpak")
}

type Service struct {
	Core  core.API
	Store *store.Store
}

// Request changes the gamertag of the account's Xbox identity.
type Request struct {
	Account  string
	Gamertag string
	// RequestID makes the call idempotent: the same id replays the recorded result.
	RequestID string
	// Actor is recorded in the audit row (the authenticated website principal).
	Actor string
	// ForceConsoleIdentity allows renaming an identity that has a console device.
	// The console stored the old name as its sign-in name; it must re-enrol.
	ForceConsoleIdentity bool
}

type Result struct {
	XUID, Gamertag, Previous string
	Changed                  bool
}

// SetGamertag validates and applies the change. Friends' list versions follow:
// xb-social reads the gamertag live from the core link and its roster watcher
// pushes the change to every signed-in friend within one interval.
func (s *Service) SetGamertag(ctx context.Context, r Request) (Result, error) {
	if r.Account == "" || len(r.Gamertag) > 15 || !xbaccountapi.ValidGamertag(r.Gamertag) || len(r.RequestID) > 64 || len(r.Actor) > 128 {
		return Result{}, ErrInvalid
	}
	acct, err := s.Core.GetAccount(ctx, r.Account)
	if errors.Is(err, core.ErrNotFound) {
		return Result{}, ErrNoIdentity
	}
	if err != nil {
		return Result{}, ErrUnavailable
	}
	if acct.Handle != "" && r.Gamertag != acct.Handle {
		return Result{}, ErrInvalid
	}
	if acct.Handle == "" && Reserved(r.Gamertag) {
		return Result{}, ErrReserved
	}
	defer projection.Lock()() // serialises with auto-identity creation and enrolment
	if r.RequestID != "" {
		if a, ok, err := s.Store.AuditByRequest(ctx, r.Account, r.RequestID); err != nil {
			return Result{}, ErrUnavailable
		} else if ok {
			if !strings.EqualFold(a.New, r.Gamertag) {
				return Result{}, ErrRequestReplay
			}
			return Result{XUID: a.XUID, Gamertag: a.New, Previous: a.Old}, nil
		}
	}
	u, ok, err := s.Store.ActiveUserOf(ctx, r.Account)
	if err != nil {
		return Result{}, ErrUnavailable
	}
	if !ok || u.Status != xbaccountapi.StatusActive {
		return Result{}, ErrNoIdentity
	}
	if u.Gamertag == r.Gamertag {
		return Result{XUID: u.XUID, Gamertag: u.Gamertag, Previous: u.Gamertag}, nil // idempotent no-op
	}
	if dev, err := s.Store.HasDevice(ctx, r.Account); err != nil {
		return Result{}, ErrUnavailable
	} else if dev && !r.ForceConsoleIdentity {
		return Result{}, ErrConsoleBound
	}
	if o, err := s.Store.GetUserByGamertag(ctx, r.Gamertag); err == nil && o.XUID != u.XUID {
		return Result{}, ErrTaken
	} else if err != nil && !errors.Is(err, store.ErrNotFound) {
		return Result{}, ErrUnavailable
	}
	if err := s.Core.SetLinkPublicCode(ctx, u.LinkID, r.Gamertag); errors.Is(err, core.ErrConflict) {
		return Result{}, ErrTaken
	} else if err != nil {
		return Result{}, ErrUnavailable
	}
	if err := s.Store.RenameUserAudited(ctx, r.Account, u.XUID, u.Gamertag, r.Gamertag, r.RequestID, r.Actor); errors.Is(err, store.ErrConflict) {
		_ = s.Core.SetLinkPublicCode(ctx, u.LinkID, u.Gamertag) // keep core and local in step
		return Result{}, ErrTaken
	} else if err != nil {
		return Result{}, ErrUnavailable
	}
	return Result{XUID: u.XUID, Gamertag: r.Gamertag, Previous: u.Gamertag, Changed: true}, nil
}
