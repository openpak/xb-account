// SPDX-License-Identifier: AGPL-3.0-only

package svc5

import (
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"
)

// detailsBody is a byte fixture of the 34-byte OfferingDetails request:
// xuid, machine id, u32, offer id, u32, u16.
func detailsBody(xuid, offer uint64) []byte {
	b := u64(nil, xuid)
	b = u64(b, 0xce0d452f69)
	b = binary.LittleEndian.AppendUint32(b, 0x409)
	b = u64(b, offer)
	b = binary.LittleEndian.AppendUint32(b, 1)
	return binary.LittleEndian.AppendUint16(b, 0x40)
}

func TestOfferingDetailsUser(t *testing.T) {
	s, _ := manageServer(t)
	b := detailsBody(userXNum, OfferID)
	if len(b) != 34 {
		t.Fatal(len(b))
	}
	// layout pinned as bytes: xuid LE, machine id LE, offer id at 20
	if got := hex.EncodeToString(b[:8]); got != "fd2e37ad41d90900" || hex.EncodeToString(b[20:28]) != "014b41504e45504f" {
		t.Fatalf("layout %x", b)
	}
	r := do(t, s, infoA, "POST", "/xbos/OfferingDetails.ashx", "xon/4", b)
	body, _ := respBody(t, r)
	n := int(binary.LittleEndian.Uint32(body[0x11:]))
	if xerrOf(r) != "0" || len(body) != 0x15+n || body[0x15] != 0xff || body[0x16] != 0xfe {
		t.Fatalf("%q", r)
	}
	// identical record to the NoUser path
	if string(body) != string(buildOfferRecord()) {
		t.Fatal("record differs")
	}
	if r := do(t, s, infoA, "POST", "/xbos/OfferingDetails.ashx", "xon/4", detailsBody(userXNum, 7)); xerrOf(r) != "80154007" {
		t.Fatal(r)
	}
	if r := do(t, s, infoA, "POST", "/xbos/OfferingDetails.ashx", "xon/4", detailsBody(otherXNum, OfferID)); xerrOf(r) != "8015400a" {
		t.Fatalf("foreign user: %q", r)
	}
	for name, c := range map[string]struct {
		ct string
		b  []byte
	}{
		"wrong service": {"xon/5", b},
		"short":         {"xon/4", b[:33]},
		"long":          {"xon/4", append(append([]byte{}, b...), 0)},
	} {
		if r := do(t, s, infoA, "POST", "/xbos/OfferingDetails.ashx", c.ct, c.b); !strings.HasPrefix(r, "HTTP/1.0 400") {
			t.Fatalf("%s: %q", name, r)
		}
	}
	// another console's machine id inside the body
	bad := detailsBody(userXNum, OfferID)
	binary.LittleEndian.PutUint64(bad[8:], 0x99)
	wantStatus(t, do(t, s, infoA, "POST", "/xbos/OfferingDetails.ashx", "xon/4", bad), "HTTP/1.0 400")
}

func TestOfferingCancel(t *testing.T) {
	s, _ := manageServer(t)
	b := u64(u64(nil, userXNum), OfferID)
	r := do(t, s, infoA, "POST", "/xbos/OfferingCancel.ashx", "xon/9", b)
	if body, _ := respBody(t, r); xerrOf(r) != "0" || len(body) != 0 || !strings.HasPrefix(r, "HTTP/1.0 200") {
		t.Fatalf("%q", r)
	}
	if r := do(t, s, infoA, "POST", "/xbos/OfferingCancel.ashx", "xon/9", u64(u64(nil, otherXNum), OfferID)); xerrOf(r) != "8015400a" {
		t.Fatalf("foreign user: %q", r)
	}
	wantStatus(t, do(t, s, infoA, "POST", "/xbos/OfferingCancel.ashx", "xon/4", b), "HTTP/1.0 400")
	wantStatus(t, do(t, s, infoA, "POST", "/xbos/OfferingCancel.ashx", "xon/9", b[:15]), "HTTP/1.0 400")
	wantStatus(t, do(t, s, infoA, "GET", "/xbos/OfferingCancel.ashx", "xon/9", b), "HTTP/1.0 405")
}
