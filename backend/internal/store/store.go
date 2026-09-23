// Package store encapsule la persistance SQLite du service.
//
// Le driver est modernc.org/sqlite, une transpilation de SQLite en Go pur :
// c'est ce qui permet de compiler avec CGO_ENABLED=0 et de produire un binaire
// statique déployable sur une image distroless.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // driver "sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store donne accès à la base de données.
type Store struct {
	db *sql.DB
}

// Open ouvre la base au chemin donné, crée le répertoire parent si besoin et
// applique les migrations en attente.
func Open(ctx context.Context, path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("création du répertoire de la base: %w", err)
		}
	}

	// WAL permet aux boucles de polling d'écrire pendant que les requêtes HTTP
	// lisent ; busy_timeout absorbe les rares collisions d'écriture.
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)",
		url.PathEscape(path),
	)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("ouverture de la base: %w", err)
	}

	// SQLite ne gère qu'un écrivain à la fois. Sérialiser les accès côté Go
	// évite de dépendre du busy_timeout pour le cas courant.
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connexion à la base: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate(ctx context.Context) error {
	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("configuration de goose: %w", err)
	}
	if err := goose.UpContext(ctx, s.db, "migrations"); err != nil {
		return fmt.Errorf("application des migrations: %w", err)
	}
	return nil
}

// Close ferme la connexion à la base.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB expose la connexion sous-jacente, utile pour les tests.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Ping vérifie que la base répond.
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}
