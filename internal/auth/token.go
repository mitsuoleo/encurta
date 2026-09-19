package auth

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/esposo/url-shortener/internal/domain"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Tokens struct {
	secret []byte
	ttl    time.Duration
}

func NewTokens(secret string, ttl time.Duration) *Tokens {
	return &Tokens{secret: []byte(secret), ttl: ttl}
}

func HashPassword(plain string) (string, error) {
	if len(plain) < 8 {
		return "", domain.ErrWeakPassword
	}
	b, err := bcrypt.GenerateFromPassword([]byte(plain), 12)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func CheckPassword(hash, plain string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)); err != nil {
		return domain.ErrInvalidCredentials
	}
	return nil
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (t *Tokens) Issue(user domain.User) (string, time.Duration, error) {
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   strconv.FormatInt(user.ID, 10),
		"email": user.Email,
		"iat":   now.Unix(),
		"exp":   now.Add(t.ttl).Unix(),
	})
	signed, err := tok.SignedString(t.secret)
	if err != nil {
		return "", 0, err
	}
	return signed, t.ttl, nil
}

func (t *Tokens) Parse(raw string) (int64, string, error) {
	parsed, err := jwt.Parse(raw, func(tok *jwt.Token) (any, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return t.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !parsed.Valid {
		return 0, "", domain.ErrUnauthorized
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return 0, "", domain.ErrUnauthorized
	}
	sub, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)
	id, err := strconv.ParseInt(sub, 10, 64)
	if err != nil || id < 1 {
		return 0, "", domain.ErrUnauthorized
	}
	return id, email, nil
}
