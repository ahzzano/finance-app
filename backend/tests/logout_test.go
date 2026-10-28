package tests

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

const logoutPath = "/api/users/logout"

func TestLogoutSuccess(t *testing.T) {
	resetDB(t)
	c := registerSession(t, "a@example.com", "password123", "Ann")
	resp, env := doJSON(t, http.MethodPost, logoutPath, nil, c)
	wantStatus(t, resp, http.StatusOK)
	if env.Errors != nil || env.Data["message"] != "user is successfully logged out" {
		t.Fatalf("unexpected body: %+v", env)
	}
	if sc := sessionCookie(resp); sc == nil || sc.MaxAge >= 0 || !sc.HttpOnly || !sc.Secure || sc.Path != "/" {
		t.Fatalf("bad cleared cookie: %+v", sc)
	}
	if raw := strings.Join(resp.Header.Values("Set-Cookie"), ";"); !strings.Contains(raw, "Max-Age=0") {
		t.Fatalf("Set-Cookie lacks Max-Age=0: %s", raw)
	}
}

func TestLogoutCookieReuse(t *testing.T) {
	resetDB(t)
	c := registerSession(t, "a@example.com", "password123", "Ann")
	resp, _ := doJSON(t, http.MethodPost, logoutPath, nil, c)
	wantStatus(t, resp, http.StatusOK)

	resp, env := doJSON(t, http.MethodPost, logoutPath, nil, c)
	wantStatus(t, resp, http.StatusUnauthorized)
	wantError(t, env, "AUTHENTICATION_ERROR")

	resp, _ = doJSON(t, http.MethodGet, "/api/users/profile", nil, c)
	wantStatus(t, resp, http.StatusUnauthorized)
}

func TestLogoutUnauthenticated(t *testing.T) {
	resetDB(t)
	c := registerSession(t, "a@example.com", "password123", "Ann")
	if _, err := pool.Exec(context.Background(), "UPDATE sessions SET expires_at = now() - interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	cases := map[string][]*http.Cookie{
		"no cookie":      nil,
		"garbage cookie": {{Name: "session", Value: "garbage"}},
		"expired":        {c},
	}
	for name, cookies := range cases {
		t.Run(name, func(t *testing.T) {
			resp, env := doJSON(t, http.MethodPost, logoutPath, nil, cookies...)
			wantStatus(t, resp, http.StatusUnauthorized)
			wantError(t, env, "AUTHENTICATION_ERROR")
		})
	}
}

func TestLogoutKeepsOtherSession(t *testing.T) {
	resetDB(t)
	c1 := registerSession(t, "a@example.com", "password123", "Ann")
	resp, _ := login(t, "a@example.com", "password123")
	wantStatus(t, resp, http.StatusOK)
	c2 := sessionCookie(resp)

	resp, _ = doJSON(t, http.MethodPost, logoutPath, nil, c1)
	wantStatus(t, resp, http.StatusOK)
	resp, _ = doJSON(t, http.MethodPost, logoutPath, nil, c2)
	wantStatus(t, resp, http.StatusOK)
}
