package users

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"backend/internal/database"
	"backend/middleware"
	"backend/services"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptCost     = 12
	sessionTTL     = 7 * 24 * time.Hour
	lockoutWindow  = 15 * time.Minute
	lockoutMaxFail = 5
)

// dummyHash equalizes timing when the email is unknown.
var dummyHash = func() []byte {
	h, err := bcrypt.GenerateFromPassword([]byte("dummy-password"), bcryptCost)
	if err != nil {
		panic(err)
	}
	return h
}()

type UserService struct {
	db *sql.DB
	q  *database.Queries
}

func NewUserService(db *sql.DB) *UserService {
	return &UserService{db: db, q: database.New(db)}
}

// inTx runs fn in a transaction; it commits unless fn returns an error.
func (s *UserService) inTx(ctx context.Context, fn func(q *database.Queries) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit()
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", services.ValidationError{Field: "email", Message: "invalid email"}
	}
	return email, nil
}

func (s *UserService) Register(ctx context.Context, email, password, name string) (string, string, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return "", "", err
	}
	if len(password) < 8 || len(password) > 72 {
		return "", "", services.ValidationError{Field: "password", Message: "password must be 8-72 bytes"}
	}
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 100 {
		return "", "", services.ValidationError{Field: "name", Message: "name must be 1-100 characters"}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", "", err
	}
	var token string
	err = s.inTx(ctx, func(q *database.Queries) error {
		u, err := q.CreateUser(ctx, database.CreateUserParams{Email: email, Name: name, PasswordHash: string(hash)})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrorEmailExists
			}
			return err
		}
		token, err = createSession(ctx, q, u.ID)
		return err
	})
	return token, name, err
}

func (s *UserService) Login(ctx context.Context, email, password string) (string, string, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return "", "", err
	}
	if password == "" || len(password) > 72 {
		return "", "", services.ValidationError{Field: "password", Message: "invalid password"}
	}
	var token, name string
	var authErr error // returned after commit so the failed_logins insert persists
	err = s.inTx(ctx, func(q *database.Queries) error {
		// serialize attempts per email so concurrent guesses can't bypass the lockout
		if err := q.LockEmail(ctx, email); err != nil {
			return err
		}
		// shortcut: lockout is per-email only so an attacker can lock out a victim; add an IP dimension when abuse appears
		count, err := q.CountRecentFailedLogins(ctx, database.CountRecentFailedLoginsParams{Email: email, Since: time.Now().Add(-lockoutWindow)})
		if err != nil {
			return err
		}
		if count >= lockoutMaxFail {
			authErr = ErrorLoginBlocked
			return nil
		}
		u, err := q.GetUserByEmail(ctx, email)
		if errors.Is(err, sql.ErrNoRows) {
			_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		} else if err != nil {
			return err
		} else if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil {
			if _, err := q.DeleteFailedLogins(ctx, email); err != nil {
				return err
			}
			name = u.Name
			token, err = createSession(ctx, q, u.ID)
			return err
		}
		authErr = ErrorInvalidCredentials
		return q.CreateFailedLogin(ctx, email)
	})
	if err != nil {
		return "", "", err
	}
	return token, name, authErr
}

func createSession(ctx context.Context, q *database.Queries, userID int64) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	_, err := q.CreateSession(ctx, database.CreateSessionParams{UserID: userID, TokenHash: middleware.HashToken(token), ExpiresAt: time.Now().Add(sessionTTL)})
	if err != nil {
		return "", err
	}
	return token, nil
}

// Logout deletes the session; a concurrent double logout yields ErrorSessionNotFound.
func (s *UserService) Logout(ctx context.Context, sessionID int64) error {
	if _, err := s.q.DeleteSession(ctx, sessionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrorSessionNotFound
		}
		return err
	}
	return nil
}

// GetProfile returns the user's profile.
func (s *UserService) GetProfile(ctx context.Context, userID int64) (database.GetUserProfileRow, error) {
	p, err := s.q.GetUserProfile(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrorSessionNotFound // user vanished
	}
	return p, err
}
