package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("port par défaut = %d, attendu 8080", cfg.Port)
	}
	// Sans identifiants, les deux intégrations restent inertes : le service
	// démarre quand même, ce qui permet de le configurer progressivement.
	if cfg.Netatmo.Enabled() {
		t.Error("Netatmo ne devrait pas être activé sans identifiants")
	}
	if cfg.Tahoma.Enabled() {
		t.Error("TaHoma ne devrait pas être activé sans identifiants")
	}
}

func TestLoadRejectsPartialNetatmoConfig(t *testing.T) {
	// Une configuration à moitié renseignée est presque toujours une faute de
	// frappe : mieux vaut refuser de démarrer qu'ignorer l'intégration.
	t.Setenv("NETATMO_CLIENT_ID", "abc")

	if _, err := Load(); err == nil {
		t.Error("attendu une erreur pour un NETATMO_CLIENT_SECRET manquant")
	}
}

func TestLoadRejectsPartialTahomaConfig(t *testing.T) {
	t.Setenv("TAHOMA_HOST", "192.168.1.42")
	t.Setenv("TAHOMA_PIN", "1234-5678-9012")
	// TAHOMA_TOKEN manquant

	if _, err := Load(); err == nil {
		t.Error("attendu une erreur pour un TAHOMA_TOKEN manquant")
	}
}

func TestLoadAcceptsCompleteConfig(t *testing.T) {
	t.Setenv("NETATMO_CLIENT_ID", "id")
	t.Setenv("NETATMO_CLIENT_SECRET", "secret")
	t.Setenv("TAHOMA_HOST", "192.168.1.42")
	t.Setenv("TAHOMA_PIN", "1234-5678-9012")
	t.Setenv("TAHOMA_TOKEN", "token")
	t.Setenv("DOMOTIC_PUBLIC_URL", "https://domotic.example.com/")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !cfg.Netatmo.Enabled() || !cfg.Tahoma.Enabled() {
		t.Error("les deux intégrations devraient être activées")
	}
	// L'URL de callback doit correspondre au caractère près à celle déclarée
	// sur dev.netatmo.com, barre oblique finale comprise.
	if got, want := cfg.NetatmoRedirectURL(), "https://domotic.example.com/auth/netatmo/callback"; got != want {
		t.Errorf("URL de callback = %q, attendu %q", got, want)
	}
}

func TestLoadRejectsTooFrequentPolling(t *testing.T) {
	t.Setenv("NETATMO_CLIENT_ID", "id")
	t.Setenv("NETATMO_CLIENT_SECRET", "secret")
	t.Setenv("NETATMO_POLL_INTERVAL", "10s")

	if _, err := Load(); err == nil {
		t.Error("attendu une erreur pour un intervalle de polling trop court")
	}
}

func TestLoadRejectsTooFrequentEventFetch(t *testing.T) {
	t.Setenv("TAHOMA_HOST", "192.168.1.42")
	t.Setenv("TAHOMA_PIN", "1234-5678-9012")
	t.Setenv("TAHOMA_TOKEN", "token")
	// La box rejette les appels à /events/{id}/fetch plus rapprochés qu'une seconde.
	t.Setenv("TAHOMA_EVENT_INTERVAL", "100ms")

	if _, err := Load(); err == nil {
		t.Error("attendu une erreur pour un intervalle d'événements trop court")
	}
}

func TestInvalidDurationFallsBackToDefault(t *testing.T) {
	t.Setenv("NETATMO_CLIENT_ID", "id")
	t.Setenv("NETATMO_CLIENT_SECRET", "secret")
	t.Setenv("NETATMO_POLL_INTERVAL", "pas-une-durée")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Netatmo.PollInterval != 10*time.Minute {
		t.Errorf("intervalle = %s, attendu la valeur par défaut de 10m", cfg.Netatmo.PollInterval)
	}
}
