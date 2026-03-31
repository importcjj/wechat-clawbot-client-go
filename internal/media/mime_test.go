package media

import "testing"

func TestMIMEFromFilename(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"photo.jpg", "image/jpeg"},
		{"photo.JPEG", "image/jpeg"},
		{"image.png", "image/png"},
		{"anim.gif", "image/gif"},
		{"clip.mp4", "video/mp4"},
		{"movie.avi", "video/x-msvideo"},
		{"doc.pdf", "application/pdf"},
		{"archive.zip", "application/zip"},
		{"readme.txt", "text/plain"},
		{"data.json", "application/json"},
		{"unknown.xyz", "application/octet-stream"},
		{"noext", "application/octet-stream"},
		{"", "application/octet-stream"},
	}

	for _, tt := range tests {
		got := MIMEFromFilename(tt.filename)
		if got != tt.want {
			t.Errorf("MIMEFromFilename(%q) = %q, want %q", tt.filename, got, tt.want)
		}
	}
}

func TestIsImage(t *testing.T) {
	if !IsImage("image/jpeg") {
		t.Error("image/jpeg should be image")
	}
	if !IsImage("image/png") {
		t.Error("image/png should be image")
	}
	if IsImage("video/mp4") {
		t.Error("video/mp4 should not be image")
	}
	if IsImage("application/pdf") {
		t.Error("application/pdf should not be image")
	}
}

func TestIsVideo(t *testing.T) {
	if !IsVideo("video/mp4") {
		t.Error("video/mp4 should be video")
	}
	if IsVideo("image/png") {
		t.Error("image/png should not be video")
	}
}
