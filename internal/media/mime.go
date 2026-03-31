package media

import (
	"path/filepath"
	"strings"
)

var mimeTypes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".bmp":  "image/bmp",
	".svg":  "image/svg+xml",
	".mp4":  "video/mp4",
	".avi":  "video/x-msvideo",
	".mov":  "video/quicktime",
	".mkv":  "video/x-matroska",
	".webm": "video/webm",
	".mp3":  "audio/mpeg",
	".wav":  "audio/wav",
	".ogg":  "audio/ogg",
	".pdf":  "application/pdf",
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".zip":  "application/zip",
	".gz":   "application/gzip",
	".txt":  "text/plain",
	".html": "text/html",
	".json": "application/json",
}

// MIMEFromFilename guesses a MIME type from a filename extension.
func MIMEFromFilename(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// IsImage returns true if the MIME type starts with "image/".
func IsImage(mime string) bool {
	return strings.HasPrefix(mime, "image/")
}

// IsVideo returns true if the MIME type starts with "video/".
func IsVideo(mime string) bool {
	return strings.HasPrefix(mime, "video/")
}
