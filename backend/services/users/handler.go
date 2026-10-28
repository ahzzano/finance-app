package users

import (
	"backend/services"
	"errors"
	"log"
	"net/http"
)

var (
	ErrorEmailExists        = errors.New("email already registered")
	ErrorInvalidCredentials = errors.New("invalid email or password")
	ErrorLoginBlocked       = errors.New("too many failed login attempts, try again later")
	ErrorInvalidInput       = errors.New("invalid input")
)

type UserHandler struct {
	users *UserService
}

func NewUserHandler(users *UserService) *UserHandler {
	return &UserHandler{users: users}
}

func (h *UserHandler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("PUT /api/users/register", h.register)
	mux.HandleFunc("POST /api/users/login", h.login)
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenData struct {
	Token string `json:"token"`
}

type tokenResponse struct {
	Data   tokenData `json:"data"`
	Errors []string  `json:"errors"`
}

// register godoc
//
//	@Summary	Register a user
//	@Tags		users
//	@Accept		json
//	@Produce	json
//	@Param		body	body		registerRequest	true	"Registration details"
//	@Success	200		{object}	tokenResponse
//	@Failure	400		{object}	services.ErrorResponse
//	@Failure	409		{object}	services.ErrorResponse
//	@Failure	500		{object}	services.ErrorResponse
//	@Router		/api/users/register [put]
func (h *UserHandler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !services.Decode(w, r, &req) {
		return
	}
	token, err := h.users.Register(r.Context(), req.Email, req.Password, req.Name)
	respond(w, token, err)
}

// login godoc
//
//	@Summary	Log in
//	@Tags		users
//	@Accept		json
//	@Produce	json
//	@Param		body	body		loginRequest	true	"Credentials"
//	@Success	200		{object}	tokenResponse
//	@Failure	400		{object}	services.ErrorResponse
//	@Failure	401		{object}	services.ErrorResponse
//	@Failure	403		{object}	services.ErrorResponse
//	@Failure	500		{object}	services.ErrorResponse
//	@Router		/api/users/login [post]
func (h *UserHandler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !services.Decode(w, r, &req) {
		return
	}
	token, err := h.users.Login(r.Context(), req.Email, req.Password)
	respond(w, token, err)
}

func respond(w http.ResponseWriter, token string, err error) {
	if err == nil {
		http.SetCookie(w, &http.Cookie{
			Name: "session", Value: token, Path: "/", MaxAge: int(sessionTTL.Seconds()),
			HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		})
		services.WriteJSON(w, http.StatusOK, tokenResponse{Data: tokenData{Token: token}})
		return
	}
	status, msg := http.StatusInternalServerError, "internal server error"
	switch {
	case errors.Is(err, ErrorInvalidInput):
		status, msg = http.StatusBadRequest, err.Error()
	case errors.Is(err, ErrorEmailExists):
		status, msg = http.StatusConflict, err.Error()
	case errors.Is(err, ErrorInvalidCredentials):
		status, msg = http.StatusUnauthorized, err.Error()
	case errors.Is(err, ErrorLoginBlocked):
		status, msg = http.StatusForbidden, err.Error()
	default:
		log.Printf("users: %v", err)
	}
	services.WriteJSON(w, status, services.ErrorResponse{Errors: []string{msg}})
}
