package service

import (
	"context"
	"errors"

	"github.com/mitsuoleo/encurta/internal/auth"
	"github.com/mitsuoleo/encurta/internal/domain"
)

type Users interface {
	CreateUser(ctx context.Context, email, passwordHash string) (domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
}

type AuthService struct {
	users  Users
	tokens *auth.Tokens
}

func NewAuth(users Users, tokens *auth.Tokens) *AuthService {
	return &AuthService{users: users, tokens: tokens}
}

func (a *AuthService) Register(ctx context.Context, email, password string) (domain.User, string, int64, error) {
	email = auth.NormalizeEmail(email)
	if email == "" || !validEmail(email) {
		return domain.User{}, "", 0, domain.ErrInvalidPayload
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return domain.User{}, "", 0, err
	}
	user, err := a.users.CreateUser(ctx, email, hash)
	if err != nil {
		return domain.User{}, "", 0, err
	}
	tok, ttl, err := a.tokens.Issue(user)
	if err != nil {
		return domain.User{}, "", 0, err
	}
	return user, tok, int64(ttl.Seconds()), nil
}

func (a *AuthService) Login(ctx context.Context, email, password string) (domain.User, string, int64, error) {
	email = auth.NormalizeEmail(email)
	user, err := a.users.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			_ = auth.CheckPassword(auth.DummyHash, password)
			return domain.User{}, "", 0, domain.ErrInvalidCredentials
		}
		return domain.User{}, "", 0, err
	}
	if err := auth.CheckPassword(user.PasswordHash, password); err != nil {
		return domain.User{}, "", 0, err
	}
	tok, ttl, err := a.tokens.Issue(user)
	if err != nil {
		return domain.User{}, "", 0, err
	}
	return user, tok, int64(ttl.Seconds()), nil
}

func (a *AuthService) Parse(raw string) (int64, string, error) {
	return a.tokens.Parse(raw)
}

func validEmail(email string) bool {
	at := 0
	for i := 0; i < len(email); i++ {
		if email[i] == '@' {
			at++
		}
	}
	return at == 1 && len(email) >= 3 && email[0] != '@' && email[len(email)-1] != '@'
}
