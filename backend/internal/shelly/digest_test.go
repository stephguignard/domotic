package shelly

import "testing"

// TestDigestResponseRFC7616 reprend l'exemple SHA-256 de la RFC 7616 (§3.9.1) :
// une référence indépendante de cette implémentation.
func TestDigestResponseRFC7616(t *testing.T) {
	got := digestResponse(
		"Mufasa", "http-auth@example.org", "Circle of Life",
		"GET", "/dir/index.html",
		"7ypf/xlj9XXwfDPEoM4URrv/xwf94BcCAzFZH4GiTo0v",
		"00000001",
		"f2/wE4q74E6zIJEtWaHKaf5wv/H5QzzpXusqGemxURZJ",
	)
	const want = "753927fa0e85d155564e2e272a28d1802ca10daf4496794697cf8db5856cb6c1"
	if got != want {
		t.Errorf("response = %s, attendu %s", got, want)
	}
}

func TestParseChallenge(t *testing.T) {
	c, err := parseChallenge(`Digest qop="auth", realm="shellypro3-841fe88e5b68", nonce="60dc59c6", algorithm=SHA-256`)
	if err != nil {
		t.Fatalf("parseChallenge: %v", err)
	}
	if c.realm != "shellypro3-841fe88e5b68" || c.nonce != "60dc59c6" || c.qop != "auth" {
		t.Errorf("défi mal lu: %+v", c)
	}
}

func TestParseChallengeRejectsMD5(t *testing.T) {
	if _, err := parseChallenge(`Digest realm="x", nonce="1", algorithm=MD5`); err == nil {
		t.Error("attendu un refus de MD5")
	}
}
