// SPDX-License-Identifier: AGPL-3.0-only

package svc5

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"

	"github.com/openpak/xb-account/internal/store"
	"github.com/openpak/xb-common/xbaccountapi"
	"github.com/openpak/xb-common/xbfeature"
)

// Dashboard status values used by the account-management calls (RE, see
// docs/xbox/account-service-protocol.md section 7).
const (
	errBadCard = 0x80154003 // "You have entered an invalid credit card number."
	errBanned  = 0x80154005 // "This account is currently banned from Xbox Live."
	errBadPin  = 0x80154006 // pin verify / account recovery mismatch
)

// Users is the slice of xb-account state the account-management calls need
// (store.Store).
type Users interface {
	GetUser(ctx context.Context, xuid string) (store.User, error)
	SetUserSettings(ctx context.Context, xuid string, word uint16) error
}

// Request body sizes (from the dashboard's request builders).
const (
	subStatusBody = 8 + 8 + 4 + 4 + 4 // xuid, machine id, three u32 (observed 28 bytes)
	troubleBody   = 8 + 8             // xuid, machine id
	pinVerifyBody = 8 + 4             // xuid, four ASCII digits
	paymentGet    = 8 + 4             // xuid, u32
	settingsBody  = 8 + 2             // xuid, u16 settings word
	userPinBody   = 8                 // xuid
)

// Layout of the getsubscriptionstatus reply the dashboard copies out: a 39-byte
// header whose byte 8 is the account status, a flag byte at 30 telling whether a
// second record follows, then two 13-byte subscription records (RE).
const (
	subHeader    = 39
	subRecord    = 13
	subStatusPos = 8
	subReply     = subHeader + 2*subRecord
	// Account status byte values the dashboard acts on (RE, FUN_0005f2c9).
	subStatusSuspended = 2
	subStatusCancelled = 4
)

func (s *Server) manageRoute(path string) handlerFunc {
	if s.Users == nil {
		return nil
	}
	switch path {
	case "/xuacs/getsubscriptionstatus.ashx":
		return s.subscriptionStatus
	case "/xuacs/troubleshootaccount.ashx":
		return s.troubleshoot
	case "/xuacs/verifybillingpin.ashx":
		return s.verifyBillingPin
	case "/xuacs/getpaymentinfo.ashx":
		return s.getPaymentInfo
	case "/xuacs/setpaymentinfo.ashx":
		return refuse(errBadCard)
	case "/xuacs/setusersettings.ashx":
		return s.setUserSettings
	case "/xuacs/setuserpin.ashx":
		if s.UserKey != nil {
			return s.setUserPin
		}
	case "/xuacs/changegamertag.ashx":
		// Gamertags change on the OpenPak website (SetGamertag), never from a console.
		return refuse(errAccountClosed)
	case "/xuacs/restoreaccount.ashx":
		// Account recovery needs billing data OpenPak does not hold: the dashboard
		// shows "The Gamertag or other information entered is invalid."
		return refuse(errBadPin)
	}
	return nil
}

// refuse answers an operation that exists on the web only. The body is not read
// or logged (setpaymentinfo carries card data).
func refuse(code uint32) handlerFunc {
	return func(context.Context, xbfeature.Info, []byte) response { return xerr(code) }
}

// owned resolves xuid to a user of the core account that owns the calling
// console. Anything else is not the caller's.
func (s *Server) owned(ctx context.Context, info xbfeature.Info, xuid uint64) (store.User, error) {
	u, err := s.Users.GetUser(ctx, fmt.Sprintf("%016x", xuid))
	if errors.Is(err, store.ErrNotFound) || err == nil && u.CoreAccountID != info.CoreAccountID {
		return store.User{}, store.ErrNotFound
	}
	return u, err
}

// ownedOrRefuse maps owned() to the failure response, ok=false when done.
func (s *Server) ownedOrRefuse(ctx context.Context, info xbfeature.Info, xuid uint64) (store.User, response, bool) {
	u, err := s.owned(ctx, info, xuid)
	switch {
	case err == nil:
		return u, response{}, true
	case errors.Is(err, store.ErrNotFound):
		return u, xerr(errAccountClosed), false
	}
	log.Printf("svc5 account management: store error")
	return u, status(503), false
}

func xuidOf(body []byte) uint64 { return binary.LittleEndian.Uint64(body) }

// machineOK compares the machine id the body carries at off with the gateway-bound one.
func machineOK(body []byte, off int, info xbfeature.Info) bool {
	if info.XUID != 0 && !xbaccountapi.ValidMachineID(info.PrincipalID) { // user session: the principal is the XUID, ownership is checked per call
		return true
	}
	return machineMatches(binary.LittleEndian.Uint64(body[off:]), info.PrincipalID)
}

