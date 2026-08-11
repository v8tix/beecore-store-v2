package hash

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

var (
	ErrEmptyPassword   = errors.New("password is required")
	ErrEmptyHash       = errors.New("hash is required")
	ErrInvalidPassword = errors.New("invalid password")
	ErrHashingFailed   = errors.New("password hashing failed")
	ErrSaltGeneration  = errors.New("salt generation failed")
)

// Argon2id configuration constants
const (
	SaltLength = 16        // Salt size in bytes
	Time       = 1         // Number of iterations
	Memory     = 64 * 1024 // Memory usage in KiB
	Threads    = uint8(4)  // Number of threads
	KeyLength  = 32        // Length of the resulting hash in bytes
)

// Argon2idHash hashes the string using Argon2id with a random salt.
func Argon2idHash(input string) (string, error) {
	if len(input) == 0 {
		return "", ErrEmptyPassword
	}

	salt := make([]byte, SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("%w: %v", ErrSaltGeneration, err)
	}

	hash := argon2.IDKey([]byte(input), salt, Time, Memory, Threads, KeyLength)
	return fmt.Sprintf("%s:%s",
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// CheckHash verifies the provided string against the stored Argon2id hash.
func CheckHash(hash, input string) error {
	if len(input) == 0 {
		return ErrEmptyPassword
	}

	if len(hash) == 0 {
		return ErrEmptyHash
	}

	parts := strings.Split(hash, ":")
	if len(parts) != 2 {
		return fmt.Errorf("%w: invalid hash format", ErrHashingFailed)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[0])
	if err != nil {
		return fmt.Errorf("%w: invalid salt encoding: %v", ErrHashingFailed, err)
	}

	hashDecoded, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("%w: invalid hash encoding: %v", ErrHashingFailed, err)
	}

	newHash := argon2.IDKey([]byte(input), salt, Time, Memory, Threads, uint32(len(hashDecoded)))
	if !compareHashes(newHash, hashDecoded) {
		return ErrInvalidPassword
	}

	return nil
}

// compareHashes performs a constant-time comparison of two byte slices.
func compareHashes(hash1, hash2 []byte) bool {
	if len(hash1) != len(hash2) {
		return false
	}

	equal := true
	for i := range hash1 {
		if hash1[i] != hash2[i] {
			equal = false
		}
	}

	return equal
}
