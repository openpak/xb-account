// SPDX-License-Identifier: AGPL-3.0-only

package svc5

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/openpak/xb-account/internal/store"
	"github.com/openpak/xb-common/xbfeature"
)

const (
	userX     = "0009d941ad372efd"
	userXNum  = 0x0009d941ad372efd
	otherX    = "0009aaaaaaaaaaaa"
	otherXNum = 0x0009aaaaaaaaaaaa
)

func manageServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	s, db := newServer(t)
	s.Users = db
	s.UserKey = func(x uint64) ([16]byte, error) { return [16]byte{byte(x), 2, 3, 4}, nil }
	ctx := context.Background()
	if err := db.CommitAuto(ctx, store.User{XUID: userX, CoreAccountID: "acct1", Gamertag: "Alpha", LinkID: "l1"}); err != nil {
		t.Fatal(err)
	}
	if err := db.CommitAuto(ctx, store.User{XUID: otherX, CoreAccountID: "acct2", Gamertag: "Bravo", LinkID: "l2"}); err != nil {
		t.Fatal(err)
	}
	return s, db
}

func u64(b []byte, v uint64) []byte { return binary.LittleEndian.AppendUint64(b, v) }

func respBody(t *testing.T, resp string) ([]byte, string) {
	t.Helper()
	head, b, _ := strings.Cut(resp, "\r\n\r\n")
	return []byte(b), head
}

func wantStatus(t *testing.T, resp, line string) {
	t.Helper()
	if !strings.HasPrefix(resp, line) {
		t.Fatalf("response %q does not start with %q", resp, line)
	}
}

func xerrOf(resp string) string {
	_, rest, _ := strings.Cut(resp, "X-Err: ")
	v, _, _ := strings.Cut(rest, "\r\n")
	return v
}