// subscriptionStatus answers getsubscriptionstatus, the first call of the
// account screens. OpenPak has no subscriptions or billing: the reply carries
// the real account state (active = status byte 0, banned = suspended, unknown
// to this console = cancelled) and no subscription records.
func (s *Server) subscriptionStatus(ctx context.Context, info xbfeature.Info, body []byte) response {
	if len(body) != subStatusBody || !machineOK(body, 8, info) {
		return status(400)
	}
	out := make([]byte, subReply)
	u, err := s.owned(ctx, info, xuidOf(body))
	switch {
	case err == nil && u.Status == xbaccountapi.StatusActive:
	case err == nil && u.Status == xbaccountapi.StatusBanned:
		out[subStatusPos] = subStatusSuspended
	case err == nil, errors.Is(err, store.ErrNotFound):
		out[subStatusPos] = subStatusCancelled
	default:
		log.Printf("svc5 account management: store error")
		return status(503)
	}
	return response{code: 200, hasErr: true, body: out}
}

// troubleshoot answers troubleshootaccount, the dashboard's "pending account
// actions" query: a 32-byte record (accept new terms, forced name change,
// suspension, billing problem). OpenPak has none of these, so an active account
// gets all zeros; a banned one is refused with the dashboard's banned message.
func (s *Server) troubleshoot(ctx context.Context, info xbfeature.Info, body []byte) response {
	if len(body) != troubleBody || !machineOK(body, 8, info) {
		return status(400)
	}
	u, rsp, ok := s.ownedOrRefuse(ctx, info, xuidOf(body))
	if !ok {
		return rsp
	}
	if u.Status == xbaccountapi.StatusBanned {
		return xerr(errBanned)
	}
	return response{code: 200, hasErr: true, body: make([]byte, 32)}
}

// verifyBillingPin gates the dashboard's Credit Card menu with the last four
// digits of the card on file. No card is ever on file, so no digits can match.
func (s *Server) verifyBillingPin(ctx context.Context, info xbfeature.Info, body []byte) response {
	if len(body) != pinVerifyBody {
		return status(400)
	}
	if _, rsp, ok := s.ownedOrRefuse(ctx, info, xuidOf(body)); !ok {
		return rsp
	}
	return xerr(errBadPin)
}

// getPaymentInfo returns the billing record: truthfully empty (no card, no
// address, no e-mail on file).
func (s *Server) getPaymentInfo(ctx context.Context, info xbfeature.Info, body []byte) response {
	if len(body) != paymentGet {
		return status(400)
	}
	if _, rsp, ok := s.ownedOrRefuse(ctx, info, xuidOf(body)); !ok {
		return rsp
	}
	// 12 bytes of scalar fields (zero), then 17 empty strings (u16 length 0).
	return response{code: 200, hasErr: true, body: make([]byte, 12+17*2)}
}

// setUserSettings stores the settings word of the account-settings and
// information-sharing screens for the caller's own user.
func (s *Server) setUserSettings(ctx context.Context, info xbfeature.Info, body []byte) response {
	if len(body) != settingsBody {
		return status(400)
	}
	u, rsp, ok := s.ownedOrRefuse(ctx, info, xuidOf(body))
	if !ok {
		return rsp
	}
	if u.Status != xbaccountapi.StatusActive {
		return xerr(errAccountClosed)
	}
	if err := s.Users.SetUserSettings(ctx, u.XUID, binary.LittleEndian.Uint16(body[8:])); err != nil {
		log.Printf("svc5 account management: store error")
		return status(503)
	}
	return response{code: 200, hasErr: true}
}

// setUserPin answers the dashboard's pass code change. The pass code itself is
// kept on the console and never sent (the request is the XUID only); the reply
// is the account key the console stores. The key is not rotated, so memory unit
// and hard disk copies of the account stay valid.
func (s *Server) setUserPin(ctx context.Context, info xbfeature.Info, body []byte) response {
	if len(body) != userPinBody {
		return status(400)
	}
	u, rsp, ok := s.ownedOrRefuse(ctx, info, xuidOf(body))
	if !ok {
		return rsp
	}
	if u.Status != xbaccountapi.StatusActive {
		return xerr(errAccountClosed)
	}
	key, err := s.UserKey(xuidOf(body))
	if err != nil {
		log.Printf("svc5 setuserpin: key unavailable")
		return status(503)
	}
	return response{code: 200, hasErr: true, body: key[:]}
}
