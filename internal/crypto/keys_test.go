package crypto

import (
	"bytes"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	priv, pub, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}
	if priv == nil || pub == nil {
		t.Fatal("keys should not be nil")
	}

	var zero [32]byte
	if *priv == zero {
		t.Error("private key should not be all zeros")
	}
	if *pub == zero {
		t.Error("public key should not be all zeros")
	}
}

func TestGenerateKeyPairUniqueness(t *testing.T) {
	priv1, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("first GenerateKeyPair: %v", err)
	}
	priv2, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("second GenerateKeyPair: %v", err)
	}
	if *priv1 == *priv2 {
		t.Error("two generated keypairs should have different private keys")
	}
}

func TestEncryptDecryptPrivateKeyRoundtrip(t *testing.T) {
	priv, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	passphrase := "TestP@ssw0rd123"
	encrypted, err := EncryptPrivateKey(priv, passphrase)
	if err != nil {
		t.Fatalf("EncryptPrivateKey: %v", err)
	}

	if len(encrypted) == 0 {
		t.Fatal("encrypted data should not be empty")
	}

	decrypted, err := DecryptPrivateKey(encrypted, passphrase)
	if err != nil {
		t.Fatalf("DecryptPrivateKey: %v", err)
	}

	if *decrypted != *priv {
		t.Error("decrypted key should match original")
	}
}

func TestDecryptPrivateKeyWrongPassphrase(t *testing.T) {
	priv, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	encrypted, err := EncryptPrivateKey(priv, "CorrectP@ss1!")
	if err != nil {
		t.Fatalf("EncryptPrivateKey: %v", err)
	}

	_, err = DecryptPrivateKey(encrypted, "WrongP@ssword1!")
	if err == nil {
		t.Error("expected error when decrypting with wrong passphrase")
	}
}

func TestDecryptPrivateKeyTruncatedData(t *testing.T) {
	_, err := DecryptPrivateKey([]byte("short"), "pass")
	if err == nil {
		t.Error("expected error for truncated data")
	}
}

func TestEncodeDecodePublicKeyRoundtrip(t *testing.T) {
	_, pub, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	encoded := EncodePublicKey(pub)
	decoded, err := DecodePublicKey(encoded)
	if err != nil {
		t.Fatalf("DecodePublicKey: %v", err)
	}

	if *decoded != *pub {
		t.Error("decoded key should match original")
	}
}

func TestEncodeDecodePublicKeyStringRoundtrip(t *testing.T) {
	_, pub, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	s := EncodePublicKeyString(pub)
	decoded, err := DecodePublicKeyString(s)
	if err != nil {
		t.Fatalf("DecodePublicKeyString: %v", err)
	}

	if *decoded != *pub {
		t.Error("decoded key should match original")
	}
}

func TestDecodePublicKeyInvalidBase64(t *testing.T) {
	_, err := DecodePublicKey([]byte("not-valid-base64!!!"))
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestDecodePublicKeyWrongLength(t *testing.T) {
	_, err := DecodePublicKey([]byte("AQID")) // decodes to 3 bytes
	if err == nil {
		t.Error("expected error for wrong key length")
	}
}

func TestSaveLoadKeyPairRoundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("VAULTENV_CONFIG_DIR", tmpDir)

	priv, pub, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	if err := SaveKeyPair(priv, pub); err != nil {
		t.Fatalf("SaveKeyPair: %v", err)
	}

	if !HasLocalKeys() {
		t.Error("HasLocalKeys should return true after save")
	}

	loadedPriv, err := LoadPrivateKey()
	if err != nil {
		t.Fatalf("LoadPrivateKey: %v", err)
	}
	if !bytes.Equal(loadedPriv[:], priv[:]) {
		t.Error("loaded private key should match saved")
	}

	loadedPub, err := LoadPublicKey()
	if err != nil {
		t.Fatalf("LoadPublicKey: %v", err)
	}
	if !bytes.Equal(loadedPub[:], pub[:]) {
		t.Error("loaded public key should match saved")
	}
}
