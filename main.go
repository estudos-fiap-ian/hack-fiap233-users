package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/hack-fiap233/users/internal/handler"
	"github.com/hack-fiap233/users/internal/middleware"
	"github.com/hack-fiap233/users/internal/repository"
	"github.com/hack-fiap233/users/internal/service"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	db := initDB()
	migrateDB(db)

	repo := repository.New().WithDB(db).Build()
	svc := service.New().WithRepository(repo).WithJWTSecret(jwtSecret).Build()
	h := handler.New().WithService(svc).Build()

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/users/health", middleware.Metrics("/users/health", h.Health))
	http.HandleFunc("/users/register", middleware.Metrics("/users/register", h.Register))
	http.HandleFunc("/users/login", middleware.Metrics("/users/login", h.Login))
	http.HandleFunc("/users/", middleware.Metrics("/users/", h.List))

	log.Printf("Users service listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func initDB() *sql.DB {
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=require",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USERNAME"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	log.Println("Connected to PostgreSQL")
	return db
}

func migrateDB(db *sql.DB) {
	migrations := []string{
		// Cria a tabela com o schema completo (se não existir)
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL DEFAULT ''
		)`,
		// Adiciona password_hash caso a tabela já existia sem essa coluna
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT NOT NULL DEFAULT ''`,
		// Garante unique constraint no email caso a tabela existia sem ela
		`CREATE UNIQUE INDEX IF NOT EXISTS users_email_unique ON users(email)`,
	}
	for _, q := range migrations {
		if _, err := db.Exec(q); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
	}
}
