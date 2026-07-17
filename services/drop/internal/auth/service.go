package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"time"

	"drop/internal/store"
)

type Service struct {
	store      *store.Store
	codeTTL    time.Duration
	sessionTTL time.Duration
	now        func() time.Time
}

type Options struct {
	CodeTTL    time.Duration
	SessionTTL time.Duration
	Now        func() time.Time
}

type Session struct {
	Token     string
	ExpiresAt time.Time
}

type Code struct {
	Value     string
	ExpiresAt time.Time
}

func NewService(storage *store.Store, opts Options) (*Service, error) {
	if storage == nil || opts.CodeTTL <= 0 || opts.SessionTTL <= 0 {
		return nil, fmt.Errorf("invalid auth service options")
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Service{store: storage, codeTTL: opts.CodeTTL, sessionTTL: opts.SessionTTL, now: opts.Now}, nil
}

func (s *Service) GenerateCode(ctx context.Context) (Code, error) {
	value, err := randomCode()
	if err != nil {
		return Code{}, err
	}
	now := s.now().UTC()
	expires := now.Add(s.codeTTL)
	if err := s.store.CreateAuthCode(ctx, hash(value), now, expires); err != nil {
		return Code{}, err
	}
	return Code{Value: value, ExpiresAt: expires}, nil
}

func (s *Service) RedeemCode(ctx context.Context, code string, metadata store.SessionMetadata) (Session, error) {
	code = normalizeCode(code)
	if code == "" {
		return Session{}, store.ErrCodeInvalid
	}
	token, err := randomToken(32)
	if err != nil {
		return Session{}, err
	}
	now := s.now().UTC()
	expires := now.Add(s.sessionTTL)
	if err := s.store.RedeemAuthCode(ctx, hash(code), hash(token), now, expires, metadata); err != nil {
		return Session{}, err
	}
	return Session{Token: token, ExpiresAt: expires}, nil
}

func (s *Service) ValidateSession(ctx context.Context, token, lastIP string) (store.TrustedSession, bool, error) {
	if token == "" {
		return store.TrustedSession{}, false, nil
	}
	return s.store.SessionByToken(ctx, hash(token), s.now().UTC(), lastIP)
}

func hash(value string) []byte {
	sum := sha256.Sum256([]byte(value))
	return sum[:]
}

func randomCode() (string, error) {
	buffer := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, buffer); err != nil {
		return "", fmt.Errorf("generate authorization code: %w", err)
	}
	value := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buffer)
	return groupCode(value), nil
}

func randomToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, buffer); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func groupCode(value string) string {
	var builder strings.Builder
	for i, char := range value {
		if i > 0 && i%5 == 0 {
			builder.WriteByte('-')
		}
		builder.WriteRune(char)
	}
	return builder.String()
}

func normalizeCode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "")
	if value == "" {
		return ""
	}
	// Store the canonical grouped form so pasted codes may omit separators.
	return groupCode(value)
}
