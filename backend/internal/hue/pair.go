package hue

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// pairRetry espace les tentatives d'appairage ; variable pour les tests.
var pairRetry = 2 * time.Second

// errLinkButton signale que le bouton du pont n'a pas été pressé.
var errLinkButton = errors.New("bouton du pont non pressé")

// Pair obtient une clé d'application. Le pont ne la délivre que dans les
// 30 secondes qui suivent un appui sur son bouton : Pair réessaie donc jusqu'à
// l'échéance du contexte, en appelant waiting à chaque tentative refusée.
//
// L'appairage passe encore par l'API v1 (POST /api) : la v2 n'a pas
// d'équivalent. La clé obtenue sert ensuite à l'API v2.
func (c *Client) Pair(ctx context.Context, deviceType string, waiting func()) (string, error) {
	for {
		key, err := c.pairOnce(ctx, deviceType)
		if !errors.Is(err, errLinkButton) {
			return key, err
		}
		if waiting != nil {
			waiting()
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("appairage Hue: %w", errLinkButton)
		case <-time.After(pairRetry):
		}
	}
}

func (c *Client) pairOnce(ctx context.Context, deviceType string) (string, error) {
	body, err := json.Marshal(map[string]string{"devicetype": deviceType})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("appairage Hue: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("appairage Hue: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", fmt.Errorf("appairage Hue: lecture de la réponse: %w", err)
	}

	var results []struct {
		Success *struct {
			Username string `json:"username"`
		} `json:"success"`
		Error *struct {
			Type        int    `json:"type"`
			Description string `json:"description"`
		} `json:"error"`
	}
	if err := json.Unmarshal(payload, &results); err != nil || len(results) == 0 {
		return "", fmt.Errorf("appairage Hue: réponse inattendue (statut %d): %s", resp.StatusCode, payload)
	}

	r := results[0]
	switch {
	case r.Success != nil && r.Success.Username != "":
		return r.Success.Username, nil
	case r.Error != nil && r.Error.Type == 101: // link button not pressed
		return "", errLinkButton
	case r.Error != nil:
		return "", fmt.Errorf("appairage Hue: erreur %d: %s", r.Error.Type, r.Error.Description)
	default:
		return "", fmt.Errorf("appairage Hue: réponse inattendue: %s", payload)
	}
}
