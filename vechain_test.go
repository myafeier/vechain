package vechain

import (
	"context"
	"testing"
)

func TestGenerate(t *testing.T) {

	tokenServer := NewDefaultToken(&config)

	req := new(GenerateRequest)
	req.Quantity = 1
	req.RequestNo = "112"

	ctx := context.WithValue(context.Background(), "request", req)

	resp, err := Generate(ctx, &config, tokenServer)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", resp)

}
