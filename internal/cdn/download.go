package cdn

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// DownloadAndDecrypt downloads a CDN file and decrypts it with AES-128-ECB.
// aesKeyBase64 supports both 16-byte raw and 32-char hex encodings.
func DownloadAndDecrypt(ctx context.Context, encryptedQueryParam, aesKeyBase64, cdnBaseURL, fullURL string) ([]byte, error) {
	key, err := ParseAESKey(aesKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("parsing AES key: %w", err)
	}

	downloadURL := resolveDownloadURL(encryptedQueryParam, cdnBaseURL, fullURL)
	encrypted, err := fetchCDNBytes(ctx, downloadURL)
	if err != nil {
		return nil, err
	}

	decrypted, err := DecryptAESECB(encrypted, key)
	if err != nil {
		return nil, fmt.Errorf("decrypting CDN data: %w", err)
	}

	return decrypted, nil
}

// DownloadPlain downloads a CDN file without decryption.
func DownloadPlain(ctx context.Context, encryptedQueryParam, cdnBaseURL, fullURL string) ([]byte, error) {
	downloadURL := resolveDownloadURL(encryptedQueryParam, cdnBaseURL, fullURL)
	return fetchCDNBytes(ctx, downloadURL)
}

// DownloadAndDecryptWithHexKey downloads and decrypts using a hex-encoded AES key.
// Used for image_item.aeskey which is a hex string (not base64).
func DownloadAndDecryptWithHexKey(ctx context.Context, encryptedQueryParam, aesKeyHex, cdnBaseURL, fullURL string) ([]byte, error) {
	keyBytes, err := hex.DecodeString(aesKeyHex)
	if err != nil {
		return nil, fmt.Errorf("hex decoding aeskey: %w", err)
	}
	aesKeyBase64 := base64.StdEncoding.EncodeToString(keyBytes)
	return DownloadAndDecrypt(ctx, encryptedQueryParam, aesKeyBase64, cdnBaseURL, fullURL)
}

func resolveDownloadURL(encryptedQueryParam, cdnBaseURL, fullURL string) string {
	if fullURL != "" {
		return fullURL
	}
	return fmt.Sprintf("%s/download?encrypted_query_param=%s",
		cdnBaseURL, url.QueryEscape(encryptedQueryParam))
}

func fetchCDNBytes(ctx context.Context, downloadURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating CDN download request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("CDN download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("CDN download %d %s: %s", resp.StatusCode, resp.Status, string(body))
	}

	return io.ReadAll(resp.Body)
}
