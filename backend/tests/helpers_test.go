package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"backend/cmd"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// Tests run in the isolated `test` schema (resetDB truncates tables), so they can share the prod database.
// TEST_DATABASE_URL must be a session-mode connection (Supabase pooler port 5432, not 6543) with
// search_path=test, and migrations must already be applied:
// goose -dir internal/database/migrations postgres "$TEST_DATABASE_URL" up
const testSchema = "test"

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
	ctx := context.Background()
	var err error
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		fmt.Println("db config:", err)
		os.Exit(1)
	}
	// forced in code, not just the URL, so a bad URL can never truncate prod tables in public
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		_, err := c.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+testSchema+"; SET search_path TO "+testSchema)
		return err
	}
	pool, err = pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		fmt.Println("db connect:", err)
		os.Exit(1)
	}
	// session-level advisory lock serializes concurrent `go test` processes on the shared test DB
	lockConn, err := pool.Acquire(ctx)
	if err != nil {
		fmt.Println("db acquire:", err)
		os.Exit(1)
	}
	if _, err := lockConn.Exec(ctx, "SELECT pg_advisory_lock(424242)"); err != nil {
		fmt.Println("advisory lock:", err)
		os.Exit(1)
	}
	api := cmd.NewAPI(cmd.APIConfig{DB: pool})
	api.Mount()
	srv = httptest.NewServer(api.Handler())
	code := m.Run()
	srv.Close()
	_, _ = lockConn.Exec(ctx, "SELECT pg_advisory_unlock(424242)")
	lockConn.Release()
	pool.Close()
	os.Exit(code)
}

func resetDB(t *testing.T) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), "TRUNCATE users, sessions, failed_logins, accounts RESTART IDENTITY CASCADE"); err != nil {
		t.Fatal(err)
	}
}

type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Field   string `json:"field"`
}

type envelope struct {
	Data   map[string]any `json:"data"`
	Errors []apiError     `json:"errors"`
}

// doJSON sends body (string sent raw) and decodes the envelope.
func doJSON(t *testing.T, method, path string, body any, cookies ...*http.Cookie) (*http.Response, envelope) {
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
	for _, c := range cookies {
		req.AddCookie(c)
	}
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
	return doJSON(t, http.MethodPost, "/api/users/register", m{"email": email, "password": pw, "name": name})
}

func login(t *testing.T, email, pw string) (*http.Response, envelope) {
	t.Helper()
	return doJSON(t, http.MethodPost, "/api/users/login", m{"email": email, "password": pw})
}

// sessionCookie returns the response's `session` cookie or nil.
func sessionCookie(resp *http.Response) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == "session" {
			return c
		}
	}
	return nil
}

// registerSession registers a user and returns its session cookie.
func registerSession(t *testing.T, email, pw, name string) *http.Cookie {
	t.Helper()
	resp, _ := register(t, email, pw, name)
	wantStatus(t, resp, http.StatusCreated)
	c := sessionCookie(resp)
	if c == nil || c.Value == "" {
		t.Fatal("no session cookie")
	}
	return c
}

func wantStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("status = %d, want %d", resp.StatusCode, want)
	}
}

// wantError asserts the error envelope: data null, first error of wantType.
func wantError(t *testing.T, env envelope, wantType string) {
	t.Helper()
	if env.Data != nil || len(env.Errors) == 0 || env.Errors[0].Type != wantType {
		t.Fatalf("want data=null and error type %s, got %+v", wantType, env)
	}
}
