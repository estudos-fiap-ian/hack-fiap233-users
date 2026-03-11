package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hack-fiap233/users/internal/service"
)

// mockService is a manual mock for service.UserService.
type mockService struct {
	registerFn  func(ctx context.Context, name, email, password string) (*service.AuthOutput, error)
	loginFn     func(ctx context.Context, email, password string) (*service.AuthOutput, error)
	listUsersFn func(ctx context.Context) ([]*service.User, error)
	healthFn    func(ctx context.Context) error
}

func (m *mockService) Register(ctx context.Context, name, email, password string) (*service.AuthOutput, error) {
	return m.registerFn(ctx, name, email, password)
}

func (m *mockService) Login(ctx context.Context, email, password string) (*service.AuthOutput, error) {
	return m.loginFn(ctx, email, password)
}

func (m *mockService) ListUsers(ctx context.Context) ([]*service.User, error) {
	return m.listUsersFn(ctx)
}

func (m *mockService) Health(ctx context.Context) error {
	return m.healthFn(ctx)
}

func newTestHandler(svc service.UserService) *UserHandler {
	return New().WithService(svc).Build()
}

func jsonBody(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	b, _ := json.Marshal(v)
	return bytes.NewReader(b)
}

// --- Health ---

func TestHealth_Healthy(t *testing.T) {
	h := newTestHandler(&mockService{
		healthFn: func(_ context.Context) error { return nil },
	})

	req := httptest.NewRequest(http.MethodGet, "/users/health", nil)
	rec := httptest.NewRecorder()
	h.Health(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", body["status"])
	}
}

func TestHealth_Unhealthy(t *testing.T) {
	h := newTestHandler(&mockService{
		healthFn: func(_ context.Context) error { return errors.New("db down") },
	})

	req := httptest.NewRequest(http.MethodGet, "/users/health", nil)
	rec := httptest.NewRecorder()
	h.Health(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "unhealthy" {
		t.Errorf("expected status=unhealthy, got %q", body["status"])
	}
}

// --- Register ---

func TestRegister_Success(t *testing.T) {
	h := newTestHandler(&mockService{
		registerFn: func(_ context.Context, name, email, _ string) (*service.AuthOutput, error) {
			return &service.AuthOutput{Token: "tok", User: service.User{ID: 1, Name: name, Email: email}}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/users/register",
		jsonBody(t, map[string]string{"name": "Alice", "email": "alice@example.com", "password": "pass"}))
	rec := httptest.NewRecorder()
	h.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}
	var out service.AuthOutput
	json.NewDecoder(rec.Body).Decode(&out)
	if out.Token != "tok" {
		t.Errorf("expected token=tok, got %q", out.Token)
	}
}

func TestRegister_InvalidJSON(t *testing.T) {
	h := newTestHandler(&mockService{})

	req := httptest.NewRequest(http.MethodPost, "/users/register", bytes.NewBufferString("not-json"))
	rec := httptest.NewRecorder()
	h.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestRegister_MissingFields(t *testing.T) {
	h := newTestHandler(&mockService{})

	cases := []map[string]string{
		{"email": "a@b.com", "password": "pass"}, // missing name
		{"name": "Alice", "password": "pass"},     // missing email
		{"name": "Alice", "email": "a@b.com"},     // missing password
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodPost, "/users/register", jsonBody(t, c))
		rec := httptest.NewRecorder()
		h.Register(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for body %v, got %d", c, rec.Code)
		}
	}
}

func TestRegister_EmailTaken(t *testing.T) {
	h := newTestHandler(&mockService{
		registerFn: func(_ context.Context, _, _, _ string) (*service.AuthOutput, error) {
			return nil, service.ErrEmailTaken
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/users/register",
		jsonBody(t, map[string]string{"name": "Alice", "email": "alice@example.com", "password": "pass"}))
	rec := httptest.NewRecorder()
	h.Register(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestRegister_InternalError(t *testing.T) {
	h := newTestHandler(&mockService{
		registerFn: func(_ context.Context, _, _, _ string) (*service.AuthOutput, error) {
			return nil, errors.New("unexpected")
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/users/register",
		jsonBody(t, map[string]string{"name": "Alice", "email": "a@b.com", "password": "pass"}))
	rec := httptest.NewRecorder()
	h.Register(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestRegister_MethodNotAllowed(t *testing.T) {
	h := newTestHandler(&mockService{})

	req := httptest.NewRequest(http.MethodGet, "/users/register", nil)
	rec := httptest.NewRecorder()
	h.Register(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

// --- Login ---

func TestLogin_Success(t *testing.T) {
	h := newTestHandler(&mockService{
		loginFn: func(_ context.Context, email, _ string) (*service.AuthOutput, error) {
			return &service.AuthOutput{Token: "tok", User: service.User{ID: 1, Email: email}}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/users/login",
		jsonBody(t, map[string]string{"email": "bob@example.com", "password": "pass"}))
	rec := httptest.NewRecorder()
	h.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestLogin_InvalidJSON(t *testing.T) {
	h := newTestHandler(&mockService{})

	req := httptest.NewRequest(http.MethodPost, "/users/login", bytes.NewBufferString("{bad"))
	rec := httptest.NewRecorder()
	h.Login(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestLogin_MissingFields(t *testing.T) {
	h := newTestHandler(&mockService{})

	cases := []map[string]string{
		{"password": "pass"},    // missing email
		{"email": "a@b.com"},   // missing password
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodPost, "/users/login", jsonBody(t, c))
		rec := httptest.NewRecorder()
		h.Login(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for body %v, got %d", c, rec.Code)
		}
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	h := newTestHandler(&mockService{
		loginFn: func(_ context.Context, _, _ string) (*service.AuthOutput, error) {
			return nil, service.ErrInvalidCredentials
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/users/login",
		jsonBody(t, map[string]string{"email": "x@x.com", "password": "wrong"}))
	rec := httptest.NewRecorder()
	h.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestLogin_InternalError(t *testing.T) {
	h := newTestHandler(&mockService{
		loginFn: func(_ context.Context, _, _ string) (*service.AuthOutput, error) {
			return nil, errors.New("unexpected")
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/users/login",
		jsonBody(t, map[string]string{"email": "x@x.com", "password": "pass"}))
	rec := httptest.NewRecorder()
	h.Login(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestLogin_MethodNotAllowed(t *testing.T) {
	h := newTestHandler(&mockService{})

	req := httptest.NewRequest(http.MethodGet, "/users/login", nil)
	rec := httptest.NewRecorder()
	h.Login(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

// --- List ---

func TestList_Success(t *testing.T) {
	h := newTestHandler(&mockService{
		listUsersFn: func(_ context.Context) ([]*service.User, error) {
			return []*service.User{{ID: 1, Name: "Alice", Email: "alice@example.com"}}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/users/", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestList_InternalError(t *testing.T) {
	h := newTestHandler(&mockService{
		listUsersFn: func(_ context.Context) ([]*service.User, error) {
			return nil, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/users/", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestList_MethodNotAllowed(t *testing.T) {
	h := newTestHandler(&mockService{})

	req := httptest.NewRequest(http.MethodPost, "/users/", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}
