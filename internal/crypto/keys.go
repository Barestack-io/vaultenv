package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/nacl/secretbox"
)

const (
	argon2Time    = 3
	argon2Memory  = 64 * 1024
	argon2Threads = 4
	argon2KeyLen  = 32
	saltSize      = 16
)

func keysDir() string {
	if override := os.Getenv("VAULTENV_CONFIG_DIR"); override != "" {
		return filepath.Join(override, "keys")
	}
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "vaultenv", "keys")
}

// GenerateKeyPair creates a new X25519 keypair.
func GenerateKeyPair() (priv *[32]byte, pub *[32]byte, err error) {
	priv = new([32]byte)
	if _, err := io.ReadFull(rand.Reader, priv[:]); err != nil {
		return nil, nil, fmt.Errorf("generating random key: %w", err)
	}

	pub = new([32]byte)
	curve25519.ScalarBaseMult(pub, priv)

	return priv, pub, nil
}

// EncryptPrivateKey encrypts a private key with a passphrase using Argon2id + NaCl secretbox.
// Returns: salt (16 bytes) || nonce (24 bytes) || secretbox ciphertext
func EncryptPrivateKey(privKey *[32]byte, passphrase string) ([]byte, error) {
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("generating salt: %w", err)
	}

	derived := deriveKey(passphrase, salt)

	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	var derivedKey [32]byte
	copy(derivedKey[:], derived)

	ciphertext := secretbox.Seal(nil, privKey[:], &nonce, &derivedKey)

	result := make([]byte, 0, saltSize+24+len(ciphertext))
	result = append(result, salt...)
	result = append(result, nonce[:]...)
	result = append(result, ciphertext...)

	return result, nil
}

// DecryptPrivateKey decrypts a private key blob produced by EncryptPrivateKey.
func DecryptPrivateKey(data []byte, passphrase string) (*[32]byte, error) {
	if len(data) < saltSize+24 {
		return nil, fmt.Errorf("encrypted key data too short")
	}

	salt := data[:saltSize]
	var nonce [24]byte
	copy(nonce[:], data[saltSize:saltSize+24])
	ciphertext := data[saltSize+24:]

	derived := deriveKey(passphrase, salt)
	var derivedKey [32]byte
	copy(derivedKey[:], derived)

	plaintext, ok := secretbox.Open(nil, ciphertext, &nonce, &derivedKey)
	if !ok {
		return nil, fmt.Errorf("decryption failed (wrong passphrase?)")
	}

	if len(plaintext) != 32 {
		return nil, fmt.Errorf("unexpected key length: %d", len(plaintext))
	}

	var key [32]byte
	copy(key[:], plaintext)
	return &key, nil
}

func deriveKey(passphrase string, salt []byte) []byte {
	return argon2.IDKey([]byte(passphrase), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
}

// SaveKeyPair saves the keypair to ~/.config/vaultenv/keys/
func SaveKeyPair(priv *[32]byte, pub *[32]byte) error {
	dir := keysDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	privPath := filepath.Join(dir, "private.key")
	if err := os.WriteFile(privPath, priv[:], 0600); err != nil {
		return err
	}

	pubPath := filepath.Join(dir, "public.key")
	return os.WriteFile(pubPath, pub[:], 0644)
}

// LoadPrivateKey loads the private key from local storage.
func LoadPrivateKey() (*[32]byte, error) {
	data, err := os.ReadFile(filepath.Join(keysDir(), "private.key"))
	if err != nil {
		return nil, err
	}
	if len(data) != 32 {
		return nil, fmt.Errorf("invalid private key size: %d", len(data))
	}
	var key [32]byte
	copy(key[:], data)
	return &key, nil
}

// LoadPublicKey loads the public key from local storage.
func LoadPublicKey() (*[32]byte, error) {
	data, err := os.ReadFile(filepath.Join(keysDir(), "public.key"))
	if err != nil {
		return nil, err
	}
	if len(data) != 32 {
		return nil, fmt.Errorf("invalid public key size: %d", len(data))
	}
	var key [32]byte
	copy(key[:], data)
	return &key, nil
}

// HasLocalKeys returns true if local keys exist.
func HasLocalKeys() bool {
	_, err := os.Stat(filepath.Join(keysDir(), "private.key"))
	return err == nil
}

// EncodePublicKey encodes a public key to base64 bytes for storage.
func EncodePublicKey(pub *[32]byte) []byte {
	return []byte(base64.StdEncoding.EncodeToString(pub[:]))
}

// DecodePublicKey decodes a base64-encoded public key.
func DecodePublicKey(data []byte) (*[32]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return nil, err
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("invalid public key size: %d", len(decoded))
	}
	var key [32]byte
	copy(key[:], decoded)
	return &key, nil
}

// EncodePublicKeyString encodes a public key to a base64 string.
func EncodePublicKeyString(pub *[32]byte) string {
	return base64.StdEncoding.EncodeToString(pub[:])
}

// DecodePublicKeyString decodes a base64-encoded public key string.
func DecodePublicKeyString(s string) (*[32]byte, error) {
	return DecodePublicKey([]byte(s))
}
