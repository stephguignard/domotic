package netatmo

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/stephguignard/domotic/internal/store"
)

// Provider est la clé sous laquelle le jeton Netatmo est stocké.
const Provider = "netatmo"

// persistingTokenSource enveloppe un oauth2.TokenSource et enregistre le jeton
// en base dès que le refresh token change.
//
// C'est indispensable avec Netatmo : chaque appel à /oauth2/token renvoie un
// nouveau refresh token et invalide le précédent. Le ReuseTokenSource fourni
// par golang.org/x/oauth2 garde la rotation en mémoire uniquement — au
// redémarrage, le jeton relu depuis la base serait déjà périmé et l'accès
// perdu jusqu'à une ré-authentification manuelle.
type persistingTokenSource struct {
	src   oauth2.TokenSource
	store *store.Store
	log   *slog.Logger

	mu   sync.Mutex
	last string // dernier refresh token connu, pour n'écrire qu'aux rotations
}

// NewTokenSource construit une source de jetons qui persiste chaque rotation.
func NewTokenSource(ctx context.Context, cfg *oauth2.Config, st *store.Store, initial *oauth2.Token, log *slog.Logger) oauth2.TokenSource {
	return &persistingTokenSource{
		src:   cfg.TokenSource(ctx, initial),
		store: st,
		log:   log,
		last:  initial.RefreshToken,
	}
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := p.src.Token()
	if err != nil {
		return nil, fmt.Errorf("rafraîchissement du jeton Netatmo: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if tok.RefreshToken == "" || tok.RefreshToken == p.last {
		return tok, nil
	}

	// Le contexte de la requête appelante peut être sur le point d'expirer ;
	// la persistance ne doit pas en dépendre.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.store.SaveToken(ctx, Provider, tok); err != nil {
		// Échouer bruyamment : continuer avec un jeton non persisté revient à
		// perdre silencieusement l'accès au prochain redémarrage.
		return nil, fmt.Errorf("persistance du jeton Netatmo après rotation: %w", err)
	}

	p.last = tok.RefreshToken
	p.log.Info("jeton Netatmo rafraîchi et persisté", "expiry", tok.Expiry)
	return tok, nil
}
