package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptSingleRecipient(t *testing.T) {
	priv, pub, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	engine := NewNaClEngine()
	plaintext := []byte("DATABASE_URL=postgres://localhost/mydb\nSECRET_KEY=abc123")

	recipients := map[string][32]byte{"alice": *pub}
	ciphertext, envelopes, err := engine.EncryptForRecipients(plaintext, recipients)
	if err != nil {
		t.Fatalf("EncryptForRecipients: %v", err)
	}

	if len(ciphertext) == 0 {
		t.Fatal("ciphertext should not be empty")
	}
	if len(envelopes) != 1 {
		t.Fatalf("expected 1 envelope, got %d", len(envelopes))
	}

	env, ok := envelopes["alice"]
	if !ok {
		t.Fatal("missing envelope for alice")
	}

	decrypted, err := engine.DecryptWithEnvelope(ciphertext, env, priv)
	if err != nil {
		t.Fatalf("DecryptWithEnvelope: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted text should match original.\ngot:  %q\nwant: %q", decrypted, plaintext)
	}
}

func TestEncryptDecryptMultipleRecipients(t *testing.T) {
	privA, pubA, _ := GenerateKeyPair()
	privB, pubB, _ := GenerateKeyPair()

	engine := NewNaClEngine()
	plaintext := []byte("SHARED_SECRET=s3cr3t")

	recipients := map[string][32]byte{
		"alice": *pubA,
		"bob":   *pubB,
	}
	ciphertext, envelopes, err := engine.EncryptForRecipients(plaintext, recipients)
	if err != nil {
		t.Fatalf("EncryptForRecipients: %v", err)
	}

	if len(envelopes) != 2 {
		t.Fatalf("expected 2 envelopes, got %d", len(envelopes))
	}

	decA, err := engine.DecryptWithEnvelope(ciphertext, envelopes["alice"], privA)
	if err != nil {
		t.Fatalf("Alice decrypt: %v", err)
	}
	if !bytes.Equal(decA, plaintext) {
		t.Error("Alice's decrypted text should match original")
	}

	decB, err := engine.DecryptWithEnvelope(ciphertext, envelopes["bob"], privB)
	if err != nil {
		t.Fatalf("Bob decrypt: %v", err)
	}
	if !bytes.Equal(decB, plaintext) {
		t.Error("Bob's decrypted text should match original")
	}
}

func TestDecryptWithWrongKeyFails(t *testing.T) {
	_, pub, _ := GenerateKeyPair()
	wrongPriv, _, _ := GenerateKeyPair()

	engine := NewNaClEngine()
	plaintext := []byte("SECRET=value")

	recipients := map[string][32]byte{"alice": *pub}
	ciphertext, envelopes, err := engine.EncryptForRecipients(plaintext, recipients)
	if err != nil {
		t.Fatalf("EncryptForRecipients: %v", err)
	}

	_, err = engine.DecryptWithEnvelope(ciphertext, envelopes["alice"], wrongPriv)
	if err == nil {
		t.Error("decryption with wrong key should fail")
	}
}

func TestWrapUnwrapKeyRoundtrip(t *testing.T) {
	priv, pub, _ := GenerateKeyPair()
	engine := NewNaClEngine()

	var symKey [32]byte
	copy(symKey[:], []byte("this-is-a-32-byte-symmetric-key!"))

	envelope, err := engine.WrapKeyForRecipient(&symKey, pub)
	if err != nil {
		t.Fatalf("WrapKeyForRecipient: %v", err)
	}

	unwrapped, err := engine.UnwrapKey(envelope, priv)
	if err != nil {
		t.Fatalf("UnwrapKey: %v", err)
	}

	if *unwrapped != symKey {
		t.Error("unwrapped key should match original")
	}
}

func TestUnwrapKeyWithWrongKeyFails(t *testing.T) {
	_, pub, _ := GenerateKeyPair()
	wrongPriv, _, _ := GenerateKeyPair()
	engine := NewNaClEngine()

	var symKey [32]byte
	copy(symKey[:], []byte("this-is-a-32-byte-symmetric-key!"))

	envelope, err := engine.WrapKeyForRecipient(&symKey, pub)
	if err != nil {
		t.Fatalf("WrapKeyForRecipient: %v", err)
	}

	_, err = engine.UnwrapKey(envelope, wrongPriv)
	if err == nil {
		t.Error("unwrap with wrong key should fail")
	}
}

func TestDecryptCorruptedCiphertext(t *testing.T) {
	priv, pub, _ := GenerateKeyPair()
	engine := NewNaClEngine()

	recipients := map[string][32]byte{"user": *pub}
	ciphertext, envelopes, err := engine.EncryptForRecipients([]byte("data"), recipients)
	if err != nil {
		t.Fatalf("EncryptForRecipients: %v", err)
	}

	// Corrupt the ciphertext
	ciphertext[len(ciphertext)-1] ^= 0xff

	_, err = engine.DecryptWithEnvelope(ciphertext, envelopes["user"], priv)
	if err == nil {
		t.Error("decryption of corrupted ciphertext should fail")
	}
}

func TestEncryptEmptyPlaintext(t *testing.T) {
	priv, pub, _ := GenerateKeyPair()
	engine := NewNaClEngine()

	recipients := map[string][32]byte{"user": *pub}
	ciphertext, envelopes, err := engine.EncryptForRecipients([]byte{}, recipients)
	if err != nil {
		t.Fatalf("EncryptForRecipients with empty plaintext: %v", err)
	}

	decrypted, err := engine.DecryptWithEnvelope(ciphertext, envelopes["user"], priv)
	if err != nil {
		t.Fatalf("DecryptWithEnvelope: %v", err)
	}

	if len(decrypted) != 0 {
		t.Errorf("expected empty plaintext, got %d bytes", len(decrypted))
	}
}

func TestCiphertextTooShort(t *testing.T) {
	priv, pub, _ := GenerateKeyPair()
	engine := NewNaClEngine()

	recipients := map[string][32]byte{"user": *pub}
	_, envelopes, _ := engine.EncryptForRecipients([]byte("data"), recipients)

	_, err := engine.DecryptWithEnvelope([]byte("short"), envelopes["user"], priv)
	if err == nil {
		t.Error("expected error for ciphertext shorter than nonce")
	}
}
