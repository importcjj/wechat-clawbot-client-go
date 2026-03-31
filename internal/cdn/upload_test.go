package cdn

import (
	"encoding/hex"
	"testing"
)

func TestMd5sum(t *testing.T) {
	tests := []struct {
		input []byte
		want  string
	}{
		{[]byte{}, "d41d8cd98f00b204e9800998ecf8427e"},         // MD5 of empty
		{[]byte("hello"), "5d41402abc4b2a76b9719d911017c592"},   // MD5 of "hello"
		{[]byte("Hello World"), "b10a8db164e0754105b7a99be72e3fe5"},
	}

	for _, tt := range tests {
		got := md5sum(tt.input)
		if got != tt.want {
			t.Errorf("md5sum(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRandomHex(t *testing.T) {
	tests := []int{0, 1, 8, 16, 32}

	for _, n := range tests {
		got := randomHex(n)

		// Length should be 2*n (hex encoding)
		if len(got) != 2*n {
			t.Errorf("randomHex(%d) length = %d, want %d", n, len(got), 2*n)
		}

		// Should be valid hex
		if n > 0 {
			if _, err := hex.DecodeString(got); err != nil {
				t.Errorf("randomHex(%d) = %q, not valid hex: %v", n, got, err)
			}
		}
	}

	// Different calls should produce different results (probabilistic)
	a := randomHex(16)
	b := randomHex(16)
	if a == b {
		t.Errorf("two calls to randomHex(16) returned same value: %q", a)
	}
}

func TestBuildUploadURL(t *testing.T) {
	got := buildUploadURL("https://cdn.example.com/c2c", "param=abc&x=1", "filekey123")
	want := "https://cdn.example.com/c2c/upload?encrypted_query_param=param%3Dabc%26x%3D1&filekey=filekey123"
	if got != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", got, want)
	}
}
