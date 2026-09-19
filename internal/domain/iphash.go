package domain

import (
	"crypto/sha256"
	"encoding/hex"
)

func HashIP(salt, ip string) string {
	sum := sha256.Sum256([]byte(salt + ip))
	return hex.EncodeToString(sum[:])
}
