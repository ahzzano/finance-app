package tests

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"backend/cmd"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// Migrations must already be applied:
// goose -dir internal/database/migrations postgres "$TEST_DATABASE_URL" up
var (
	pool *pgxpool.Pool
	srv  *httptest.Server
)

func TestMain(m *testing.M) {
	_ = godotenv.Load("../.env")
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		fmt.Println("TEST_DATABASE_URL not set; skipping integration tests")
		os.Exit(0)
	}
	var err error
	pool, err = pgxpool.New(context.Background(), url)
	if err != nil {
		fmt.Println("db connect:", err)
		os.Exit(1)
	}
	api := cmd.NewAPI(cmd.APIConfig{DB: pool})
	api.Mount()
	srv = httptest.NewServer(api.Handler())
	code := m.Run()
	srv.Close()
	pool.Close()
	os.Exit(code)
}

func resetDB(t *testing.T) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), "TRUNCATE users, sessions, failed_logins RESTART IDENTITY CASCADE"); err != nil {
		t.Fatal(err)
	}
}

type envelope struct {
	Data   map[string]any `json:"data"`
	Errors []string       `json:"errors"`
}

// doJSON sends body (string sent raw) and decodes the envelope.
func doJSON(t *testing.T, method, path string, body any) (*http.Response, envelope) {
	t.Helper()
	var r io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		r = strings.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		r = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, srv.URL+path, r)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var env envelope
	raw, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(raw, &env)
	return resp, env
}

type m = map[string]string

func register(t *testing.T, email, pw, name string) (*http.Response, envelope) {
	t.Helper()
	return doJSON(t, http.MethodPut, "/api/users/register", m{"email": email, "password": pw, "name": name})
}

func login(t *testing.T, email, pw string) (*http.Response, envelope) {
	t.Helper()
	return doJSON(t, http.MethodPost, "/api/users/login", m{"email": email, "password": pw})
}

func wantStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("status = %d, want %d", resp.StatusCode, want)
	}
}

// wantError asserts the error envelope: data null, errors non-empty.
func wantError(t *testing.T, env envelope) {
	t.Helper()
	if env.Data != nil || len(env.Errors) == 0 {
		t.Fatalf("want data=null and non-empty errors, got %+v", env)
	}
}

func wantToken(t *testing.T, resp *http.Response, env envelope) string {
	t.Helper()
	wantStatus(t, resp, http.StatusOK)
	tok, _ := env.Data["token"].(string)
	if tok == "" || env.Errors != nil {
		t.Fatalf("want token and null errors, got %+v", env)
	}
	for _, c := range resp.Cookies() {
		if c.Name == "session" {
			if c.Value != tok {
				t.Fatalf("cookie %q != token %q", c.Value, tok)
			}
			return tok
		}
	}
	t.Fatal("no session cookie")
	return ""
}

func TestRegister(t *testing.T) {
	resetDB(t)
	resp, env := register(t, "  A@X.com ", "password123", "Alice")
	tok := wantToken(t, resp, env)

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
	wantToken(t, resp, env)
	for _, email := range []string{"a@x.com", "A@x.com"} {
		t.Run(email, func(t *testing.T) {
			resp, env := register(t, email, "password123", "Alice")
			wantStatus(t, resp, http.StatusConflict)
			wantError(t, env)
		})
	}
}

func TestRegisterBadRequest(t *testing.T) {
	resetDB(t)
	tests := []struct {
		name string
		body any
	}{
		{"malformed json", "{not json"},
		{"empty body", ""},
		{"missing email", m{"password": "password123", "name": "A"}},
		{"missing password", m{"email": "a@x.com", "name": "A"}},
		{"missing name", m{"email": "a@x.com", "password": "password123"}},
		{"invalid email", m{"email": "not-an-email", "password": "password123", "name": "A"}},
		{"password 7 bytes", m{"email": "a@x.com", "password": strings.Repeat("a", 7), "name": "A"}},
		{"password 73 bytes", m{"email": "a@x.com", "password": strings.Repeat("a", 73), "name": "A"}},
		{"whitespace name", m{"email": "a@x.com", "password": "password123", "name": "   "}},
	}
	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, env := doJSON(t, http.MethodPut, "/api/users/register", tc.body)
			wantStatus(t, resp, http.StatusBadRequest)
			if i == 0 {
				wantError(t, env)
			}
		})
	}
}

func TestMethodNotAllowed(t *testing.T) {
	for _, p := range []string{"/api/users/register", "/api/users/login"} {
		t.Run(p, func(t *testing.T) {
			resp, _ := doJSON(t, http.MethodGet, p, nil)
			wantStatus(t, resp, http.StatusMethodNotAllowed)
		})
	}
}

func TestLogin(t *testing.T) {
	resetDB(t)
	resp, env := register(t, "a@x.com", "password123", "Alice")
	regTok := wantToken(t, resp, env)

	t.Run("correct", func(t *testing.T) {
		resp, env := login(t, "a@x.com", "password123")
		if tok := wantToken(t, resp, env); tok == regTok {
			t.Error("login token equals registration token")
		}
	})
	t.Run("email case-insensitive", func(t *testing.T) {
		resp, env := login(t, "A@X.COM", "password123")
		wantToken(t, resp, env)
	})
	t.Run("wrong password", func(t *testing.T) {
		resp, env := login(t, "a@x.com", "wrongpass1")
		wantStatus(t, resp, http.StatusUnauthorized)
		wantError(t, env)
	})
	t.Run("unknown email", func(t *testing.T) {
		resp, env := login(t, "nobody@x.com", "password123")
		wantStatus(t, resp, http.StatusUnauthorized)
		wantError(t, env)
	})
	t.Run("malformed json", func(t *testing.T) {
		resp, _ := doJSON(t, http.MethodPost, "/api/users/login", "{bad")
		wantStatus(t, resp, http.StatusBadRequest)
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
	wantToken(t, resp, env)

	t.Run("5 failures block correct password", func(t *testing.T) {
		failLogins(t, 5)
		resp, env := login(t, "a@x.com", "password123")
		wantStatus(t, resp, http.StatusForbidden)
		wantError(t, env)
	})

	t.Run("success clears counter", func(t *testing.T) {
		resetDB(t)
		resp, env := register(t, "a@x.com", "password123", "Alice")
		wantToken(t, resp, env)
		for range 2 {
			failLogins(t, 4)
			resp, env := login(t, "a@x.com", "password123")
			wantToken(t, resp, env)
		}
	})
}
