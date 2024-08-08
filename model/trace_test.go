package model

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/exvulsec/skyeye/client"
	"github.com/exvulsec/skyeye/utils"
)

func TestListAssetTransferWithDFS(t *testing.T) {
	var call *TransactionTraceCall
	call, ok := utils.Retry(func() (any, error) {
		ctxWithTimeOut, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
		defer cancel()
		err := client.MultiEvmClient()["ethereum"].Client().CallContext(ctxWithTimeOut, &call,
			"debug_traceTransaction",
			common.HexToHash("0xd927843e30c6b2bf43103d83bca6abead648eac3cad0d05b1b0eb84cd87de9b6"),
			map[string]any{
				"tracer": "callTracer",
				"tracerConfig": map[string]any{
					"withLog": true,
				},
			})

		return call, err
	}).(*TransactionTraceCall)
	if !ok {
		t.Fatalf("should have returned true")
	}
	assetTransfers := call.ListTransferEventWithDFS(AssetTransfers{}, "0xd927843e30c6b2bf43103d83bca6abead648eac3cad0d05b1b0eb84cd87de9b6")
	fmt.Println(assetTransfers)
}
