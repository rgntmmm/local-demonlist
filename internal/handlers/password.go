package handlers

import (
	"crypto/subtle"
	"encoding/base64"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
	"strings"
)

// Accept the fixed Argon2id format of existing test accounts and current bcrypt hashes.
func passwordMatches(hash, password string) bool {
	if !strings.HasPrefix(hash, "$argon2id$") {
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
	}
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[2] != "v=19" || parts[3] != "m=65536,t=3,p=1" {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) != 32 {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, 3, 65536, 1, 32)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
