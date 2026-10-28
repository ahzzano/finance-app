package tests

import (
	"context"
	"crypto/sha256"
	"net/http"
	"strings"
	"testing"
)

// wantSession asserts status, data == {"name": wantName}, null errors and the session cookie attributes; returns the cookie value.
func wantSession(t *testing.T, resp *http.Response, env envelope, status int, wantName string) string {
	t.Helper()
	wantStatus(t, resp, status)
	if len(env.Data) != 1 || env.Data["name"] != wantName || env.Errors != nil {
		t.Fatalf("want data {name:%q} and null errors, got %+v", wantName, env)
	}
	c := sessionCookie(resp)
	if c == nil || c.Value == "" {
		t.Fatal("no session cookie")
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.MaxAge != 7*24*3600 {
		t.Fatalf("bad cookie attributes: %+v", c)
	}
	return c.Value
}

// wantField asserts a 400 VALIDATION_ERROR whose first error has the given field.
func wantField(t *testing.T, resp *http.Response, env envelope, field string) {
	t.Helper()
	wantStatus(t, resp, http.StatusBadRequest)
	wantError(t, env, "VALIDATION_ERROR")
	if env.Errors[0].Field != field {
		t.Fatalf("field = %q, want %q", env.Errors[0].Field, field)
	}
}

func TestRegister(t *testing.T) {
	resetDB(t)
	resp, env := register(t, "  A@X.com ", "password123", "  Alice ")
	tok := wantSession(t, resp, env, http.StatusCreated, "Alice")

	h := sha256.Sum256([]byte(tok))
	var n int
	err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM sessions WHERE token_hash = $1 AND expires_at > now()", h[:]).Scan(&n)
	if err != nil || n != 1 {
		t.Fatalf("session rows = %d, err %v", n, err)
	}
	var email, hash string
	if err := pool.QueryRow(context.Background(), "SELECT email, password_hash FROM users").Scan(&email, &hash); err != nil {
		t.Fatal(err)
	}
	if email != "a@x.com" {
		t.Errorf("email = %q, want lowercased/trimmed", email)
	}
	if !strings.HasPrefix(hash, "$2") {
		t.Errorf("password_hash %q is not bcrypt", hash)
	}
}

func TestRegisterDuplicate(t *testing.T) {
	resetDB(t)
	resp, env := register(t, "a@x.com", "password123", "Alice")
	wantSession(t, resp, env, http.StatusCreated, "Alice")
	for _, email := range []string{"a@x.com", "A@x.com"} {
		t.Run(email, func(t *testing.T) {
			resp, env := register(t, email, "password123", "Alice")
			wantStatus(t, resp, http.StatusConflict)
			wantError(t, env, "CONFLICT")
		})
	}
}

func TestRegisterBadRequest(t *testing.T) {
	resetDB(t)
	tests := []struct {
		name  string
		body  any
		field string
	}{
		{"malformed json", "{not json", ""},
		{"empty body", "", "email"},
		{"missing email", m{"password": "password123", "name": "A"}, "email"},
		{"invalid email", m{"email": "not-an-email", "password": "password123", "name": "A"}, "email"},
		{"missing password", m{"email": "a@x.com", "name": "A"}, "password"},
		{"password 7 bytes", m{"email": "a@x.com", "password": strings.Repeat("a", 7), "name": "A"}, "password"},
		{"password 73 bytes", m{"email": "a@x.com", "password": strings.Repeat("a", 73), "name": "A"}, "password"},
		{"missing name", m{"email": "a@x.com", "password": "password123"}, "name"},
		{"whitespace name", m{"email": "a@x.com", "password": "password123", "name": "   "}, "name"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, env := doJSON(t, http.MethodPost, "/api/users/register", tc.body)
			wantField(t, resp, env, tc.field)
		})
	}
}

func TestLogin(t *testing.T) {
	resetDB(t)
	resp, env := register(t, "a@x.com", "password123", "Alice")
	regTok := wantSession(t, resp, env, http.StatusCreated, "Alice")

	t.Run("correct", func(t *testing.T) {
		resp, env := login(t, "a@x.com", "password123")
		if tok := wantSession(t, resp, env, http.StatusOK, "Alice"); tok == regTok {
			t.Error("login token equals registration token")
		}
	})
	t.Run("email case-insensitive", func(t *testing.T) {
		resp, env := login(t, "A@X.COM", "password123")
		wantSession(t, resp, env, http.StatusOK, "Alice")
	})
	t.Run("wrong password", func(t *testing.T) {
		resp, env := login(t, "a@x.com", "wrongpass1")
		wantStatus(t, resp, http.StatusUnauthorized)
		wantError(t, env, "AUTHENTICATION_ERROR")
		if env.Errors[0].Field != "" {
			t.Errorf("field = %q, want none", env.Errors[0].Field)
		}
	})
	t.Run("unknown email", func(t *testing.T) {
		resp, env := login(t, "nobody@x.com", "password123")
		wantStatus(t, resp, http.StatusUnauthorized)
		wantError(t, env, "AUTHENTICATION_ERROR")
		if env.Errors[0].Field != "" {
			t.Errorf("field = %q, want none", env.Errors[0].Field)
		}
	})
	t.Run("malformed json", func(t *testing.T) {
		resp, env := doJSON(t, http.MethodPost, "/api/users/login", "{bad")
		wantField(t, resp, env, "")
	})
	t.Run("empty password", func(t *testing.T) {
		resp, env := login(t, "a@x.com", "")
		wantField(t, resp, env, "password")
	})
}

func failLogins(t *testing.T, n int) {
	t.Helper()
	for range n {
		resp, _ := login(t, "a@x.com", "wrongpass1")
		wantStatus(t, resp, http.StatusUnauthorized)
	}
}

func TestLoginLockout(t *testing.T) {
	resetDB(t)
	resp, env := register(t, "a@x.com", "password123", "Alice")
	wantSession(t, resp, env, http.StatusCreated, "Alice")

	t.Run("5 failures block correct password", func(t *testing.T) {
		failLogins(t, 5)
		resp, env := login(t, "a@x.com", "password123")
		wantStatus(t, resp, http.StatusTooManyRequests)
		wantError(t, env, "RATE_LIMITED")
	})

	t.Run("success clears counter", func(t *testing.T) {
		resetDB(t)
		resp, env := register(t, "a@x.com", "password123", "Alice")
		wantSession(t, resp, env, http.StatusCreated, "Alice")
		for range 2 {
			failLogins(t, 4)
			resp, env := login(t, "a@x.com", "password123")
			wantSession(t, resp, env, http.StatusOK, "Alice")
		}
	})
}
