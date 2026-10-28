package accounts

import (
	"errors"
	"log"
	"net/http"

	"backend/middleware"
	"backend/services"
)

var (
	ErrorAccountNameExists = errors.New("account name already exists")
	ErrorSessionNotFound   = errors.New("invalid or expired session")
	ErrorAccountNotFound   = errors.New("account not found")
)

type AccountHandler struct {
	accounts       *AccountService
	requireSession func(http.Handler) http.Handler
}

func NewAccountHandler(accounts *AccountService, requireSession func(http.Handler) http.Handler) *AccountHandler {
	return &AccountHandler{accounts: accounts, requireSession: requireSession}
}

func (h *AccountHandler) Mount(mux *http.ServeMux) {
	mux.Handle("POST /api/accounts", h.requireSession(http.HandlerFunc(h.create)))
	mux.Handle("GET /api/accounts", h.requireSession(http.HandlerFunc(h.list)))
	mux.Handle("PATCH /api/accounts/{id}", h.requireSession(http.HandlerFunc(h.update)))
	mux.Handle("DELETE /api/accounts/{id}", h.requireSession(http.HandlerFunc(h.delete)))
}

type createRequest struct {
	Name                string `json:"name"`
	Currency            string `json:"currency"`
	BalanceInMinorUnits int64  `json:"balanceInMinorUnits"`
}

type accountData struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Currency            string `json:"currency"`
	BalanceInMinorUnits int64  `json:"balanceInMinorUnits"`
}

type accountResponse struct {
	Data   accountData         `json:"data"`
	Errors []services.APIError `json:"errors"`
}

// create godoc
//
//	@Summary	Create an account
//	@Tags		accounts
//	@Accept		json
//	@Produce	json
//	@Param		body	body		createRequest	true	"Account details"
//	@Success	201		{object}	accountResponse
//	@Failure	400		{object}	services.ErrorResponse
//	@Failure	401		{object}	services.ErrorResponse
//	@Failure	409		{object}	services.ErrorResponse
//	@Failure	500		{object}	services.ErrorResponse
//	@Router		/api/accounts [post]
func (h *AccountHandler) create(w http.ResponseWriter, r *http.Request) {
	sess, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		writeError(w, ErrorSessionNotFound)
		return
	}
	var req createRequest
	if !services.Decode(w, r, &req) {
		return
	}
	a, err := h.accounts.CreateAccount(r.Context(), sess.UserID, req.Name, req.Currency, req.BalanceInMinorUnits)
	if err != nil {
		writeError(w, err)
		return
	}
	services.WriteData(w, http.StatusCreated, accountData(a))
}

type listData struct {
	Accounts []accountData `json:"accounts"`
}

type listResponse struct {
	Data   listData            `json:"data"`
	Errors []services.APIError `json:"errors"`
}

// list godoc
//
//	@Summary	List the user's accounts
//	@Tags		accounts
//	@Produce	json
//	@Success	200	{object}	listResponse
//	@Failure	401	{object}	services.ErrorResponse
//	@Failure	500	{object}	services.ErrorResponse
//	@Router		/api/accounts [get]
func (h *AccountHandler) list(w http.ResponseWriter, r *http.Request) {
	sess, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		writeError(w, ErrorSessionNotFound)
		return
	}
	rows, err := h.accounts.ListAccounts(r.Context(), sess.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]accountData, 0, len(rows)) // non-nil so an empty list encodes as []
	for _, a := range rows {
		out = append(out, accountData(a))
	}
	services.WriteData(w, http.StatusOK, listData{Accounts: out})
}

type updateRequest struct {
	Name                *string `json:"name"`
	Currency            *string `json:"currency"`
	BalanceInMinorUnits *int64  `json:"balanceInMinorUnits"`
}

// update godoc
//
//	@Summary	Update an account
//	@Tags		accounts
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string			true	"Account ID (uuid4)"
//	@Param		body	body		updateRequest	false	"Fields to update"
//	@Success	200		{object}	accountResponse
//	@Failure	400		{object}	services.ErrorResponse
//	@Failure	401		{object}	services.ErrorResponse
//	@Failure	404		{object}	services.ErrorResponse
//	@Failure	409		{object}	services.ErrorResponse
//	@Failure	500		{object}	services.ErrorResponse
//	@Router		/api/accounts/{id} [patch]
func (h *AccountHandler) update(w http.ResponseWriter, r *http.Request) {
	sess, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		writeError(w, ErrorSessionNotFound)
		return
	}
	var req updateRequest
	if !services.Decode(w, r, &req) {
		return
	}
	a, err := h.accounts.UpdateAccount(r.Context(), sess.UserID, r.PathValue("id"), req.Name, req.Currency, req.BalanceInMinorUnits)
	if err != nil {
		writeError(w, err)
		return
	}
	services.WriteData(w, http.StatusOK, accountData(a))
}

// delete godoc
//
//	@Summary	Delete an account
//	@Tags		accounts
//	@Produce	json
//	@Param		id	path		string	true	"Account ID (uuid4)"
//	@Success	200	{object}	accountResponse
//	@Failure	400	{object}	services.ErrorResponse
//	@Failure	401	{object}	services.ErrorResponse
//	@Failure	404	{object}	services.ErrorResponse
//	@Failure	500	{object}	services.ErrorResponse
//	@Router		/api/accounts/{id} [delete]
func (h *AccountHandler) delete(w http.ResponseWriter, r *http.Request) {
	sess, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		writeError(w, ErrorSessionNotFound)
		return
	}
	a, err := h.accounts.DeleteAccount(r.Context(), sess.UserID, r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	services.WriteData(w, http.StatusOK, accountData(a))
}

func writeError(w http.ResponseWriter, err error) {
	var ve services.ValidationError
	switch {
	case errors.As(err, &ve):
		services.WriteError(w, services.APIError{Type: services.TypeValidation, Message: ve.Message, Field: ve.Field})
	case errors.Is(err, ErrorAccountNameExists):
		services.WriteError(w, services.APIError{Type: services.TypeConflict, Message: err.Error()})
	case errors.Is(err, ErrorSessionNotFound):
		services.WriteError(w, services.APIError{Type: services.TypeAuthentication, Message: err.Error()})
	case errors.Is(err, ErrorAccountNotFound):
		services.WriteError(w, services.APIError{Type: services.TypeNotFound, Message: err.Error()})
	default:
		log.Printf("accounts: %v", err)
		services.WriteError(w, services.APIError{Type: services.TypeInternal, Message: "internal server error"})
	}
}
