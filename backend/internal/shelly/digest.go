package shelly

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// digestUser est l'utilisateur imposé par les modules Gen2+ : seul le mot de
// passe est configurable.
const digestUser = "admin"

// challenge porte les paramètres d'un en-tête WWW-Authenticate: Digest.
type challenge struct {
	realm     string
	nonce     string
	qop       string
	algorithm string
}

// parseChallenge lit un en-tête de la forme
//
//	Digest qop="auth", realm="shellypro3-841fe88e5b68", nonce="60dc59c6", algorithm=SHA-256
func parseChallenge(header string) (challenge, error) {
	rest, ok := strings.CutPrefix(header, "Digest ")
	if !ok {
		return challenge{}, fmt.Errorf("schéma d'authentification inattendu: %q", header)
	}

	params := map[string]string{}
	for _, part := range splitParams(rest) {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		params[strings.ToLower(strings.TrimSpace(k))] = strings.Trim(strings.TrimSpace(v), `"`)
	}

	c := challenge{
		realm:     params["realm"],
		nonce:     params["nonce"],
		qop:       params["qop"],
		algorithm: params["algorithm"],
	}
	if c.nonce == "" {
		return challenge{}, fmt.Errorf("défi d'authentification sans nonce: %q", header)
	}
	// Les modules Gen2+ n'emploient que SHA-256 ; MD5, que la RFC 7616
	// conserve par compatibilité, n'est pas pris en charge.
	if c.algorithm != "" && !strings.EqualFold(c.algorithm, "SHA-256") {
		return challenge{}, fmt.Errorf("algorithme d'authentification non pris en charge: %s", c.algorithm)
	}
	return c, nil
}

// splitParams découpe sur les virgules situées hors des guillemets.
func splitParams(s string) []string {
	var (
		out    []string
		quoted bool
		start  int
	)
	for i, r := range s {
		switch r {
		case '"':
			quoted = !quoted
		case ',':
			if !quoted {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}

// authorization construit l'en-tête Authorization répondant au défi, selon la
// RFC 7616 avec qop=auth.
func (c challenge) authorization(method, uri, password, cnonce string) string {
	const nc = "00000001"
	response := digestResponse(digestUser, c.realm, password, method, uri, c.nonce, nc, cnonce)

	return fmt.Sprintf(
		`Digest username="%s", realm="%s", nonce="%s", uri="%s", algorithm=SHA-256, qop=auth, nc=%s, cnonce="%s", response="%s"`,
		digestUser, c.realm, c.nonce, uri, nc, cnonce, response)
}

// digestResponse calcule la valeur `response` de la RFC 7616 (SHA-256, qop=auth).
func digestResponse(user, realm, password, method, uri, nonce, nc, cnonce string) string {
	ha1 := sha256Hex(user + ":" + realm + ":" + password)
	ha2 := sha256Hex(method + ":" + uri)
	return sha256Hex(ha1 + ":" + nonce + ":" + nc + ":" + cnonce + ":auth:" + ha2)
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// newCnonce tire le nonce client.
func newCnonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // ne peut pas échouer depuis Go 1.24
	return hex.EncodeToString(b)
}
