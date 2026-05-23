package apikeys

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

func GenerateKey(prefix string) (fullKey, keyHash string, keyPrefix string, err error) {
	if prefix == "" {
		b := make([]byte, 3)
		if _, err = rand.Read(b); err != nil {
			return "", "", "", err
		}
		prefix = hex.EncodeToString(b)
	}

	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return "", "", "", err
	}

	fullKey = fmt.Sprintf("sk-gorouter-%s_%s", prefix, hex.EncodeToString(secret))
	keyHash, err = HashKey(fullKey)
	if err != nil {
		return "", "", "", err
	}
	keyPrefix = fmt.Sprintf("sk-gorouter-%s", prefix)
	return fullKey, keyHash, keyPrefix, nil
}

func HashKey(key string) (string, error) {
	// bcrypt has 72-byte limit, so hash first with SHA256
	h := sha256.Sum256([]byte(key))
	// Use base64 to keep it compact for bcrypt
	normalized := base64.RawURLEncoding.EncodeToString(h[:])
	hash, err := bcrypt.GenerateFromPassword([]byte(normalized), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func VerifyKey(key, hash string) bool {
	h := sha256.Sum256([]byte(key))
	normalized := base64.RawURLEncoding.EncodeToString(h[:])
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(normalized))
	return err == nil
}
