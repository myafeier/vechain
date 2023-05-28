package vechain

import "testing"

func TestDefaultToken_GetToken(t *testing.T) {
	token := NewDefaultToken(&config)
	tokenStr := token.GetToken()
	if tokenStr == "" {
		t.Error("get token error")
	}
}
