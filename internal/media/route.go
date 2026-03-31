package media

import (
	"context"
	"encoding/base64"
	"strconv"

	"github.com/importcjj/wechat-clawbot-client-go/internal/api"
	"github.com/importcjj/wechat-clawbot-client-go/internal/cdn"
)

// UploadAndBuildItem uploads media data and returns the appropriate MessageItem.
// Routes by MIME type: image/* -> IMAGE, video/* -> VIDEO, else -> FILE.
func UploadAndBuildItem(ctx context.Context, tc *api.TransportConfig, cdnBaseURL string, data []byte, filename, toUserID string) (*api.MessageItem, error) {
	mime := MIMEFromFilename(filename)

	var mediaType int
	switch {
	case IsImage(mime):
		mediaType = api.UploadMediaTypeImage
	case IsVideo(mime):
		mediaType = api.UploadMediaTypeVideo
	default:
		mediaType = api.UploadMediaTypeFile
	}

	uploaded, err := cdn.UploadMedia(ctx, tc, cdnBaseURL, data, toUserID, mediaType)
	if err != nil {
		return nil, err
	}

	aesKeyBase64 := base64.StdEncoding.EncodeToString([]byte(uploaded.AESKeyHex))

	switch mediaType {
	case api.UploadMediaTypeImage:
		return &api.MessageItem{
			Type: api.MessageItemTypeImage,
			ImageItem: &api.ImageItem{
				Media: &api.CDNMedia{
					EncryptQueryParam: uploaded.DownloadEncryptedQueryParam,
					AESKey:            aesKeyBase64,
					EncryptType:       1,
				},
				MidSize: uploaded.FileSizeCiphertext,
			},
		}, nil

	case api.UploadMediaTypeVideo:
		return &api.MessageItem{
			Type: api.MessageItemTypeVideo,
			VideoItem: &api.VideoItem{
				Media: &api.CDNMedia{
					EncryptQueryParam: uploaded.DownloadEncryptedQueryParam,
					AESKey:            aesKeyBase64,
					EncryptType:       1,
				},
				VideoSize: uploaded.FileSizeCiphertext,
			},
		}, nil

	default:
		return &api.MessageItem{
			Type: api.MessageItemTypeFile,
			FileItem: &api.FileItem{
				Media: &api.CDNMedia{
					EncryptQueryParam: uploaded.DownloadEncryptedQueryParam,
					AESKey:            aesKeyBase64,
					EncryptType:       1,
				},
				FileName: filename,
				Len:      intToString(uploaded.FileSize),
			},
		}, nil
	}
}

func intToString(n int) string {
	return strconv.Itoa(n)
}
