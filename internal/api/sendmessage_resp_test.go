package api

import (
	"encoding/json"
	"testing"
)

// Issue #115: the server returns message_id as a number; decoding it as a string turned every successful send into
// "unmarshaling sendMessage response: json: cannot unmarshal number into Go struct field SendMessageResp.message_id".
func TestSendMessageRespDecodesNumericMessageID(t *testing.T) {
	var resp SendMessageResp
	if err := json.Unmarshal([]byte(`{"message_id":7314506598200000001,"ret":0,"errmsg":"ok"}`), &resp); err != nil {
		t.Fatalf("成功发送的回包解析失败: %v", err)
	}
	if resp.MessageID != 7314506598200000001 || resp.Ret == nil || *resp.Ret != 0 {
		t.Errorf("解析结果: %+v", resp)
	}
	// Quoted ids (older fixtures) keep working; garbage is rejected instead of silently zeroed.
	if err := json.Unmarshal([]byte(`{"message_id":"42","ret":0}`), &resp); err != nil || resp.MessageID != 42 {
		t.Errorf("带引号的 id 也该能解: %v %+v", err, resp)
	}
	if err := json.Unmarshal([]byte(`{"message_id":"abc"}`), &resp); err == nil {
		t.Error("非数字的 message_id 该报错")
	}
}