// observed is the getsubscriptionstatus request of a real dashboard run (Berth,
// 2026-10-17): user XUID, machine id, three u32.
func observedSub(t *testing.T) []byte {
	b, err := hex.DecodeString("1c3bf1ff254f0900692f450dce0000000000feff0100000001000000")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSubscriptionStatusObservedShapeActive(t *testing.T) {
	s, db := manageServer(t)
	// The observed XUID belongs to the console's account in this fixture.
	b := observedSub(t)
	if err := db.CommitAuto(context.Background(), store.User{XUID: "00094f25fff13b1c", CoreAccountID: "acctX", Gamertag: "Real", LinkID: "l9"}); err != nil {
		t.Fatal(err)
	}
	info := infoA
	info.CoreAccountID = "acctX"
	resp := do(t, s, info, "POST", "/xuacs/getsubscriptionstatus.ashx", "xon/5", b)
	wantStatus(t, resp, "HTTP/1.0 200 OK\r\nX-Err: 0\r\n")
	body, _ := respBody(t, resp)
	if len(body) != subReply {
		t.Fatalf("len = %d, want %d", len(body), subReply)
	}
	for i, v := range body {
		if v != 0 {
			t.Fatalf("byte %d = %d: an active account with no subscription is all zero", i, v)
		}
	}
}

func TestSubscriptionStatusStates(t *testing.T) {
	s, db := manageServer(t)
	req := func(x uint64, info xbfeature.Info) []byte {
		b := u64(nil, x)
		b = u64(b, 0xce0d452f69)
		b = append(b, make([]byte, 12)...)
		return []byte(do(t, s, info, "POST", "/xuacs/getsubscriptionstatus.ashx", "xon/5", b))
	}
	r := string(req(userXNum, infoA))
	if body, _ := respBody(t, r); body[subStatusPos] != 0 {
		t.Fatalf("active status byte = %d", body[subStatusPos])
	}
	// Another account's XUID, or an unknown one: cancelled, never leaked.
	for _, x := range []uint64{otherXNum, 0x1234} {
		r = string(req(x, infoA))
		wantStatus(t, r, "HTTP/1.0 200 OK")
		if body, _ := respBody(t, r); body[subStatusPos] != subStatusCancelled {
			t.Fatalf("xuid %x status byte = %d, want cancelled", x, body[subStatusPos])
		}
	}
	_ = db
}

func TestSubscriptionStatusRejectsBadShape(t *testing.T) {
	s, _ := manageServer(t)
	wantStatus(t, do(t, s, infoA, "POST", "/xuacs/getsubscriptionstatus.ashx", "xon/5", make([]byte, 27)), "HTTP/1.0 400")
	b := u64(nil, userXNum)
	b = u64(b, 0x99) // not this console
	b = append(b, make([]byte, 12)...)
	wantStatus(t, do(t, s, infoA, "POST", "/xuacs/getsubscriptionstatus.ashx", "xon/5", b), "HTTP/1.0 400")
}

func TestTroubleshootNoPendingActions(t *testing.T) {
	s, _ := manageServer(t)
	b := u64(u64(nil, userXNum), 0xce0d452f69)
	resp := do(t, s, infoA, "POST", "/xuacs/troubleshootaccount.ashx", "xon/5", b)
	wantStatus(t, resp, "HTTP/1.0 200 OK\r\nX-Err: 0\r\n")
	if body, _ := respBody(t, resp); len(body) != 32 || strings.Trim(string(body), "\x00") != "" {
		t.Fatalf("body %x", body)
	}
	// Someone else's XUID is refused as not usable.
	b = u64(u64(nil, otherXNum), 0xce0d452f69)
	resp = do(t, s, infoA, "POST", "/xuacs/troubleshootaccount.ashx", "xon/5", b)
	if xerrOf(resp) != "8015400a" {
		t.Fatalf("foreign xuid: %q", resp)
	}
}

func TestVerifyBillingPinNeverSucceeds(t *testing.T) {
	s, _ := manageServer(t)
	// Observed request: XUID then the typed digits 1,1,1,1 (raw bytes 01 01 01 01).
	b := append(u64(nil, userXNum), 1, 1, 1, 1)
	resp := do(t, s, infoA, "POST", "/xuacs/verifybillingpin.ashx", "xon/5", b)
	if xerrOf(resp) != "80154006" {
		t.Fatalf("got %q", resp)
	}
	for _, digits := range []string{"0000", "1234", "9999"} {
		resp = do(t, s, infoA, "POST", "/xuacs/verifybillingpin.ashx", "xon/5", append(u64(nil, userXNum), digits...))
		if xerrOf(resp) != "80154006" {
			t.Fatalf("%s: %q", digits, resp)
		}
	}
	wantStatus(t, do(t, s, infoA, "POST", "/xuacs/verifybillingpin.ashx", "xon/5", []byte{1, 2, 3}), "HTTP/1.0 400")
}

func TestPaymentIsEmptyAndNeverStored(t *testing.T) {
	s, _ := manageServer(t)
	b := append(u64(nil, userXNum), 0, 0, 0, 0)
	resp := do(t, s, infoA, "POST", "/xuacs/getpaymentinfo.ashx", "xon/5", b)
	wantStatus(t, resp, "HTTP/1.0 200 OK\r\nX-Err: 0\r\n")
	if body, _ := respBody(t, resp); len(body) != 12+17*2 || strings.Trim(string(body), "\x00") != "" {
		t.Fatalf("body %x", body)
	}
	// Any card data is refused, whatever the shape.
	for _, n := range []int{10, 600, 2000} {
		resp = do(t, s, infoA, "POST", "/xuacs/setpaymentinfo.ashx", "xon/5", make([]byte, n))
		if xerrOf(resp) != "80154003" {
			t.Fatalf("n=%d: %q", n, resp)
		}
	}
}

func TestSetUserSettingsStoresForOwnUserOnly(t *testing.T) {
	s, db := manageServer(t)
	ctx := context.Background()
	b := binary.LittleEndian.AppendUint16(u64(nil, userXNum), 0x000b)
	resp := do(t, s, infoA, "POST", "/xuacs/setusersettings.ashx", "xon/5", b)
	wantStatus(t, resp, "HTTP/1.0 200 OK\r\nX-Err: 0\r\n")
	if w, ok, err := db.UserSettings(ctx, userX); err != nil || !ok || w != 0x000b {
		t.Fatalf("stored %x %v %v", w, ok, err)
	}
	// Overwrites.
	do(t, s, infoA, "POST", "/xuacs/setusersettings.ashx", "xon/5", binary.LittleEndian.AppendUint16(u64(nil, userXNum), 0))
	if w, _, _ := db.UserSettings(ctx, userX); w != 0 {
		t.Fatalf("not overwritten: %x", w)
	}
	// Another account's user is refused and untouched.
	resp = do(t, s, infoA, "POST", "/xuacs/setusersettings.ashx", "xon/5", binary.LittleEndian.AppendUint16(u64(nil, otherXNum), 7))
	if xerrOf(resp) != "8015400a" {
		t.Fatalf("foreign: %q", resp)
	}
	if _, ok, _ := db.UserSettings(ctx, otherX); ok {
		t.Fatal("foreign settings were stored")
	}
	wantStatus(t, do(t, s, infoA, "POST", "/xuacs/setusersettings.ashx", "xon/5", make([]byte, 11)), "HTTP/1.0 400")
}

func TestSetUserPinReturnsUnchangedKey(t *testing.T) {
	s, _ := manageServer(t)
	b := u64(nil, userXNum)
	resp := do(t, s, infoA, "POST", "/xuacs/setuserpin.ashx", "xon/5", b)
	wantStatus(t, resp, "HTTP/1.0 200 OK\r\nX-Err: 0\r\n")
	body, _ := respBody(t, resp)
	want := [16]byte{0xfd, 2, 3, 4}
	if string(body) != string(want[:]) {
		t.Fatalf("key %x", body)
	}
	if xerrOf(do(t, s, infoA, "POST", "/xuacs/setuserpin.ashx", "xon/5", u64(nil, otherXNum))) != "8015400a" {
		t.Fatal("foreign xuid accepted")
	}
	s.UserKey = nil
	wantStatus(t, do(t, s, infoA, "POST", "/xuacs/setuserpin.ashx", "xon/5", b), "HTTP/1.0 501")
}

func TestWebOnlyOperationsRefuse(t *testing.T) {
	s, _ := manageServer(t)
	for path, want := range map[string]string{
		"/xuacs/changegamertag.ashx": "8015400a",
		"/xuacs/restoreaccount.ashx": "80154006",
	} {
		resp := do(t, s, infoA, "POST", path, "xon/5", make([]byte, 40))
		if xerrOf(resp) != want {
			t.Fatalf("%s: %q", path, resp)
		}
	}
}

func TestManageNeedsProvisionedConsoleAndUsers(t *testing.T) {
	s, _ := manageServer(t)
	anon := infoA
	anon.CoreAccountID = ""
	b := u64(nil, userXNum)
	wantStatus(t, do(t, s, anon, "POST", "/xuacs/setuserpin.ashx", "xon/5", b), "HTTP/1.0 403")
	wantStatus(t, do(t, s, infoA, "GET", "/xuacs/setuserpin.ashx", "xon/5", b), "HTTP/1.0 405")
	s.Users = nil
	wantStatus(t, do(t, s, infoA, "POST", "/xuacs/setuserpin.ashx", "xon/5", b), "HTTP/1.0 501")
}
