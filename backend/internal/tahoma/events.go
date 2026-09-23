package tahoma

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// EventListener maintient un abonnement au flux d'événements de la box et
// réenregistre automatiquement le listener quand celui-ci expire.
//
// La box impose un maximum d'un appel par seconde sur /events/{id}/fetch ; le
// rythme réel est fixé par l'appelant via la configuration.
type EventListener struct {
	client *Client

	mu sync.Mutex
	id string // identifiant du listener courant, vide s'il faut (ré)enregistrer
}

// NewEventListener construit un écouteur d'événements.
func NewEventListener(c *Client) *EventListener {
	return &EventListener{client: c}
}

// register demande un nouvel identifiant de listener à la box.
func (l *EventListener) register(ctx context.Context) (string, error) {
	var resp listenerResponse
	if err := l.client.do(ctx, http.MethodPost, "/events/register", nil, &resp); err != nil {
		return "", err
	}
	if resp.ID == "" {
		return "", fmt.Errorf("tahoma: identifiant de listener vide")
	}

	l.client.log.Info("listener d'événements TaHoma enregistré", "listener_id", resp.ID)
	return resp.ID, nil
}

// Fetch récupère les événements en attente. Si le listener a expiré — la box
// les recycle après quelques minutes d'inactivité, ou au redémarrage — il est
// réenregistré et l'appel est retenté une fois.
func (l *EventListener) Fetch(ctx context.Context) ([]Event, error) {
	id, err := l.ensureListener(ctx)
	if err != nil {
		return nil, err
	}

	events, err := l.fetchWith(ctx, id)
	if err == nil {
		return events, nil
	}

	// Un listener inconnu de la box se traduit par une erreur HTTP ; on le
	// recrée plutôt que de laisser la boucle échouer indéfiniment.
	var httpErr *HTTPError
	if !asHTTPError(err, &httpErr) || !isListenerExpired(httpErr) {
		return nil, err
	}

	l.client.log.Warn("listener TaHoma expiré, réenregistrement", "listener_id", id, "status", httpErr.Status)
	l.invalidate(id)

	id, err = l.ensureListener(ctx)
	if err != nil {
		return nil, err
	}
	return l.fetchWith(ctx, id)
}

func (l *EventListener) ensureListener(ctx context.Context) (string, error) {
	l.mu.Lock()
	id := l.id
	l.mu.Unlock()

	if id != "" {
		return id, nil
	}

	id, err := l.register(ctx)
	if err != nil {
		return "", err
	}

	l.mu.Lock()
	l.id = id
	l.mu.Unlock()
	return id, nil
}

// invalidate oublie le listener donné, sauf s'il a déjà été remplacé entre-temps.
func (l *EventListener) invalidate(id string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.id == id {
		l.id = ""
	}
}

func (l *EventListener) fetchWith(ctx context.Context, id string) ([]Event, error) {
	var events []Event
	if err := l.client.do(ctx, http.MethodPost, "/events/"+id+"/fetch", nil, &events); err != nil {
		return nil, err
	}
	return events, nil
}

// Unregister libère le listener auprès de la box. À appeler à l'arrêt : la box
// n'en tolère qu'un nombre limité simultanément.
func (l *EventListener) Unregister(ctx context.Context) error {
	l.mu.Lock()
	id := l.id
	l.id = ""
	l.mu.Unlock()

	if id == "" {
		return nil
	}
	return l.client.do(ctx, http.MethodPost, "/events/"+id+"/unregister", nil, nil)
}

// StatesJSON extrait les états d'un événement sous forme de JSON, prêt à être
// enregistré.
func (e Event) StatesJSON() (string, error) {
	b, err := json.Marshal(statesToMap(e.DeviceStates))
	if err != nil {
		return "", fmt.Errorf("sérialisation des états de %s: %w", e.DeviceURL, err)
	}
	return string(b), nil
}

// isListenerExpired reconnaît les réponses signalant un listener inconnu.
func isListenerExpired(e *HTTPError) bool {
	return e.Status == http.StatusBadRequest ||
		e.Status == http.StatusNotFound ||
		e.Status == http.StatusUnauthorized
}

// asHTTPError est un errors.As spécialisé, isolé pour garder Fetch lisible.
func asHTTPError(err error, target **HTTPError) bool {
	for err != nil {
		if e, ok := err.(*HTTPError); ok {
			*target = e
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
