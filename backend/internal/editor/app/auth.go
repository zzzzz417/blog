package app

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrUnauthorized = errors.New("invalid editor session")

type Service struct {
	keyHash    [32]byte
	signingKey []byte
	enabled    bool
	ttl        time.Duration
	now        func() time.Time
}

func NewService(editorKey, signingKey string, ttl time.Duration) *Service {
	return &Service{keyHash: sha256.Sum256([]byte(editorKey)), signingKey: []byte(signingKey), enabled: len(editorKey) >= 16 && len(signingKey) >= 32, ttl: ttl, now: time.Now}
}

func (s *Service) Enabled() bool { return s.enabled }

func (s *Service) Login(key string) (string, time.Time, error) {
	hash := sha256.Sum256([]byte(key))
	if !s.enabled || subtle.ConstantTimeCompare(hash[:], s.keyHash[:]) != 1 {
		return "", time.Time{}, ErrUnauthorized
	}
	now := s.now()
	expires := now.Add(s.ttl)
	claims := jwt.RegisteredClaims{
		Issuer: "listening", Subject: "editor", Audience: jwt.ClaimStrings{"listening-editor"},
		IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(expires),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.signingKey)
	return token, expires, err
}

func (s *Service) Verify(raw string) (time.Time, error) {
	if !s.enabled || len(raw) > 4096 {
		return time.Time{}, ErrUnauthorized
	}
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) { return s.signingKey, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("listening"), jwt.WithAudience("listening-editor"), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithTimeFunc(s.now))
	if err != nil || !token.Valid || claims.Subject != "editor" {
		return time.Time{}, ErrUnauthorized
	}
	return claims.ExpiresAt.Time, nil
}
