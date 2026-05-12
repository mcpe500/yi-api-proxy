package apikeys

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func GenerateKey(prefix string) (fullKey, keyHash, keyPrefix string, err error) {
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
	keyHash = HashKey(fullKey)
	keyPrefix = fmt.Sprintf("sk-gorouter-%s", prefix)
	return fullKey, keyHash, keyPrefix, nil
}

func HashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}
