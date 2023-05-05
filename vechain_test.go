package vechain

import (
	"context"
	"testing"
)

func TestGenerate(t *testing.T) {

	req := new(GenerateRequest)
	req.Quantity = 1
	req.RequestNo = "111"
	tokenServer := NewDefaultToken(config)

	ctx := context.WithValue(context.Background(), "request", req)

	resp, err := Generate(ctx, config, tokenServer)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", resp)

}
