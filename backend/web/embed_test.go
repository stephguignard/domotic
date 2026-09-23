package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsFingerprinted(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		// Noms réellement produits par le build Angular : l'empreinte est en
		// base36 et mêle chiffres et lettres des deux casses.
		{"main-PZFTEJS6.js", true},
		{"chunk-Dl5xfOCj.js", true},
		{"styles-GKUBA3AH.css", true},
		{"chunk-B8yzRf9I.js", true},

		// index.html doit rester revalidé à chaque chargement, sous peine de
		// servir indéfiniment une version périmée après un déploiement.
		{"index.html", false},
		{"favicon.ico", false},
		{"main.js", false},           // pas d'empreinte
		{"main-abc.js", false},       // empreinte trop courte
		{"main-PZFT.EJS6.js", false}, // le point n'appartient pas à l'alphabet
	}

	for _, c := range cases {
		if got := isFingerprinted(c.name); got != c.want {
			t.Errorf("isFingerprinted(%q) = %v, attendu %v", c.name, got, c.want)
		}
	}
}

func TestHandlerWorksWithoutBuiltFrontend(t *testing.T) {
	// Sur un dépôt fraîchement cloné, web/dist ne contient que .gitkeep : le
	// backend doit rester démarrable et servir une page explicative plutôt que
	// d'échouer.
	h, err := Handler()
	if err != nil {
		t.Fatalf("Handler() a échoué: %v", err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("statut = %d, attendu 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, attendu du HTML", ct)
	}
}

func TestHandlerServesSPAFallback(t *testing.T) {
	h, err := Handler()
	if err != nil {
		t.Fatalf("Handler() a échoué: %v", err)
	}

	// Une route Angular rechargée directement doit renvoyer l'application,
	// jamais une 404 : le routage est résolu côté navigateur.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/devices/io%3A%2F%2F1234%2F1", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("statut = %d, attendu 200", rec.Code)
	}
}
