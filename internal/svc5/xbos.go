// SPDX-License-Identifier: AGPL-3.0-only

package svc5

import (
	"context"
	"encoding/binary"

	"github.com/openpak/xb-common/xbfeature"
)

// Request sizes of the signed-in offering calls (dashboard request builders,
// research/xbox/xbos-protocol.md).
const (
	offerDetailsBody = 8 + 8 + 4 + 8 + 4 + 2 // xuid, machine id, u32, offer id, u32, u16
	offerCancelBody  = 8 + 8                 // xuid, offer id
)

// offeringDetailsUser answers /xbos/OfferingDetails.ashx (service 4), the
// subscription screens' offer lookup for a signed-in user. OpenPak has one free
// offer; the reply is the same record the NoUser path gives, bound to the user
// (only the caller's own user may ask).
func (s *Server) offeringDetailsUser(ctx context.Context, info xbfeature.Info, body []byte) response {
	if len(body) != offerDetailsBody || !machineOK(body, 8, info) {
		return status(400)
	}
	if _, rsp, ok := s.ownedOrRefuse(ctx, info, xuidOf(body)); !ok {
		return rsp
	}
	if binary.LittleEndian.Uint64(body[20:28]) != OfferID {
		return xerr(errCodeInvalid)
	}
	return response{code: 200, hasErr: true, body: buildOfferRecord()}
}

// offeringCancel answers /xbos/OfferingCancel.ashx (service 9): "unsubscribe"
// on the current-subscription screen. The client only looks at the status.
// ponytail: OpenPak has no paid subscription to cancel, so success changes
// nothing; revisit if an offer with a lifetime is ever introduced.
func (s *Server) offeringCancel(ctx context.Context, info xbfeature.Info, body []byte) response {
	if len(body) != offerCancelBody {
		return status(400)
	}
	if _, rsp, ok := s.ownedOrRefuse(ctx, info, xuidOf(body)); !ok {
		return rsp
	}
	return response{code: 200, hasErr: true}
}
