package tests

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
)

const accountsPath = "/api/accounts"

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func createAccount(t *testing.T, c *http.Cookie, body any) (*http.Response, envelope) {
	t.Helper()
	return doJSON(t, http.MethodPost, accountsPath, body, c)
}

func TestCreateAccountOK(t *testing.T) {
	resetDB(t)
	c := registerSession(t, "a@x.com", "password123", "Alice")
	resp, env := createAccount(t, c, map[string]any{"name": "PayPal", "currency": "PHP", "balanceInMinorUnits": 1050})
	wantStatus(t, resp, http.StatusCreated)
	if env.Errors != nil {
		t.Fatalf("errors = %+v, want null", env.Errors)
	}
	d := env.Data
	if d["name"] != "PayPal" || d["currency"] != "PHP" || d["balanceInMinorUnits"] != float64(1050) || len(d) != 4 {
		t.Fatalf("got %+v", d)
	}
	if id, _ := d["id"].(string); !uuidRe.MatchString(id) {
		t.Fatalf("id = %v, want uuid string", d["id"])
	}
}

func TestCreateAccountDefaultBalance(t *testing.T) {
	resetDB(t)
	c := registerSession(t, "a@x.com", "password123", "Alice")
	resp, env := createAccount(t, c, map[string]any{"name": "Cash", "currency": "USD"})
	wantStatus(t, resp, http.StatusCreated)
	if env.Data["balanceInMinorUnits"] != float64(0) {
		t.Fatalf("got %+v", env.Data)
	}
}

func TestCreateAccountValidation(t *testing.T) {
	resetDB(t)
	c := registerSession(t, "a@x.com", "password123", "Alice")
	for name, tc := range map[string]struct {
		body  any
		field string
	}{
		"empty name":       {map[string]any{"name": "", "currency": "PHP"}, "name"},
		"whitespace name":  {map[string]any{"name": "   ", "currency": "PHP"}, "name"},
		"long name":        {map[string]any{"name": strings.Repeat("a", 101), "currency": "PHP"}, "name"},
		"nul in name":      {map[string]any{"name": "A\x00B", "currency": "PHP"}, "name"},
		"missing currency": {map[string]any{"name": "A"}, "currency"},
		"unknown currency": {map[string]any{"name": "A", "currency": "ZZZ"}, "currency"},
		"lowercase":        {map[string]any{"name": "A", "currency": "php"}, "currency"},
		"short currency":   {map[string]any{"name": "A", "currency": "US"}, "currency"},
		"negative balance": {map[string]any{"name": "A", "currency": "PHP", "balanceInMinorUnits": -1}, "balanceInMinorUnits"},
		"malformed json":   {`{"name":`, ""},
		"float balance":    {`{"name":"A","currency":"PHP","balanceInMinorUnits":1.5}`, ""},
	} {
		t.Run(name, func(t *testing.T) {
			resp, env := createAccount(t, c, tc.body)
			wantStatus(t, resp, http.StatusBadRequest)
			wantError(t, env, "VALIDATION_ERROR")
			if env.Errors[0].Field != tc.field {
				t.Fatalf("field = %q, want %q", env.Errors[0].Field, tc.field)
			}
		})
	}
}

func TestCreateAccountConflict(t *testing.T) {
	resetDB(t)
	ca := registerSession(t, "a@x.com", "password123", "Alice")
	cb := registerSession(t, "b@x.com", "password123", "Bob")
	body := map[string]any{"name": "PayPal", "currency": "USD"}

	resp, _ := createAccount(t, ca, body)
	wantStatus(t, resp, http.StatusCreated)
	resp, env := createAccount(t, ca, body)
	wantStatus(t, resp, http.StatusConflict)
	wantError(t, env, "CONFLICT")

	resp, _ = createAccount(t, ca, map[string]any{"name": "paypal", "currency": "USD"})
	wantStatus(t, resp, http.StatusCreated)
	resp, _ = createAccount(t, cb, body)
	wantStatus(t, resp, http.StatusCreated)
}

func TestListAccountsEmpty(t *testing.T) {
	resetDB(t)
	c := registerSession(t, "a@x.com", "password123", "Alice")
	resp, env := doJSON(t, http.MethodGet, accountsPath, nil, c)
	wantStatus(t, resp, http.StatusOK)
	accts, ok := env.Data["accounts"].([]any)
	if !ok || len(accts) != 0 || env.Errors != nil {
		t.Fatalf("got %+v, want accounts=[]", env)
	}
}

func TestListAccountsOwnInOrder(t *testing.T) {
	resetDB(t)
	ca := registerSession(t, "a@x.com", "password123", "Alice")
	cb := registerSession(t, "b@x.com", "password123", "Bob")
	createAccount(t, ca, map[string]any{"name": "Bank", "currency": "PHP", "balanceInMinorUnits": 1050})
	createAccount(t, cb, map[string]any{"name": "Other", "currency": "USD"})
	createAccount(t, ca, map[string]any{"name": "Yen", "currency": "JPY", "balanceInMinorUnits": 1050})

	resp, env := doJSON(t, http.MethodGet, accountsPath, nil, ca)
	wantStatus(t, resp, http.StatusOK)
	accts, _ := env.Data["accounts"].([]any)
	if len(accts) != 2 {
		t.Fatalf("got %+v", env.Data)
	}
	for i, want := range []struct {
		name, cur string
		bal       float64
	}{{"Bank", "PHP", 1050}, {"Yen", "JPY", 1050}} {
		a := accts[i].(map[string]any)
		id, _ := a["id"].(string)
		if a["name"] != want.name || a["currency"] != want.cur || a["balanceInMinorUnits"] != want.bal || !uuidRe.MatchString(id) || len(a) != 4 {
			t.Fatalf("account %d = %+v", i, a)
		}
	}
}

