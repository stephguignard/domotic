package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
	"time"
)

// stateCookie porte le paramètre `state` OAuth2 entre la redirection vers
// Netatmo et le retour sur le callback.
const stateCookie = "netatmo_oauth_state"

// authHandler implémente le flux OAuth2 « authorization code » de Netatmo.
//
// Ce flux n'est parcouru qu'une fois, à la configuration initiale : ensuite, le
// refresh token persisté suffit à maintenir l'accès indéfiniment.
type authHandler struct {
	deps Deps
}

// start redirige l'utilisateur vers la page de consentement Netatmo.
func (h *authHandler) start(w http.ResponseWriter, r *http.Request) {
	state, err := randomState()
	if err != nil {
		http.Error(w, "génération de l'état OAuth2 impossible", http.StatusInternalServerError)
		return
	}

	// Le state est conservé côté client dans un cookie HttpOnly : il n'y a pas
	// de session serveur, et sa seule fonction est d'être relu au retour pour
	// écarter une requête forgée.
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookie,
		Value:    state,
		Path:     "/auth/netatmo",
		HttpOnly: true,
		Secure:   strings.HasPrefix(h.deps.Config.PublicURL, "https://"),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((10 * time.Minute).Seconds()),
	})

	http.Redirect(w, r, h.deps.Netatmo.AuthCodeURL(state), http.StatusFound)
}

// callback reçoit le code d'autorisation et le convertit en jetons persistés.
func (h *authHandler) callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	if e := q.Get("error"); e != "" {
		h.deps.Log.Warn("consentement Netatmo refusé", "error", e)
		http.Error(w, "autorisation refusée: "+e, http.StatusBadRequest)
		return
	}

	cookie, err := r.Cookie(stateCookie)
	if err != nil {
		http.Error(w, "état OAuth2 absent ou expiré, relancez /auth/netatmo", http.StatusBadRequest)
		return
	}
	// Comparaison à temps constant : le state est un secret de courte durée.
	if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(q.Get("state"))) != 1 {
		http.Error(w, "état OAuth2 invalide", http.StatusBadRequest)
		return
	}

	// Le state a servi : l'invalider immédiatement.
	http.SetCookie(w, &http.Cookie{Name: stateCookie, Path: "/auth/netatmo", MaxAge: -1})

	code := q.Get("code")
	if code == "" {
		http.Error(w, "code d'autorisation absent", http.StatusBadRequest)
		return
	}

	if err := h.deps.Netatmo.Exchange(r.Context(), code); err != nil {
		h.deps.Log.Error("échange du code Netatmo", "error", err)
		http.Error(w, "échec de l'authentification Netatmo", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	//nolint:errcheck // rien à faire si l'écriture de la page de confirmation échoue
	w.Write([]byte(`<!doctype html><html lang="fr"><meta charset="utf-8">
<title>Netatmo connecté</title>
<body style="font-family:system-ui;max-width:32rem;margin:4rem auto">
<h1>Netatmo connecté</h1>
<p>Les jetons ont été enregistrés. Les données seront rafraîchies au prochain cycle de polling.</p>
<p><a href="/">Retour à l'application</a></p>
</body></html>`))
}

// status indique si une authentification Netatmo est active, sans exposer le
// moindre élément du jeton.
func (h *authHandler) status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.deps.Netatmo.Authenticated(r.Context()) {
		//nolint:errcheck // réponse courte, une écriture partielle n'appelle aucun traitement
		w.Write([]byte(`{"configured":true,"authenticated":true}`))
		return
	}
	//nolint:errcheck // idem
	w.Write([]byte(`{"configured":true,"authenticated":false,"authorize_url":"/auth/netatmo"}`))
}

// randomState produit un state OAuth2 imprévisible.
func randomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// unconfigured répond aux routes OAuth2 quand l'intégration Netatmo n'a pas
// d'identifiants. Une page explicite vaut mieux qu'un renvoi silencieux vers
// l'application, qui donnerait l'impression d'un lien cassé.
func unconfigured(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	//nolint:errcheck // page statique : une écriture partielle n'appelle aucun traitement
	w.Write([]byte(`<!doctype html><html lang="fr"><meta charset="utf-8">
<title>Netatmo non configuré</title>
<body style="font-family:system-ui;max-width:34rem;margin:4rem auto;line-height:1.6">
<h1>Netatmo non configuré</h1>
<p>Le service n'a pas d'identifiants Netatmo : l'intégration est inactive.</p>
<ol>
  <li>Créer une application sur <a href="https://dev.netatmo.com/apps/">dev.netatmo.com</a>.</li>
  <li>Y déclarer l'URL de redirection affichée par le service au démarrage,
      au caractère près.</li>
  <li>Renseigner <code>NETATMO_CLIENT_ID</code> et <code>NETATMO_CLIENT_SECRET</code>
      dans <code>.env</code>, puis redémarrer.</li>
</ol>
<p><a href="/">Retour à l'application</a></p>
</body></html>`))
}

// unconfiguredStatus conserve le contrat JSON de /auth/netatmo/status, pour que
// le frontend distingue « non configuré » de « configuré mais pas authentifié ».
func unconfiguredStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	//nolint:errcheck // réponse courte, une écriture partielle n'appelle aucun traitement
	w.Write([]byte(`{"configured":false,"authenticated":false}`))
}
