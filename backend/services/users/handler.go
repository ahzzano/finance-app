package users

import (
	"backend/middleware"
	"backend/services"
	"errors"
	"log"
	"net/http"
)

var (
	ErrorEmailExists        = errors.New("email already registered")
	ErrorInvalidCredentials = errors.New("invalid email or password")
	ErrorLoginBlocked       = errors.New("too many failed login attempts, try again later")
	ErrorSessionNotFound    = errors.New("invalid or expired session")
)

type UserHandler struct {
	users          *UserService
	requireSession func(http.Handler) http.Handler
}

func NewUserHandler(users *UserService, requireSession func(http.Handler) http.Handler) *UserHandler {
	return &UserHandler{users: users, requireSession: requireSession}
}

func (h *UserHandler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/users/register", h.register)
	mux.HandleFunc("POST /api/users/login", h.login)
	mux.Handle("POST /api/users/logout", h.requireSession(http.HandlerFunc(h.logout)))
	mux.Handle("GET /api/users/profile", h.requireSession(http.HandlerFunc(h.profile)))
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

type nameData struct {
	Name string `json:"name"`
}

type nameResponse struct {
	Data   nameData            `json:"data"`
	Errors []services.APIError `json:"errors"`
}

// register godoc
//
//	@Summary	Register a user
//	@Tags		users
//	@Accept		json
//	@Produce	json
//	@Param		body	body		registerRequest	true	"Registration details"
//	@Success	201		{object}	nameResponse
//	@Failure	400		{object}	services.ErrorResponse
//	@Failure	409		{object}	services.ErrorResponse
//	@Failure	500		{object}	services.ErrorResponse
//	@Router		/api/users/register [post]
func (h *UserHandler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !services.Decode(w, r, &req) {
		return
	}
	token, name, err := h.users.Register(r.Context(), req.Email, req.Password, req.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	setSessionCookie(w, token)
	services.WriteData(w, http.StatusCreated, nameData{Name: name})
}

// login godoc
//
//	@Summary	Log in
//	@Tags		users
//	@Accept		json
//	@Produce	json
//	@Param		body	body		loginRequest	true	"Credentials"
//	@Success	200		{object}	nameResponse
//	@Failure	400		{object}	services.ErrorResponse
//	@Failure	401		{object}	services.ErrorResponse
//	@Failure	429		{object}	services.ErrorResponse
//	@Failure	500		{object}	services.ErrorResponse
//	@Router		/api/users/login [post]
func (h *UserHandler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !services.Decode(w, r, &req) {
		return
	}
	token, name, err := h.users.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	setSessionCookie(w, token)
	services.WriteData(w, http.StatusOK, nameData{Name: name})
}

type messageData struct {
	Message string `json:"message"`
}

type messageResponse struct {
	Data   messageData         `json:"data"`
	Errors []services.APIError `json:"errors"`
}

// logout godoc
//
//	@Summary	Log out
//	@Tags		users
//	@Produce	json
//	@Success	200	{object}	messageResponse
//	@Failure	401	{object}	services.ErrorResponse
//	@Failure	500	{object}	services.ErrorResponse
//	@Router		/api/users/logout [post]
func (h *UserHandler) logout(w http.ResponseWriter, r *http.Request) {
	sess, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		writeError(w, ErrorSessionNotFound)
		return
	}
	if err := h.users.Logout(r.Context(), sess.ID); err != nil {
		writeError(w, err)
		return
	}
	clearSessionCookie(w)
	services.WriteData(w, http.StatusOK, messageData{Message: "user is successfully logged out"})
}

type profileData struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type profileResponse struct {
	Data   profileData         `json:"data"`
	Errors []services.APIError `json:"errors"`
}

// profile godoc
//
//	@Summary	Get profile
//	@Tags		users
//	@Produce	json
//	@Success	200	{object}	profileResponse
//	@Failure	401	{object}	services.ErrorResponse
//	@Failure	500	{object}	services.ErrorResponse
//	@Router		/api/users/profile [get]
func (h *UserHandler) profile(w http.ResponseWriter, r *http.Request) {
	sess, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		writeError(w, ErrorSessionNotFound)
		return
	}
	p, err := h.users.GetProfile(r.Context(), sess.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	services.WriteData(w, http.StatusOK, profileData{Name: p.Name, Email: p.Email})
}

func sessionCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name: "session", Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	}
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, sessionCookie(token, int(sessionTTL.Seconds())))
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, sessionCookie("", -1))
}

func writeError(w http.ResponseWriter, err error) {
	var ve services.ValidationError
	switch {
	case errors.As(err, &ve):
		services.WriteError(w, services.APIError{Type: services.TypeValidation, Message: ve.Message, Field: ve.Field})
	case errors.Is(err, ErrorEmailExists):
		services.WriteError(w, services.APIError{Type: services.TypeConflict, Message: err.Error()})
	case errors.Is(err, ErrorInvalidCredentials), errors.Is(err, ErrorSessionNotFound):
		services.WriteError(w, services.APIError{Type: services.TypeAuthentication, Message: err.Error()})
	case errors.Is(err, ErrorLoginBlocked):
		services.WriteError(w, services.APIError{Type: services.TypeRateLimited, Message: err.Error()})
	default:
		log.Printf("users: %v", err)
		services.WriteError(w, services.APIError{Type: services.TypeInternal, Message: "internal server error"})
	}
}
