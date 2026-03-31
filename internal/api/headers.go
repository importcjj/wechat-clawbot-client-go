package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const (
	DefaultBaseURL    = "https://ilinkai.weixin.qq.com"
	DefaultCDNBaseURL = "https://novac2c.cdn.weixin.qq.com/c2c"
	DefaultAppID      = "bot"
	DefaultVersion    = "2.1.1"
)

// BuildClientVersion encodes a semver string as uint32: major<<16 | minor<<8 | patch.
// e.g. "2.1.1" -> 0x00020101 = 131329
func BuildClientVersion(version string) uint32 {
	parts := strings.SplitN(version, ".", 3)
	var major, minor, patch uint32
	if len(parts) > 0 {
		v, _ := strconv.ParseUint(parts[0], 10, 8)
		major = uint32(v)
	}
	if len(parts) > 1 {
		v, _ := strconv.ParseUint(parts[1], 10, 8)
		minor = uint32(v)
	}
	if len(parts) > 2 {
		v, _ := strconv.ParseUint(parts[2], 10, 8)
		patch = uint32(v)
	}
	return (major << 16) | (minor << 8) | patch
}

// RandomWechatUIN generates the X-WECHAT-UIN header value:
// random uint32 -> decimal string -> base64.
func RandomWechatUIN() string {
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	uint32Val := binary.BigEndian.Uint32(buf)
	decStr := strconv.FormatUint(uint64(uint32Val), 10)
	return base64.StdEncoding.EncodeToString([]byte(decStr))
}

// SetCommonHeaders adds headers shared by all requests (GET and POST).
func SetCommonHeaders(h http.Header, appID, version, routeTag string) {
	h.Set("iLink-App-Id", appID)
	h.Set("iLink-App-ClientVersion", fmt.Sprintf("%d", BuildClientVersion(version)))
	if routeTag != "" {
		h.Set("SKRouteTag", routeTag)
	}
}

// SetPOSTHeaders adds headers specific to POST requests.
func SetPOSTHeaders(h http.Header, token string, bodyLen int) {
	h.Set("Content-Type", "application/json")
	h.Set("AuthorizationType", "ilink_bot_token")
	h.Set("Content-Length", strconv.Itoa(bodyLen))
	h.Set("X-WECHAT-UIN", RandomWechatUIN())
	if token != "" {
		h.Set("Authorization", "Bearer "+token)
	}
}

// BuildBaseInfo creates the base_info payload for API requests.
func BuildBaseInfo(version string) *BaseInfo {
	return &BaseInfo{ChannelVersion: version}
}
