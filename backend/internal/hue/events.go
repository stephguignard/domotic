package hue

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// idleTimeout borne le silence toléré sur le flux. Le pont n'envoie aucun
// signe de vie : une connexion coupée sans fermeture propre (pont redémarré,
// NAT expiré) resterait sinon ouverte indéfiniment, et les changements d'état
// seraient perdus sans que rien ne le signale. Reconnecter après ce délai ne
// coûte qu'une relecture de l'inventaire.
var idleTimeout = 5 * time.Minute // variable pour les tests

// ErrIdle signale une reconnexion après un silence prolongé. Ce n'est pas une
// panne : une maison calme la nuit ne produit aucun événement.
var ErrIdle = errors.New("flux Hue inactif, reconnexion")

// Event est un message du flux d'événements.
type Event struct {
	Type string          `json:"type"` // add, update, delete, error
	Data []EventResource `json:"data"`
}

// EventResource est une ressource modifiée, réduite aux champs utiles ici.
type EventResource struct {
	light
}

// LightState retourne les grandeurs portées par l'événement, au format de
// l'état unifié : seules celles qui ont changé sont présentes.
func (r EventResource) LightState() map[string]any {
	return r.state()
}

// Stream lit le flux d'événements jusqu'à une erreur ou l'annulation du
// contexte, en passant chaque lot d'événements à handle. Il retourne nil si le
// contexte de l'appelant est terminé, ErrIdle après un silence prolongé, une
// autre erreur sinon.
func (c *Client) Stream(ctx context.Context, handle func([]Event)) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	// Armé avant la requête : le client de flux n'a pas de délai global, un
	// pont qui accepterait la connexion sans répondre bloquerait sinon ici.
	watchdog := time.AfterFunc(idleTimeout, func() { cancel(ErrIdle) })
	defer watchdog.Stop()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/eventstream/clip/v2", nil)
	if err != nil {
		return fmt.Errorf("flux Hue: %w", err)
	}
	req.Header.Set("hue-application-key", c.appKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.stream.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			if errors.Is(context.Cause(ctx), ErrIdle) {
				return ErrIdle
			}
			return nil
		}
		return fmt.Errorf("flux Hue: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("flux Hue: statut %d", resp.StatusCode)
	}

	err = readEvents(resp.Body, func(events []Event) {
		watchdog.Reset(idleTimeout)
		handle(events)
	}, func() { watchdog.Reset(idleTimeout) })

	if ctx.Err() != nil {
		// Contexte terminé : par le watchdog, ou par l'appelant.
		if errors.Is(context.Cause(ctx), ErrIdle) {
			return ErrIdle
		}
		return nil
	}
	if err == nil {
		err = io.EOF
	}
	return fmt.Errorf("flux Hue interrompu: %w", err)
}

// readEvents découpe un flux SSE. Chaque message porte un tableau d'événements
// dans une ou plusieurs lignes « data: » ; une ligne vide le termine. alive est
// appelée à chaque ligne reçue, commentaires compris.
func readEvents(r io.Reader, handle func([]Event), alive func()) error {
	sc := bufio.NewScanner(r)
	// Un ajout d'appareil peut produire un message de plusieurs dizaines de Ko.
	sc.Buffer(make([]byte, 64<<10), 1<<20)

	var data strings.Builder
	for sc.Scan() {
		alive()
		line := sc.Text()

		switch {
		case line == "":
			if data.Len() == 0 {
				continue
			}
			var events []Event
			if err := json.Unmarshal([]byte(data.String()), &events); err == nil {
				handle(events)
			}
			// Un message illisible est ignoré : perdre un événement vaut mieux
			// que couper le flux, et l'inventaire périodique rattrape l'écart.
			data.Reset()
		case strings.HasPrefix(line, "data:"):
			data.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
		// « id: » et les commentaires (« : hi ») n'apportent rien ici.
	}
	return sc.Err()
}
