package domain

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const (
	ShortCodeLength = 7
	Base62Alphabet  = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

func GenerateShortCode() (string, error) {
	buf := make([]byte, ShortCodeLength)
	max := big.NewInt(int64(len(Base62Alphabet)))
	for i := range buf {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("generate short code: %w", err)
		}
		buf[i] = Base62Alphabet[n.Int64()]
	}
	return string(buf), nil
}

func ValidShortCode(code string) bool {
	if len(code) == 0 || len(code) > 20 {
		return false
	}
	for i := 0; i < len(code); i++ {
		c := code[i]
		ok := (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
		if !ok {
			return false
		}
	}
	return true
}

func ValidAlias(alias string) bool {
	if len(alias) < 3 || len(alias) > 20 {
		return false
	}
	if !ValidShortCode(alias) {
		return false
	}
	return !ReservedCode(alias)
}

func ReservedCode(code string) bool {
	switch code {
	case "health", "metrics", "links", "auth", "assets", "static", "favicon", "index":
		return true
	default:
		return false
	}
}
