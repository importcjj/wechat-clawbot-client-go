package api

import (
	"encoding/base64"
	"strconv"
	"testing"
)

func TestBuildClientVersion(t *testing.T) {
	tests := []struct {
		version string
		want    uint32
	}{
		{"2.1.1", 0x00020101},   // 131329
		{"1.0.0", 0x00010000},   // 65536
		{"1.0.11", 0x0001000B},  // 65547
		{"0.0.0", 0},
		{"255.255.255", 0x00FFFFFF},
	}

	for _, tt := range tests {
		got := BuildClientVersion(tt.version)
		if got != tt.want {
			t.Errorf("BuildClientVersion(%q) = %d (0x%X), want %d (0x%X)",
				tt.version, got, got, tt.want, tt.want)
		}
	}
}

func TestRandomWechatUIN(t *testing.T) {
	uin := RandomWechatUIN()
	if uin == "" {
		t.Fatal("empty UIN")
	}

	// Should be valid base64
	decoded, err := base64.StdEncoding.DecodeString(uin)
	if err != nil {
		t.Fatalf("invalid base64: %v", err)
	}

	// Decoded should be a decimal number string
	_, err = strconv.ParseUint(string(decoded), 10, 64)
	if err != nil {
		t.Fatalf("decoded UIN %q is not a decimal number: %v", string(decoded), err)
	}
}
