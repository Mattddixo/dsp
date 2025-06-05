package utils

import (
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"os"

	"github.com/zeebo/blake3"
)

// HashFile computes the hash of a file using the specified algorithm
func HashFile(path string, algorithm string) (string, error) {
	// Open the file
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create the hasher
	hasher, err := GetHasher(algorithm)
	if err != nil {
		return "", fmt.Errorf("failed to create hasher: %w", err)
	}

	// Copy the file into the hasher
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}

	// Get the hash
	hash := hasher.Sum(nil)
	return fmt.Sprintf("%x", hash), nil
}

// HashReader computes the hash of a reader using the specified algorithm
func HashReader(reader io.Reader, algorithm string) (string, error) {
	// Create the hasher
	hasher, err := GetHasher(algorithm)
	if err != nil {
		return "", fmt.Errorf("failed to create hasher: %w", err)
	}

	// Copy the reader into the hasher
	if _, err := io.Copy(hasher, reader); err != nil {
		return "", fmt.Errorf("failed to hash reader: %w", err)
	}

	// Get the hash
	hash := hasher.Sum(nil)
	return fmt.Sprintf("%x", hash), nil
}

// GetHasher returns a new hasher for the specified algorithm
func GetHasher(algorithm string) (hash.Hash, error) {
	switch algorithm {
	case "blake3":
		return blake3.New(), nil
	case "sha256":
		return sha256.New(), nil
	case "sha512":
		return sha512.New(), nil
	default:
		return nil, fmt.Errorf("unsupported hash algorithm: %s", algorithm)
	}
}
