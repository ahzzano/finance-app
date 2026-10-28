package tests

import (
	"context"
	"net/http"
	"reflect"
	"testing"
)

const profilePath = "/api/users/profile"

func TestProfileOK(t *testing.T) {
	resetDB(t)
	c := registerSession(t, "a@x.com", "password123", "Alice")
	resp, env := doJSON(t, http.MethodGet, profilePath, nil, c)
	wantStatus(t, resp, http.StatusOK)
	want := map[string]any{"name": "Alice", "email": "a@x.com"}
	if !reflect.DeepEqual(env.Data, want) || env.Errors != nil {
		t.Fatalf("got %+v, want data %v and errors null", env, want)
	}
}

func TestProfileNormalizedEmail(t *testing.T) {
	resetDB(t)
	c := registerSession(t, "  A@X.com ", "password123", "Alice")
	_, env := doJSON(t, http.MethodGet, profilePath, nil, c)
	if env.Data["email"] != "a@x.com" || len(env.Data) != 2 {
		t.Fatalf("got %+v", env)
	}
}

func TestProfileUnauthenticated(t *testing.T) {
	resetDB(t)
	registerSession(t, "a@x.com", "password123", "Alice")
	for name, c := range map[string]*http.Cookie{
		"no cookie": nil,
		"garbage":   {Name: "session", Value: "garbage"},
	} {
		t.Run(name, func(t *testing.T) {
			var resp *http.Response
			var env envelope
			if c == nil {
				resp, env = doJSON(t, http.MethodGet, profilePath, nil)
			} else {
				resp, env = doJSON(t, http.MethodGet, profilePath, nil, c)
			}
			wantStatus(t, resp, http.StatusUnauthorized)
			wantError(t, env, "AUTHENTICATION_ERROR")
		})
	}
}

func TestProfileExpiredSession(t *testing.T) {
	resetDB(t)
	c := registerSession(t, "a@x.com", "password123", "Alice")
	if _, err := pool.Exec(context.Background(), "UPDATE sessions SET expires_at = now() - interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	resp, env := doJSON(t, http.MethodGet, profilePath, nil, c)
	wantStatus(t, resp, http.StatusUnauthorized)
	wantError(t, env, "AUTHENTICATION_ERROR")
}

func TestProfileTokenOutsideCookieIgnored(t *testing.T) {
	resetDB(t)
	c := registerSession(t, "a@x.com", "password123", "Alice")
	resp, env := doJSON(t, http.MethodGet, profilePath+"?session="+c.Value, nil)
	wantStatus(t, resp, http.StatusUnauthorized)
	wantError(t, env, "AUTHENTICATION_ERROR")

	req, err := http.NewRequest(http.MethodGet, srv.URL+profilePath, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Value)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	wantStatus(t, resp2, http.StatusUnauthorized)
}

func TestProfileIsolatedPerUser(t *testing.T) {
	resetDB(t)
	ca := registerSession(t, "a@x.com", "password123", "Alice")
	cb := registerSession(t, "b@x.com", "password123", "Bob")
	for c, want := range map[*http.Cookie]string{ca: "Alice", cb: "Bob"} {
		_, env := doJSON(t, http.MethodGet, profilePath, nil, c)
		if env.Data["name"] != want {
			t.Fatalf("got %+v, want %s", env.Data, want)
		}
	}
}
