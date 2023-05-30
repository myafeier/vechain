package vechain

import (
	"context"
	"testing"
)

var tokenServer *DefaultToken

func init() {

	tokenServer = NewDefaultToken(&config)
}

func TestGenerate(t *testing.T) {

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

func TestCommit(t *testing.T) {

	hashV := "e323e330e10f645051f3c6ea0d20f95e38ae1402fbbffc8a3878674a09b11f3b"

	req := new(SubmitRequest)
	req.OperatorUID = config.UserIdOfYuanZhiLian
	req.RequestNo = "113"
	req.HashList = append(req.HashList, &Hash{
		Vid:      "0x636e2e763af79827001685312977c76890b88a516902eb55c09abfd4d43f8dcf",
		DataHash: hashV,
	})

	ctx := context.WithValue(context.Background(), "request", req)

	resp, err := Submit(ctx, &config, tokenServer)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", resp)

}
