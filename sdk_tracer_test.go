package tracing

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/stretchr/testify/assert"

	"github.com/CosmWasm/wasmd/x/wasm/keeper/wasmtesting"
	wasmvmtypes "github.com/CosmWasm/wasmvm/types"
	"github.com/cometbft/cometbft/libs/rand"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/address"
	"github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/require"
)

func TestDispatchWithTracingStore(t *testing.T) {
	tracerEnabled = true
	var (
		myKey                   = []byte(`foo`)
		myVal                   = []byte(`bar`)
		randAddr sdk.AccAddress = rand.Bytes(address.Len)
	)
	t.Cleanup(func() { tracerEnabled = false })

	ctx, enc, storeSvc := createMinTestInput(t)
	xctx, commit := ctx.CacheContext()
	mock := wasmtesting.MockMessageHandler{DispatchMsgFn: func(ctx sdk.Context, contractAddr sdk.AccAddress, contractIBCPortID string, msg wasmvmtypes.CosmosMsg) (events []sdk.Event, data [][]byte, err error) {
		return nil, nil, storeSvc.OpenKVStore(ctx).Set(myKey, myVal)
	}}
	m := TraceMessageHandlerDecorator(enc)(&mock)

	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	// when
	_, _, err := m.DispatchMsg(xctx, randAddr, "noIBC", wasmvmtypes.CosmosMsg{Bank: &wasmvmtypes.BankMsg{Send: &wasmvmtypes.SendMsg{
		ToAddress: randAddr.String(),
		Amount:    wasmvmtypes.Coins{wasmvmtypes.NewCoin(123, "ALX")},
	}}})
	commit()
	require.NoError(t, err)

	// then
	spans := tracer.FinishedSpans()
	require.Len(t, spans, 1)
	require.Len(t, spans[0].Logs(), 5)
	idx := make(map[string]string, 5)
	for _, v := range spans[0].Logs() {
		require.Len(t, v.Fields, 1)
		idx[v.Fields[0].Key] = v.Fields[0].ValueString
	}
	require.NotEmpty(t, idx[logGasUsage])
	require.NotEmpty(t, idx[logRawWasmMsg])
	line := idx[logRawStoreIO]

	require.NotEmpty(t, line)
	encoding := base64.StdEncoding
	exp := fmt.Sprintf(`{"operation":"write","key":"%s","value":"%s","metadata":null}`, encoding.EncodeToString(myKey), encoding.EncodeToString(myVal))
	assert.Equal(t, exp, line)
}
