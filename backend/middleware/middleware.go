package middleware

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"log"
	"net/http"

	"backend/internal/database"
	"backend/services"
)

// HashToken is the only place session tokens are hashed.
func HashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

type Session struct{ ID, UserID int64 }

type ctxKey struct{}

func SessionFromContext(ctx context.Context) (Session, bool) {
	s, ok := ctx.Value(ctxKey{}).(Session)
	return s, ok
}

// RequireSession authenticates via the `session` cookie only and stores the Session in the request context.
func RequireSession(q *database.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie("session")
			if err != nil || c.Value == "" {
				services.WriteError(w, services.APIError{Type: services.TypeAuthentication, Message: "authentication required"})
				return
			}
			row, err := q.GetActiveSession(r.Context(), HashToken(c.Value))
			if errors.Is(err, sql.ErrNoRows) {
				services.WriteError(w, services.APIError{Type: services.TypeAuthentication, Message: "invalid or expired session"})
				return
			}
			if err != nil {
				log.Printf("middleware: %v", err)
				services.WriteError(w, services.APIError{Type: services.TypeInternal, Message: "internal server error"})
				return
			}
			ctx := context.WithValue(r.Context(), ctxKey{}, Session{ID: row.ID, UserID: row.UserID})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// JSONErrors rewrites ServeMux's plain-text 404/405 responses into the error envelope.
func JSONErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&jsonErrorWriter{ResponseWriter: w}, r)
	})
}

type jsonErrorWriter struct {
	http.ResponseWriter
	swallow bool
}

func (w *jsonErrorWriter) WriteHeader(code int) {
	if (code == http.StatusNotFound || code == http.StatusMethodNotAllowed) &&
		w.Header().Get("Content-Type") == "text/plain; charset=utf-8" {
		w.swallow = true
		w.Header().Del("X-Content-Type-Options")
		e := services.APIError{Type: services.TypeNotFound, Message: "not found"}
		if code == http.StatusMethodNotAllowed {
			e = services.APIError{Type: services.TypeMethodNotAllowed, Message: "method not allowed"}
		}
		services.WriteError(w.ResponseWriter, e)
		return
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *jsonErrorWriter) Write(b []byte) (int, error) {
	if w.swallow {
		return len(b), nil
	}
	return w.ResponseWriter.Write(b)
}

func (w *jsonErrorWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
