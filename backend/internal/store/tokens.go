package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"golang.org/x/oauth2"
)

// SaveToken persiste le jeton OAuth2 d'un fournisseur.
//
// Cette écriture est le point critique de l'intégration Netatmo : leur API fait
// tourner le refresh token à chaque rafraîchissement et invalide l'ancien. Si
// la rotation n'est pas persistée, le service perd son accès au prochain
// redémarrage et exige une ré-authentification manuelle.
func (s *Store) SaveToken(ctx context.Context, provider string, tok *oauth2.Token) error {
	if tok == nil {
		return fmt.Errorf("jeton nil pour %s", provider)
	}
	if tok.RefreshToken == "" {
		// Sans refresh token, écrire écraserait celui déjà en base par une
		// chaîne vide et couperait définitivement l'accès.
		return fmt.Errorf("refus d'enregistrer un jeton %s sans refresh token", provider)
	}

	tokenType := tok.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO oauth_token (provider, access_token, refresh_token, token_type, expiry, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (provider) DO UPDATE SET
			access_token  = excluded.access_token,
			refresh_token = excluded.refresh_token,
			token_type    = excluded.token_type,
			expiry        = excluded.expiry,
			updated_at    = excluded.updated_at`,
		provider, tok.AccessToken, tok.RefreshToken, tokenType, tok.Expiry, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("enregistrement du jeton %s: %w", provider, err)
	}
	return nil
}

// GetToken relit le jeton d'un fournisseur. Retourne ErrNotFound si aucune
// authentification n'a encore eu lieu.
func (s *Store) GetToken(ctx context.Context, provider string) (*oauth2.Token, error) {
	var (
		tok       oauth2.Token
		tokenType string
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT access_token, refresh_token, token_type, expiry FROM oauth_token WHERE provider = ?`,
		provider,
	).Scan(&tok.AccessToken, &tok.RefreshToken, &tokenType, &tok.Expiry)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lecture du jeton %s: %w", provider, err)
	}

	tok.TokenType = tokenType
	return &tok, nil
}

// DeleteToken supprime le jeton d'un fournisseur, forçant une
// ré-authentification.
func (s *Store) DeleteToken(ctx context.Context, provider string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM oauth_token WHERE provider = ?`, provider); err != nil {
		return fmt.Errorf("suppression du jeton %s: %w", provider, err)
	}
	return nil
}
