package app

import (
	"github.com/golang-jwt/jwt/v5"
	"strings"
	"testing"
	"time"
)

func TestEditorJWT(t *testing.T) {
	key, secret := "editor-test-key-at-least-16", strings.Repeat("s", 48)
	service := NewService(key, secret, time.Hour)
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	if _, _, err := service.Login("wrong"); err == nil {
		t.Fatal("accepted incorrect login key")
	}
	token, expires, err := service.Login(key)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := service.Verify(token); err != nil || !got.Equal(expires) {
		t.Fatalf("valid token rejected: %v", err)
	}
	parts := strings.Split(token, ".")
	parts[1] = parts[1][:len(parts[1])-2] + "aa"
	if _, err := service.Verify(strings.Join(parts, ".")); err == nil {
		t.Fatal("accepted modified token")
	}
	service.now = func() time.Time { return now.Add(2 * time.Hour) }
	if _, err := service.Verify(token); err == nil {
		t.Fatal("accepted expired token")
	}
	service.now = func() time.Time { return now }
	for _, claims := range []jwt.RegisteredClaims{
		{Issuer: "listening", Subject: "editor", Audience: jwt.ClaimStrings{"listening-editor"}},
		{Issuer: "listening", Subject: "reader", Audience: jwt.ClaimStrings{"listening-editor"}, ExpiresAt: jwt.NewNumericDate(expires)},
		{Issuer: "another-site", Subject: "editor", Audience: jwt.ClaimStrings{"listening-editor"}, ExpiresAt: jwt.NewNumericDate(expires)},
	} {
		raw, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
		if _, err := service.Verify(raw); err == nil {
			t.Fatal("accepted invalid claims")
		}
	}
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(expires)}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := service.Verify(none); err == nil {
		t.Fatal("accepted unsigned JWT")
	}
	if _, err := NewService(key, strings.Repeat("x", 48), time.Hour).Verify(token); err == nil {
		t.Fatal("accepted token after secret rotation")
	}
	if NewService("", "", time.Hour).Enabled() {
		t.Fatal("empty config enabled editor")
	}
}