func TestAccountsUnauthenticated(t *testing.T) {
	resetDB(t)
	expired := registerSession(t, "a@x.com", "password123", "Alice")
	if _, err := pool.Exec(context.Background(), "UPDATE sessions SET expires_at = now() - interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		for name, c := range map[string]*http.Cookie{
			"no cookie": nil,
			"garbage":   {Name: "session", Value: "garbage"},
			"expired":   expired,
		} {
			t.Run(method+" "+name, func(t *testing.T) {
				var cookies []*http.Cookie
				if c != nil {
					cookies = append(cookies, c)
				}
				resp, env := doJSON(t, method, accountsPath, map[string]any{"name": "A", "currency": "PHP"}, cookies...)
				wantStatus(t, resp, http.StatusUnauthorized)
				wantError(t, env, "AUTHENTICATION_ERROR")
			})
		}
	}
}

func TestAccountsMethodNotAllowed(t *testing.T) {
	resp, env := doJSON(t, http.MethodDelete, accountsPath, nil)
	wantStatus(t, resp, http.StatusMethodNotAllowed)
	wantError(t, env, "METHOD_NOT_ALLOWED")
}

func deleteAccount(t *testing.T, id string, c ...*http.Cookie) (*http.Response, envelope) {
	t.Helper()
	return doJSON(t, http.MethodDelete, accountsPath+"/"+id, nil, c...)
}

func listLen(t *testing.T, c *http.Cookie) int {
	t.Helper()
	_, env := doJSON(t, http.MethodGet, accountsPath, nil, c)
	accts, _ := env.Data["accounts"].([]any)
	return len(accts)
}

func TestDeleteAccountOK(t *testing.T) {
	resetDB(t)
	c := registerSession(t, "a@x.com", "password123", "Alice")
	_, created := createAccount(t, c, map[string]any{"name": "PayPal", "currency": "PHP", "balanceInMinorUnits": 1050})
	id, _ := created.Data["id"].(string)

	resp, env := deleteAccount(t, id, c)
	wantStatus(t, resp, http.StatusOK)
	d := env.Data
	if env.Errors != nil || d["id"] != id || d["name"] != "PayPal" || d["currency"] != "PHP" || d["balanceInMinorUnits"] != float64(1050) || len(d) != 4 {
		t.Fatalf("got %+v", env)
	}
	if n := listLen(t, c); n != 0 {
		t.Fatalf("list len = %d, want 0", n)
	}
	resp, env = deleteAccount(t, id, c)
	wantStatus(t, resp, http.StatusNotFound)
	wantError(t, env, "NOT_FOUND")
}

func TestDeleteAccountOtherUser(t *testing.T) {
	resetDB(t)
	ca := registerSession(t, "a@x.com", "password123", "Alice")
	cb := registerSession(t, "b@x.com", "password123", "Bob")
	_, created := createAccount(t, ca, map[string]any{"name": "PayPal", "currency": "PHP"})
	id, _ := created.Data["id"].(string)

	resp, env := deleteAccount(t, id, cb)
	wantStatus(t, resp, http.StatusNotFound)
	wantError(t, env, "NOT_FOUND")
	if n := listLen(t, ca); n != 1 {
		t.Fatalf("list len = %d, want 1", n)
	}
}

func TestDeleteAccountUnknownID(t *testing.T) {
	resetDB(t)
	c := registerSession(t, "a@x.com", "password123", "Alice")
	resp, env := deleteAccount(t, uuid.NewString(), c)
	wantStatus(t, resp, http.StatusNotFound)
	wantError(t, env, "NOT_FOUND")
}

func TestDeleteAccountInvalidID(t *testing.T) {
	resetDB(t)
	c := registerSession(t, "a@x.com", "password123", "Alice")
	v4 := uuid.NewString()
	for name, id := range map[string]string{
		"abc":        "abc",
		"digits":     "123",
		"no hyphens": strings.ReplaceAll(v4, "-", ""),
		"v1":         "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		"uppercase":  strings.ToUpper(v4),
		"braces":     url.PathEscape("{" + v4 + "}"),
	} {
		t.Run(name, func(t *testing.T) {
			resp, env := deleteAccount(t, id, c)
			wantField(t, resp, env, "id")
		})
	}
}

func TestDeleteAccountUnauthenticated(t *testing.T) {
	resetDB(t)
	expired := registerSession(t, "a@x.com", "password123", "Alice")
	if _, err := pool.Exec(context.Background(), "UPDATE sessions SET expires_at = now() - interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]*http.Cookie{
		"no cookie": nil,
		"garbage":   {Name: "session", Value: "garbage"},
		"expired":   expired,
	} {
		for _, id := range []string{uuid.NewString(), "abc"} {
			t.Run(name+" "+id, func(t *testing.T) {
				var cookies []*http.Cookie
				if c != nil {
					cookies = append(cookies, c)
				}
				resp, env := deleteAccount(t, id, cookies...)
				wantStatus(t, resp, http.StatusUnauthorized)
				wantError(t, env, "AUTHENTICATION_ERROR")
			})
		}
	}
}
