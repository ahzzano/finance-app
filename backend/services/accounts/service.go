package accounts

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"

	"backend/internal/database"
	"backend/services"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/text/currency"
)

type AccountService struct {
	q *database.Queries
}

func NewAccountService(db *sql.DB) *AccountService {
	return &AccountService{q: database.New(db)}
}

// validCurrency accepts only exact uppercase ISO 4217 codes such as "PHP".
func validCurrency(code string) bool {
	if len(code) != 3 || code != strings.ToUpper(code) {
		return false
	}
	_, err := currency.ParseISO(code)
	return err == nil
}

func (s *AccountService) CreateAccount(ctx context.Context, userID int64, name, cur string, balance int64) (database.CreateAccountRow, error) {
	name = strings.TrimSpace(name)
	// Postgres TEXT rejects NUL bytes, which would otherwise surface as a 500
	if n := utf8.RuneCountInString(name); n < 1 || n > 100 || strings.ContainsRune(name, 0) {
		return database.CreateAccountRow{}, services.ValidationError{Field: "name", Message: "name must be 1-100 characters"}
	}
	if !validCurrency(cur) {
		return database.CreateAccountRow{}, services.ValidationError{Field: "currency", Message: "currency must be an uppercase ISO 4217 code"}
	}
	if balance < 0 {
		return database.CreateAccountRow{}, services.ValidationError{Field: "balanceInMinorUnits", Message: "balanceInMinorUnits must not be negative"}
	}
	a, err := s.q.CreateAccount(ctx, database.CreateAccountParams{UserID: userID, Name: name, Currency: cur, BalanceInMinorUnits: balance})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return a, ErrorAccountNameExists
	}
	return a, err
}

func (s *AccountService) ListAccounts(ctx context.Context, userID int64) ([]database.ListAccountsByUserRow, error) {
	return s.q.ListAccountsByUser(ctx, userID)
}
