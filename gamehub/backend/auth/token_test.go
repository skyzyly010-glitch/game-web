package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var testSecret = []byte("a secret key with at least 32 bytes long")

func newTestIssuer(t *testing.T, ttl time.Duration) *Issuer {
	t.Helper()
	iss, err := NewIssuer(testSecret, ttl)
	if err != nil {
		t.Fatal(err)
	}
	return iss
}

func TestIssueAndParse(t *testing.T) {
	iss := newTestIssuer(t, time.Hour)
	token, err := iss.Issue(42, 7)
	if err != nil {
		t.Fatal(err)
	}
	id, version, err := iss.Parse(token)
	if err != nil {
		t.Fatal(err)
	}
	if id != 42 || version != 7 {
		t.Fatalf("Parse() = (%d, %d), want (42, 7)", id, version)
	}
}

func TestParseRejectsTamperedToken(t *testing.T) {
	iss := newTestIssuer(t, time.Hour)
	token, _ := iss.Issue(42, 0)
	sig := strings.LastIndexByte(token, '.') + 1
	repl := byte('x')
	if token[sig] == repl {
		repl = 'y'
	}
	tampered := token[:sig] + string(repl) + token[sig+1:]
	if _, _, err := iss.Parse(tampered); err == nil {
		t.Fatal("Parse() accepted a tampered token")
	}
}

func TestParseRejectsExpiredToken(t *testing.T) {
	iss := newTestIssuer(t, time.Hour)
	// ออกตั๋วที่หมดอายุไปแล้วด้วยกุญแจเดียวกัน
	claims := Claims{RegisteredClaims: jwt.RegisteredClaims{
		Subject:   "42",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
	}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(testSecret)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := iss.Parse(token); err == nil {
		t.Fatal("Parse() accepted an expired token")
	}
}

func TestParseRejectsTokenWithoutExpiry(t *testing.T) {
	iss := newTestIssuer(t, time.Hour)
	claims := Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: "42"}}
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(testSecret)
	if _, _, err := iss.Parse(token); err == nil {
		t.Fatal("Parse() accepted a token with no expiry")
	}
}

func TestParseRejectsOtherSigningMethods(t *testing.T) {
	iss := newTestIssuer(t, time.Hour)
	claims := Claims{RegisteredClaims: jwt.RegisteredClaims{
		Subject:   "42",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	hs512, _ := jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString(testSecret)
	if _, _, err := iss.Parse(hs512); err == nil {
		t.Fatal("Parse() accepted an HS512 token")
	}
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, _, err := iss.Parse(none); err == nil {
		t.Fatal("Parse() accepted an unsigned (alg=none) token")
	}
}

func TestParseRejectsTokenFromAnotherSecret(t *testing.T) {
	other, err := NewIssuer([]byte("another secret key with 32+ bytes!!"), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	token, _ := other.Issue(42, 0)
	if _, _, err := newTestIssuer(t, time.Hour).Parse(token); err == nil {
		t.Fatal("Parse() accepted a token signed with a different secret")
	}
}

func TestParseRejectsBadSubject(t *testing.T) {
	iss := newTestIssuer(t, time.Hour)
	for _, sub := range []string{"", "abc", "0", "-5"} {
		claims := Claims{RegisteredClaims: jwt.RegisteredClaims{
			Subject:   sub,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}}
		token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(testSecret)
		if _, _, err := iss.Parse(token); err == nil {
			t.Errorf("Parse() accepted subject %q", sub)
		}
	}
}

func TestNewIssuerValidatesInput(t *testing.T) {
	if _, err := NewIssuer([]byte("short"), time.Hour); err == nil {
		t.Error("NewIssuer accepted a short secret")
	}
	if _, err := NewIssuer(testSecret, 0); err == nil {
		t.Error("NewIssuer accepted a zero ttl")
	}
}
