package tests

import (
	"net/http"
	"testing"
)

func TestNotFound(t *testing.T) {
	resp, env := doJSON(t, http.MethodGet, "/api/nope", nil)
	wantStatus(t, resp, http.StatusNotFound)
	wantError(t, env, "NOT_FOUND")
}

func TestMethodNotAllowed(t *testing.T) {
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/users/register"},
		{http.MethodGet, "/api/users/login"},
		{http.MethodGet, "/api/users/logout"},
		{http.MethodPost, "/api/users/profile"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			resp, env := doJSON(t, tc.method, tc.path, nil)
			wantStatus(t, resp, http.StatusMethodNotAllowed)
			wantError(t, env, "METHOD_NOT_ALLOWED")
			if resp.Header.Get("Allow") == "" {
				t.Error("missing Allow header")
			}
		})
	}
}
