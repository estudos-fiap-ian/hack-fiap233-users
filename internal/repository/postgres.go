package repository

import (
	"context"
	"database/sql"

	"github.com/hack-fiap233/users/internal/service"
	"github.com/lib/pq"
)

type postgresRepository struct {
	db *sql.DB
}

type UserRepositoryBuilder struct {
	repo *postgresRepository
}

func New() *UserRepositoryBuilder {
	return &UserRepositoryBuilder{repo: &postgresRepository{}}
}

func (b *UserRepositoryBuilder) WithDB(db *sql.DB) *UserRepositoryBuilder {
	b.repo.db = db
	return b
}

func (b *UserRepositoryBuilder) Build() service.UserRepository {
	return b.repo
}

func (r *postgresRepository) Create(ctx context.Context, name, email, passwordHash string) (*service.User, error) {
	var u service.User
	err := r.db.QueryRowContext(ctx,
		"INSERT INTO users (name, email, password_hash) VALUES ($1, $2, $3) RETURNING id, name, email",
		name, email, passwordHash,
	).Scan(&u.ID, &u.Name, &u.Email)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return nil, service.ErrDuplicateEmail
		}
		return nil, err
	}
	return &u, nil
}

func (r *postgresRepository) FindByEmail(ctx context.Context, email string) (*service.User, string, error) {
	var u service.User
	var hash string
	err := r.db.QueryRowContext(ctx,
		"SELECT id, name, email, password_hash FROM users WHERE email = $1",
		email,
	).Scan(&u.ID, &u.Name, &u.Email, &hash)
	if err != nil {
		return nil, "", err
	}
	return &u, hash, nil
}

func (r *postgresRepository) List(ctx context.Context) ([]*service.User, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, email FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*service.User
	for rows.Next() {
		var u service.User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email); err != nil {
			return nil, err
		}
		users = append(users, &u)
	}
	return users, rows.Err()
}

func (r *postgresRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}
