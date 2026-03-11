package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/hack-fiap233/users/internal/service"
)

type UserHandler struct {
	svc service.UserService
}

type UserHandlerBuilder struct {
	handler *UserHandler
}

func New() *UserHandlerBuilder {
	return &UserHandlerBuilder{handler: &UserHandler{}}
}

func (b *UserHandlerBuilder) WithService(svc service.UserService) *UserHandlerBuilder {
	b.handler.svc = svc
	return b
}

func (b *UserHandlerBuilder) Build() *UserHandler {
	return b.handler
}

func (h *UserHandler) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Health(r.Context()); err != nil {
		respond(w, http.StatusServiceUnavailable, map[string]string{
			"status": "unhealthy",
			"db":     err.Error(),
		})
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "ok", "service": "users", "db": "connected"})
}

func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var body struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Name == "" || body.Email == "" || body.Password == "" {
		respondError(w, http.StatusBadRequest, "name, email and password are required")
		return
	}

	out, err := h.svc.Register(r.Context(), body.Name, body.Email, body.Password)
	if err != nil {
		if errors.Is(err, service.ErrEmailTaken) {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	respond(w, http.StatusCreated, out)
}

func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Email == "" || body.Password == "" {
		respondError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	out, err := h.svc.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			respondError(w, http.StatusUnauthorized, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	respond(w, http.StatusOK, out)
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	users, err := h.svc.ListUsers(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	respond(w, http.StatusOK, users)
}

func respond(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respond(w, status, map[string]string{"error": msg})
}
