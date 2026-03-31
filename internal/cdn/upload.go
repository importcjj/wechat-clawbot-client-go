package cdn

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/importcjj/wechat-clawbot-client-go/internal/api"
)

const uploadMaxRetries = 3

// UploadedFileInfo is returned after a successful CDN upload.
type UploadedFileInfo struct {
	FileKey                    string
	DownloadEncryptedQueryParam string
	AESKeyHex                  string // 16-byte key as hex
	FileSize                   int    // plaintext bytes
	FileSizeCiphertext         int    // AES-128-ECB padded size
}

// UploadMedia reads data, encrypts, uploads to CDN, and returns info for building a MessageItem.
func UploadMedia(ctx context.Context, tc *api.TransportConfig, cdnBaseURL string, data []byte, toUserID string, mediaType int) (*UploadedFileInfo, error) {
	rawSize := len(data)
	rawMD5 := md5sum(data)
	fileSize := AESECBPaddedSize(rawSize)

	fileKey := randomHex(16)
	aesKey := make([]byte, 16)
	if _, err := rand.Read(aesKey); err != nil {
		return nil, fmt.Errorf("generating AES key: %w", err)
	}
	aesKeyHex := hex.EncodeToString(aesKey)

	uploadResp, err := api.GetUploadURL(ctx, tc, &api.GetUploadURLReq{
		FileKey:    fileKey,
		MediaType:  mediaType,
		ToUserID:   toUserID,
		RawSize:    rawSize,
		RawFileMD5: rawMD5,
		FileSize:   fileSize,
		NoNeedThumb: true,
		AESKey:     aesKeyHex,
	})
	if err != nil {
		return nil, fmt.Errorf("getUploadUrl: %w", err)
	}

	uploadFullURL := strings.TrimSpace(uploadResp.UploadFullURL)
	uploadParam := uploadResp.UploadParam
	if uploadFullURL == "" && uploadParam == "" {
		return nil, fmt.Errorf("getUploadUrl returned no upload URL")
	}

	ciphertext, err := EncryptAESECB(data, aesKey)
	if err != nil {
		return nil, fmt.Errorf("encrypting file: %w", err)
	}

	var cdnURL string
	if uploadFullURL != "" {
		cdnURL = uploadFullURL
	} else {
		cdnURL = buildUploadURL(cdnBaseURL, uploadParam, fileKey)
	}

	downloadParam, err := uploadToCDN(ctx, cdnURL, ciphertext)
	if err != nil {
		return nil, err
	}

	return &UploadedFileInfo{
		FileKey:                    fileKey,
		DownloadEncryptedQueryParam: downloadParam,
		AESKeyHex:                  aesKeyHex,
		FileSize:                   rawSize,
		FileSizeCiphertext:         len(ciphertext),
	}, nil
}

func uploadToCDN(ctx context.Context, cdnURL string, ciphertext []byte) (string, error) {
	var lastErr error

	for attempt := 1; attempt <= uploadMaxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, cdnURL, strings.NewReader(string(ciphertext)))
		if err != nil {
			return "", fmt.Errorf("creating CDN upload request: %w", err)
		}
		req.Header.Set("Content-Type", "application/octet-stream")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < uploadMaxRetries {
				continue
			}
			break
		}
		resp.Body.Close()

		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			errMsg := resp.Header.Get("x-error-message")
			if errMsg == "" {
				errMsg = fmt.Sprintf("status %d", resp.StatusCode)
			}
			return "", fmt.Errorf("CDN upload client error %d: %s", resp.StatusCode, errMsg)
		}

		if resp.StatusCode != 200 {
			errMsg := resp.Header.Get("x-error-message")
			if errMsg == "" {
				errMsg = fmt.Sprintf("status %d", resp.StatusCode)
			}
			lastErr = fmt.Errorf("CDN upload server error: %s", errMsg)
			if attempt < uploadMaxRetries {
				continue
			}
			break
		}

		downloadParam := resp.Header.Get("x-encrypted-param")
		if downloadParam == "" {
			lastErr = fmt.Errorf("CDN response missing x-encrypted-param header")
			if attempt < uploadMaxRetries {
				continue
			}
			break
		}

		return downloadParam, nil
	}

	return "", fmt.Errorf("CDN upload failed after %d attempts: %w", uploadMaxRetries, lastErr)
}

func buildUploadURL(cdnBaseURL, uploadParam, fileKey string) string {
	return fmt.Sprintf("%s/upload?encrypted_query_param=%s&filekey=%s",
		cdnBaseURL,
		url.QueryEscape(uploadParam),
		url.QueryEscape(fileKey))
}

func md5sum(data []byte) string {
	h := md5.New()
	_, _ = io.Copy(h, strings.NewReader(string(data)))
	return hex.EncodeToString(h.Sum(nil))
}

func randomHex(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
