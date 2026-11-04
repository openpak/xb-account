// SPDX-License-Identifier: AGPL-3.0-only

package erase

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler(t *testing.T) {
	calls := 0
	h := Handler("svc", "k3y", func(_ context.Context, id string) (int, error) { calls++; return 3, nil })
	do := func(auth, id string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("DELETE", "/internal/accounts/"+id+"/data", nil)
		if auth != "" {
			r.Header.Set("Authorization", auth)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	const id = "11111111-2222-3333-4444-555555555555"
	if w := do("", id); w.Code != 401 {
		t.Fatalf("no key: %d", w.Code)
	}
	if w := do("Bearer nope", id); w.Code != 401 {
		t.Fatalf("wrong key: %d", w.Code)
	}
	if w := do("Bearer k3y", "not-a-uuid"); w.Code != 400 {
		t.Fatalf("bad id: %d", w.Code)
	}
	if calls != 0 {
		t.Fatal("erase ran without authorisation")
	}
	w := do("Bearer k3y", id)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"service":"svc"`) || !strings.Contains(w.Body.String(), `"rows":3`) {
		t.Fatalf("ok: %d %s", w.Code, w.Body)
	}
	_ = http.StatusOK
}
