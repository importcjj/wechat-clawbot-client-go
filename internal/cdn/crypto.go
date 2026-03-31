package cdn

import (
	"crypto/aes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
)

var hexPattern = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)

// EncryptAESECB encrypts plaintext with AES-128-ECB and PKCS7 padding.
func EncryptAESECB(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}

	padded := pkcs7Pad(plaintext, aes.BlockSize)
	ciphertext := make([]byte, len(padded))

	for i := 0; i < len(padded); i += aes.BlockSize {
		block.Encrypt(ciphertext[i:i+aes.BlockSize], padded[i:i+aes.BlockSize])
	}

	return ciphertext, nil
}

// DecryptAESECB decrypts ciphertext with AES-128-ECB and removes PKCS7 padding.
func DecryptAESECB(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}

	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext length %d is not a multiple of block size %d", len(ciphertext), aes.BlockSize)
	}

	plaintext := make([]byte, len(ciphertext))
	for i := 0; i < len(ciphertext); i += aes.BlockSize {
		block.Decrypt(plaintext[i:i+aes.BlockSize], ciphertext[i:i+aes.BlockSize])
	}

	return pkcs7Unpad(plaintext)
}

// AESECBPaddedSize computes ciphertext size after PKCS7 padding.
func AESECBPaddedSize(plaintextSize int) int {
	// PKCS7 always adds at least 1 byte of padding
	return ((plaintextSize + 1 + aes.BlockSize - 1) / aes.BlockSize) * aes.BlockSize
}

// ParseAESKey decodes a base64-encoded AES key.
// Two formats are seen in the wild:
//   - base64(16 raw bytes) -> direct key
//   - base64(32-char hex string) -> hex decode to 16 bytes
func ParseAESKey(aesKeyBase64 string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(aesKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("base64 decoding aes_key: %w", err)
	}

	if len(decoded) == 16 {
		return decoded, nil
	}

	if len(decoded) == 32 && hexPattern.Match(decoded) {
		key, err := hex.DecodeString(string(decoded))
		if err != nil {
			return nil, fmt.Errorf("hex decoding aes_key: %w", err)
		}
		return key, nil
	}

	return nil, fmt.Errorf("aes_key must decode to 16 raw bytes or 32-char hex string, got %d bytes", len(decoded))
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padLen := blockSize - len(data)%blockSize
	padding := make([]byte, padLen)
	for i := range padding {
		padding[i] = byte(padLen)
	}
	return append(data, padding...)
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data for PKCS7 unpad")
	}
	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > len(data) {
		return nil, fmt.Errorf("invalid PKCS7 padding: %d", padLen)
	}
	for i := len(data) - padLen; i < len(data); i++ {
		if data[i] != byte(padLen) {
			return nil, fmt.Errorf("invalid PKCS7 padding at byte %d", i)
		}
	}
	return data[:len(data)-padLen], nil
}
