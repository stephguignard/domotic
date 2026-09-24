// Package hue implémente le client de l'API locale v2 (CLIP v2) d'un pont
// Philips Hue.
package hue

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/stephguignard/domotic/internal/command"
	"github.com/stephguignard/domotic/internal/config"
	"github.com/stephguignard/domotic/internal/store"
)

// Client interroge un pont Hue.
type Client struct {
	baseURL string
	appKey  string
	http    *http.Client
	// stream sert au flux d'événements : même transport, mais sans délai
	// global, la connexion étant faite pour durer.
	stream *http.Client
	log    *slog.Logger
}

// NewClient construit un client pour le pont configuré. La clé d'application
// peut être vide : seul l'appairage (Pair) fonctionne alors.
func NewClient(cfg config.HueConfig, log *slog.Logger) (*Client, error) {
	roots, err := signifyRoots()
	if err != nil {
		return nil, err
	}
	return newClient(cfg, roots, log)
}

// newClient permet aux tests de substituer leur propre CA.
func newClient(cfg config.HueConfig, roots *x509.CertPool, log *slog.Logger) (*Client, error) {
	tlsCfg, err := tlsConfig(cfg.BridgeID, roots)
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{
		DialContext:         (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSClientConfig:     tlsCfg,
		MaxIdleConns:        4,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	return &Client{
		baseURL: "https://" + cfg.Host,
		appKey:  cfg.AppKey,
		http:    &http.Client{Transport: transport, Timeout: 15 * time.Second},
		stream:  &http.Client{Transport: transport},
		log:     log,
	}, nil
}

// FetchDevices récupère les lumières et les convertit au format unifié, avec
// leur pièce et leur joignabilité.
func (c *Client) FetchDevices(ctx context.Context) ([]store.Device, error) {
	var (
		lights []light
		rooms  []room
		links  []zigbeeConnectivity
	)
	if err := c.get(ctx, "light", &lights); err != nil {
		return nil, err
	}
	if err := c.get(ctx, "room", &rooms); err != nil {
		return nil, err
	}
	if err := c.get(ctx, "zigbee_connectivity", &links); err != nil {
		return nil, err
	}

	// Une lumière appartient à un appareil physique (owner) ; ce sont les
	// appareils que les pièces regroupent, et dont la liaison Zigbee est suivie.
	roomOf := map[string]string{}
	for _, r := range rooms {
		for _, child := range r.Children {
			if child.RType == "device" {
				roomOf[child.RID] = r.Metadata.Name
			}
		}
	}
	connected := map[string]bool{}
	for _, z := range links {
		connected[z.Owner.RID] = z.Status == "connected"
	}

	now := time.Now().UTC()
	devices := make([]store.Device, 0, len(lights))
	for _, l := range lights {
		state, err := json.Marshal(l.state())
		if err != nil {
			return nil, fmt.Errorf("sérialisation de l'état de %s: %w", l.ID, err)
		}

		// Sans suivi Zigbee (lumière Wi-Fi ou Matter), rien ne permet de la
		// dire injoignable.
		reachable, tracked := connected[l.Owner.RID]
		if !tracked {
			reachable = true
		}

		devices = append(devices, store.Device{
			ID:        l.ID,
			Source:    "hue",
			Name:      l.Metadata.Name,
			Kind:      "light",
			Room:      roomOf[l.Owner.RID],
			State:     string(state),
			Reachable: reachable,
			UpdatedAt: now,
		})
	}
	slices.SortFunc(devices, func(a, b store.Device) int { return strings.Compare(a.ID, b.ID) })
	return devices, nil
}

// Execute pilote une lumière : on, off, ou setBrightness avec un pourcentage.
func (c *Client) Execute(ctx context.Context, id, cmd string, params []any) (string, error) {
	var body map[string]any
	switch cmd {
	case "on", "off":
		body = map[string]any{"on": map[string]bool{"on": cmd == "on"}}
	case "setBrightness":
		pct, err := brightnessParam(params)
		if err != nil {
			return "", err
		}
		if pct == 0 {
			// Le pont refuse une luminosité nulle : 0 % signifie éteindre.
			body = map[string]any{"on": map[string]bool{"on": false}}
		} else {
			body = map[string]any{
				"on":      map[string]bool{"on": true},
				"dimming": map[string]float64{"brightness": pct},
			}
		}
	default:
		return "", fmt.Errorf("%w par une lumière Hue: %s", command.ErrUnsupported, cmd)
	}

	if err := c.do(ctx, http.MethodPut, "/clip/v2/resource/light/"+id, body, nil); err != nil {
		return "", err
	}
	c.log.Info("commande Hue envoyée", "device", id, "command", cmd)
	return "", nil
}

func brightnessParam(params []any) (float64, error) {
	if len(params) != 1 {
		return 0, fmt.Errorf("%w: setBrightness attend un pourcentage", command.ErrUnsupported)
	}
	pct, ok := params[0].(float64) // un nombre JSON décodé
	if !ok || pct < 0 || pct > 100 {
		return 0, fmt.Errorf("%w: luminosité invalide %v, attendu 0 à 100", command.ErrUnsupported, params[0])
	}
	return pct, nil
}

// get lit toutes les ressources d'un type.
func (c *Client) get(ctx context.Context, resource string, out any) error {
	return c.do(ctx, http.MethodGet, "/clip/v2/resource/"+resource, nil, out)
}

// do exécute une requête CLIP v2 et décode le champ data de la réponse.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("hue %s: encodage de la requête: %w", path, err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("hue %s: %w", path, err)
	}
	req.Header.Set("hue-application-key", c.appKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("hue %s: %w", path, err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("hue %s: lecture de la réponse: %w", path, err)
	}

	var env envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return fmt.Errorf("hue %s: statut %d, réponse illisible: %w", path, resp.StatusCode, err)
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("hue %s: clé d'application refusée, relancer `domotic hue-pair`", path)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || len(env.Errors) > 0 {
		return fmt.Errorf("hue %s: statut %d: %s", path, resp.StatusCode, env.describeErrors())
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("hue %s: décodage: %w", path, err)
	}
	return nil
}
