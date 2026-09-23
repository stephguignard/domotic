// Package tahoma implémente le client de l'API locale des passerelles Somfy
// TaHoma, exposée en mode développeur sur le réseau local.
package tahoma

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/stephguignard/domotic/internal/config"
	"github.com/stephguignard/domotic/internal/store"
)

// La box présente un certificat signé par la CA Overkiz, absente des magasins
// système. L'embarquer permet de valider la connexion sans jamais recourir à
// InsecureSkipVerify.
//
//go:embed overkiz-root-ca-2048.crt
var overkizRootCA []byte

// Client interroge l'API locale d'une passerelle TaHoma.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
	log     *slog.Logger
}

// NewClient construit un client TaHoma pour la box configurée.
func NewClient(cfg *config.Config, log *slog.Logger) (*Client, error) {
	tc := cfg.Tahoma

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(overkizRootCA) {
		return nil, fmt.Errorf("CA Overkiz illisible")
	}

	// Le certificat de la box est émis pour gateway-<pin>.local, alors qu'on
	// se connecte par adresse IP : mDNS est peu fiable depuis un conteneur
	// Docker sur Synology. On force donc la destination réseau tout en
	// validant le nom attendu dans le certificat.
	serverName := fmt.Sprintf("gateway-%s.local", tc.PIN)
	target := net.JoinHostPort(tc.Host, fmt.Sprint(tc.Port))

	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			// L'adresse demandée est ignorée au profit de l'IP configurée.
			return dialer.DialContext(ctx, network, target)
		},
		TLSClientConfig: &tls.Config{
			RootCAs:    pool,
			ServerName: serverName,
			MinVersion: tls.VersionTLS12,
		},
		MaxIdleConns:        4,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	return &Client{
		baseURL: fmt.Sprintf("https://%s/enduser-mobile-web/1/enduserAPI", serverName),
		token:   tc.Token,
		http:    &http.Client{Transport: transport, Timeout: 30 * time.Second},
		log:     log,
	}, nil
}

// do exécute une requête authentifiée et décode la réponse JSON dans out.
// Passer out à nil ignore le corps de la réponse.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("tahoma %s: encodage de la requête: %w", path, err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("tahoma %s: %w", path, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("tahoma %s: %w", path, err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("tahoma %s: lecture de la réponse: %w", path, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPError{Status: resp.StatusCode, Path: path, Body: truncate(payload, 200)}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("tahoma %s: décodage: %w", path, err)
	}
	return nil
}

// HTTPError décrit une réponse d'erreur de la box.
type HTTPError struct {
	Status int
	Path   string
	Body   string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("tahoma %s: statut %d: %s", e.Path, e.Status, e.Body)
}

// FetchDevices récupère l'inventaire complet et le convertit au format unifié.
func (c *Client) FetchDevices(ctx context.Context) ([]store.Device, error) {
	var setup Setup
	if err := c.do(ctx, http.MethodGet, "/setup", nil, &setup); err != nil {
		return nil, err
	}

	// Les équipements ne portent que l'OID de leur pièce : la table de
	// correspondance vient de l'arborescence des lieux.
	rooms := map[string]string{}
	setup.RootPlace.flatten(rooms)

	now := time.Now().UTC()
	devices := make([]store.Device, 0, len(setup.Devices))

	for _, d := range setup.Devices {
		state, err := json.Marshal(statesToMap(d.States))
		if err != nil {
			return nil, fmt.Errorf("sérialisation de l'état de %s: %w", d.DeviceURL, err)
		}

		devices = append(devices, store.Device{
			ID:        d.DeviceURL,
			Source:    "tahoma",
			Name:      d.Label,
			Kind:      kindForControllable(d.ControllableName),
			Room:      rooms[d.PlaceOID],
			State:     string(state),
			Reachable: d.Available && d.Enabled,
			UpdatedAt: now,
		})
	}

	return devices, nil
}

// Commands retourne les commandes acceptées par un équipement, ce qui permet
// au frontend de n'afficher que des actions réellement disponibles.
func (c *Client) Commands(ctx context.Context, deviceURL string) ([]string, error) {
	var setup Setup
	if err := c.do(ctx, http.MethodGet, "/setup", nil, &setup); err != nil {
		return nil, err
	}

	for _, d := range setup.Devices {
		if d.DeviceURL != deviceURL {
			continue
		}
		names := make([]string, 0, len(d.Definition.Commands))
		for _, cmd := range d.Definition.Commands {
			names = append(names, cmd.CommandName)
		}
		return names, nil
	}
	return nil, fmt.Errorf("équipement %s introuvable", deviceURL)
}

// Execute envoie une commande à un équipement et retourne l'identifiant
// d'exécution attribué par la box.
func (c *Client) Execute(ctx context.Context, deviceURL, command string, params []any) (string, error) {
	if params == nil {
		params = []any{}
	}

	req := applyRequest{
		Label: "domotic",
		Actions: []action{{
			DeviceURL: deviceURL,
			Commands:  []Command{{Name: command, Parameters: params}},
		}},
	}

	var resp execResponse
	if err := c.do(ctx, http.MethodPost, "/exec/apply", req, &resp); err != nil {
		return "", err
	}

	c.log.Info("commande TaHoma envoyée", "device", deviceURL, "command", command, "exec_id", resp.ExecID)
	return resp.ExecID, nil
}

// Ping vérifie que la box répond et que le jeton est accepté.
func (c *Client) Ping(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/setup/gateways", nil, nil)
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
