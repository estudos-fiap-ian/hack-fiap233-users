package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// mockRepo is a manual mock for UserRepository.
type mockRepo struct {
	createFn      func(ctx context.Context, name, email, passwordHash string) (*User, error)
	findByEmailFn func(ctx context.Context, email string) (*User, string, error)
	listFn        func(ctx context.Context) ([]*User, error)
	pingFn        func(ctx context.Context) error
}

func (m *mockRepo) Create(ctx context.Context, name, email, passwordHash string) (*User, error) {
	return m.createFn(ctx, name, email, passwordHash)
}

func (m *mockRepo) FindByEmail(ctx context.Context, email string) (*User, string, error) {
	return m.findByEmailFn(ctx, email)
}

func (m *mockRepo) List(ctx context.Context) ([]*User, error) {
	return m.listFn(ctx)
}

func (m *mockRepo) Ping(ctx context.Context) error {
	return m.pingFn(ctx)
}

func newTestService(repo UserRepository) UserService {
	return New().WithRepository(repo).WithJWTSecret("test-secret").Build()
}

// --- Register ---

func TestRegister_Success(t *testing.T) {
	repo := &mockRepo{
		createFn: func(_ context.Context, name, email, _ string) (*User, error) {
			return &User{ID: 1, Name: name, Email: email}, nil
		},
	}
	svc := newTestService(repo)

	out, err := svc.Register(context.Background(), "Alice", "alice@example.com", "password123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out.User.Email != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %s", out.User.Email)
	}
	if out.Token == "" {
		t.Error("expected non-empty JWT token")
	}
}

func TestRegister_EmailTaken(t *testing.T) {
	repo := &mockRepo{
		createFn: func(_ context.Context, _, _, _ string) (*User, error) {
			return nil, ErrDuplicateEmail
		},
	}
	svc := newTestService(repo)

	_, err := svc.Register(context.Background(), "Alice", "alice@example.com", "password123")
	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("expected ErrEmailTaken, got %v", err)
	}
}

func TestRegister_RepoError(t *testing.T) {
	repo := &mockRepo{
		createFn: func(_ context.Context, _, _, _ string) (*User, error) {
			return nil, errors.New("db error")
		},
	}
	svc := newTestService(repo)

	_, err := svc.Register(context.Background(), "Alice", "alice@example.com", "password123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, ErrEmailTaken) {
		t.Error("should not be ErrEmailTaken for a generic repo error")
	}
}

// --- Login ---

func TestLogin_Success(t *testing.T) {
	password := "secret123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)

	repo := &mockRepo{
		findByEmailFn: func(_ context.Context, email string) (*User, string, error) {
			return &User{ID: 2, Name: "Bob", Email: email}, string(hash), nil
		},
	}
	svc := newTestService(repo)

	out, err := svc.Login(context.Background(), "bob@example.com", password)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out.User.Email != "bob@example.com" {
		t.Errorf("expected bob@example.com, got %s", out.User.Email)
	}
	if out.Token == "" {
		t.Error("expected non-empty JWT token")
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	repo := &mockRepo{
		findByEmailFn: func(_ context.Context, _ string) (*User, string, error) {
			return nil, "", sql.ErrNoRows
		},
	}
	svc := newTestService(repo)

	_, err := svc.Login(context.Background(), "nobody@example.com", "pass")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct"), bcrypt.MinCost)

	repo := &mockRepo{
		findByEmailFn: func(_ context.Context, email string) (*User, string, error) {
			return &User{ID: 3, Name: "Carol", Email: email}, string(hash), nil
		},
	}
	svc := newTestService(repo)

	_, err := svc.Login(context.Background(), "carol@example.com", "wrong")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_RepoError(t *testing.T) {
	repo := &mockRepo{
		findByEmailFn: func(_ context.Context, _ string) (*User, string, error) {
			return nil, "", errors.New("connection refused")
		},
	}
	svc := newTestService(repo)

	_, err := svc.Login(context.Background(), "x@example.com", "pass")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, ErrInvalidCredentials) {
		t.Error("generic repo error should not be wrapped as ErrInvalidCredentials")
	}
}

// --- ListUsers ---

func TestListUsers_Success(t *testing.T) {
	want := []*User{
		{ID: 1, Name: "Alice", Email: "alice@example.com"},
		{ID: 2, Name: "Bob", Email: "bob@example.com"},
	}
	repo := &mockRepo{
		listFn: func(_ context.Context) ([]*User, error) { return want, nil },
	}
	svc := newTestService(repo)

	got, err := svc.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 users, got %d", len(got))
	}
}

func TestListUsers_Empty(t *testing.T) {
	repo := &mockRepo{
		listFn: func(_ context.Context) ([]*User, error) { return nil, nil },
	}
	svc := newTestService(repo)

	got, err := svc.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 users, got %d", len(got))
	}
}

func TestListUsers_Error(t *testing.T) {
	repo := &mockRepo{
		listFn: func(_ context.Context) ([]*User, error) { return nil, errors.New("db down") },
	}
	svc := newTestService(repo)

	_, err := svc.ListUsers(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- Health ---

func TestHealth_OK(t *testing.T) {
	repo := &mockRepo{
		pingFn: func(_ context.Context) error { return nil },
	}
	svc := newTestService(repo)

	if err := svc.Health(context.Background()); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestHealth_Error(t *testing.T) {
	repo := &mockRepo{
		pingFn: func(_ context.Context) error { return errors.New("db unreachable") },
	}
	svc := newTestService(repo)

	if err := svc.Health(context.Background()); err == nil {
		t.Error("expected error, got nil")
	}
}
