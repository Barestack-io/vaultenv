package crypto

import "github.com/Barestack-io/vaultenv/internal/vault"

// Engine defines the encryption/decryption interface.
// Implementations can use different crypto backends (NaCl, age, KMS, etc.).
type Engine interface {
	EncryptForRecipients(plaintext []byte, recipients map[string][32]byte) (ciphertext []byte, envelopes map[string]vault.Envelope, err error)
	DecryptWithEnvelope(ciphertext []byte, envelope vault.Envelope, privateKey *[32]byte) (plaintext []byte, err error)
	WrapKeyForRecipient(symKey *[32]byte, recipientPub *[32]byte) (vault.Envelope, error)
	UnwrapKey(envelope vault.Envelope, privateKey *[32]byte) (*[32]byte, error)
}
