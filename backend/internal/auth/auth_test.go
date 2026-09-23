package auth

import (
	"strings"
	"testing"
)

func TestCookieSignatureRejectsTampering(t *testing.T) {
	a := &Auth{secret: []byte(strings.Repeat("s", 32))}
	value := a.signCookie("opaque-token")
	if token, ok := a.verifyCookie(value); !ok || token != "opaque-token" {
		t.Fatalf("valid cookie was rejected: %q %v", token, ok)
	}
	if _, ok := a.verifyCookie(value + "tampered"); ok {
		t.Fatal("tampered cookie was accepted")
	}
}
