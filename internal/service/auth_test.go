package service

import (
	"context"
	"errors"
	"testing"

	"github.com/mitsuoleo/encurta/internal/auth"
	"github.com/mitsuoleo/encurta/internal/domain"
	"github.com/stretchr/testify/require"
)

type authUsers struct {
	user domain.User
	err  error
}

func (a *authUsers) CreateUser(context.Context, string, string) (domain.User, error) {
	return domain.User{}, errors.New("unused")
}

func (a *authUsers) GetUserByEmail(context.Context, string) (domain.User, error) {
	return a.user, a.err
}

func TestLoginUnknownEmail(t *testing.T) {
	toks := auth.NewTokens("secret", 0)
	svc := NewAuth(&authUsers{err: domain.ErrInvalidCredentials}, toks)
	_, _, _, err := svc.Login(context.Background(), "a@b.com", "password1")
	require.ErrorIs(t, err, domain.ErrInvalidCredentials)
}

func TestLoginDatabaseError(t *testing.T) {
	toks := auth.NewTokens("secret", 0)
	svc := NewAuth(&authUsers{err: errors.New("postgres down")}, toks)
	_, _, _, err := svc.Login(context.Background(), "a@b.com", "password1")
	require.Error(t, err)
	require.False(t, errors.Is(err, domain.ErrInvalidCredentials))
}
