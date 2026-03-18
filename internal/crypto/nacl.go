package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/scaler/vaultenv/internal/vault"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/nacl/secretbox"
)

const (
	symKeySize       = 32
	nonceSize        = 24
	secretboxOverhead = secretbox.Overhead
)

type NaClEngine struct{}

func NewNaClEngine() *NaClEngine {
	return &NaClEngine{}
}

func (n *NaClEngine) EncryptForRecipients(plaintext []byte, recipients map[string][32]byte) ([]byte, map[string]vault.Envelope, error) {
	var symKey [symKeySize]byte
	if _, err := io.ReadFull(rand.Reader, symKey[:]); err != nil {
		return nil, nil, fmt.Errorf("generating symmetric key: %w", err)
	}

	var nonce [nonceSize]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return nil, nil, fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := secretbox.Seal(nonce[:], plaintext, &nonce, &symKey)

	envelopes := make(map[string]vault.Envelope)
	for name, pubKey := range recipients {
		pub := pubKey
		envelope, err := n.WrapKeyForRecipient(&symKey, &pub)
		if err != nil {
			return nil, nil, fmt.Errorf("wrapping key for %s: %w", name, err)
		}
		envelopes[name] = envelope
	}

	return ciphertext, envelopes, nil
}

func (n *NaClEngine) DecryptWithEnvelope(ciphertext []byte, envelope vault.Envelope, privateKey *[32]byte) ([]byte, error) {
	symKey, err := n.UnwrapKey(envelope, privateKey)
	if err != nil {
		return nil, fmt.Errorf("unwrapping key: %w", err)
	}

	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	var nonce [nonceSize]byte
	copy(nonce[:], ciphertext[:nonceSize])

	plaintext, ok := secretbox.Open(nil, ciphertext[nonceSize:], &nonce, symKey)
	if !ok {
		return nil, fmt.Errorf("secretbox decryption failed")
	}

	return plaintext, nil
}

func (n *NaClEngine) WrapKeyForRecipient(symKey *[32]byte, recipientPub *[32]byte) (vault.Envelope, error) {
	ephPub, ephPriv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return vault.Envelope{}, fmt.Errorf("generating ephemeral key: %w", err)
	}

	var nonce [nonceSize]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return vault.Envelope{}, fmt.Errorf("generating nonce: %w", err)
	}

	encrypted := box.Seal(nil, symKey[:], &nonce, recipientPub, ephPriv)

	return vault.Envelope{
		EncryptedKey:    base64.StdEncoding.EncodeToString(encrypted),
		Nonce:           base64.StdEncoding.EncodeToString(nonce[:]),
		EphemeralPublic: base64.StdEncoding.EncodeToString(ephPub[:]),
	}, nil
}

func (n *NaClEngine) UnwrapKey(envelope vault.Envelope, privateKey *[32]byte) (*[32]byte, error) {
	encrypted, err := base64.StdEncoding.DecodeString(envelope.EncryptedKey)
	if err != nil {
		return nil, fmt.Errorf("decoding encrypted key: %w", err)
	}

	nonceBytes, err := base64.StdEncoding.DecodeString(envelope.Nonce)
	if err != nil {
		return nil, fmt.Errorf("decoding nonce: %w", err)
	}

	ephPubBytes, err := base64.StdEncoding.DecodeString(envelope.EphemeralPublic)
	if err != nil {
		return nil, fmt.Errorf("decoding ephemeral public key: %w", err)
	}

	var nonce [nonceSize]byte
	copy(nonce[:], nonceBytes)

	var ephPub [32]byte
	copy(ephPub[:], ephPubBytes)

	decrypted, ok := box.Open(nil, encrypted, &nonce, &ephPub, privateKey)
	if !ok {
		return nil, fmt.Errorf("box decryption failed (wrong key?)")
	}

	if len(decrypted) != symKeySize {
		return nil, fmt.Errorf("unexpected key size: %d", len(decrypted))
	}

	var symKey [symKeySize]byte
	copy(symKey[:], decrypted)

	return &symKey, nil
}
