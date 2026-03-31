package cdn

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestEncryptDecryptAESECB(t *testing.T) {
	tests := []struct {
		name      string
		plaintext []byte
		key       []byte
	}{
		{"empty", []byte{}, bytes.Repeat([]byte{0x01}, 16)},
		{"short", []byte("hello"), bytes.Repeat([]byte{0x42}, 16)},
		{"exact block", bytes.Repeat([]byte{0xAB}, 16), bytes.Repeat([]byte{0xCD}, 16)},
		{"two blocks", bytes.Repeat([]byte{0xDE}, 32), bytes.Repeat([]byte{0xEF}, 16)},
		{"odd size", bytes.Repeat([]byte{0x11}, 37), bytes.Repeat([]byte{0x22}, 16)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := EncryptAESECB(tt.plaintext, tt.key)
			if err != nil {
				t.Fatalf("encrypt: %v", err)
			}

			if len(ciphertext)%16 != 0 {
				t.Fatalf("ciphertext length %d not multiple of 16", len(ciphertext))
			}

			decrypted, err := DecryptAESECB(ciphertext, tt.key)
			if err != nil {
				t.Fatalf("decrypt: %v", err)
			}

			if !bytes.Equal(decrypted, tt.plaintext) {
				t.Fatalf("round-trip mismatch: got %x, want %x", decrypted, tt.plaintext)
			}
		})
	}
}

func TestAESECBPaddedSize(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{0, 16},   // 0 bytes -> 16 (1 byte padding minimum, padded to block)
		{1, 16},
		{15, 16},
		{16, 32},  // full block -> another full block of padding
		{17, 32},
		{31, 32},
		{32, 48},
	}

	for _, tt := range tests {
		got := AESECBPaddedSize(tt.input)
		if got != tt.want {
			t.Errorf("AESECBPaddedSize(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseAESKey_Raw16Bytes(t *testing.T) {
	rawKey := bytes.Repeat([]byte{0x42}, 16)
	encoded := base64.StdEncoding.EncodeToString(rawKey)

	key, err := ParseAESKey(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(key, rawKey) {
		t.Fatalf("key mismatch: got %x, want %x", key, rawKey)
	}
}

func TestParseAESKey_HexEncoded(t *testing.T) {
	rawKey := bytes.Repeat([]byte{0xAB}, 16)
	hexStr := hex.EncodeToString(rawKey) // 32-char hex string
	encoded := base64.StdEncoding.EncodeToString([]byte(hexStr))

	key, err := ParseAESKey(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(key, rawKey) {
		t.Fatalf("key mismatch: got %x, want %x", key, rawKey)
	}
}

func TestParseAESKey_Invalid(t *testing.T) {
	// 20 random bytes -> neither 16 nor 32-char hex
	badKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x01}, 20))
	_, err := ParseAESKey(badKey)
	if err == nil {
		t.Fatal("expected error for invalid key length")
	}
}
