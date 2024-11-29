package tracing

import (
	"testing"

	"github.com/opentracing/opentracing-go/mocktracer"

	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	"github.com/CosmWasm/wasmd/x/wasm/keeper/wasmtesting"
	cosmwasm "github.com/CosmWasm/wasmvm"
	wasmvmtypes "github.com/CosmWasm/wasmvm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIBCPacketAck(t *testing.T) {
	specs := map[string]struct {
		querier func(ctx sdk.Context) wasmvmtypes.Querier
		expErr  bool
	}{
		"vanilla querier": {
			querier: func(ctx sdk.Context) wasmvmtypes.Querier {
				return wasmkeeper.QueryHandler{Ctx: ctx}
			},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			tracer := mocktracer.New()
			opentracing.SetGlobalTracer(tracer)
			ctx, _, _ := createMinTestInput(t)
			myRsp := &wasmvmtypes.IBCBasicResponse{}
			myGas := uint64(123)
			mock := &wasmtesting.MockWasmEngine{IBCPacketAckFn: func(codeID cosmwasm.Checksum, env wasmvmtypes.Env, msg wasmvmtypes.IBCPacketAckMsg, store cosmwasm.KVStore, goapi cosmwasm.GoAPI, querier cosmwasm.Querier, gasMeter cosmwasm.GasMeter, gasLimit uint64, deserCost wasmvmtypes.UFraction) (*wasmvmtypes.IBCBasicResponse, uint64, error) {
				return myRsp, myGas, nil
			}}
			en := NewTraceWasmVM(mock)
			msg := wasmvmtypes.IBCPacketAckMsg{}
			querier := spec.querier(ctx)

			gotRsp, gotGas, gotErr := en.IBCPacketAck([]byte{}, wasmvmtypes.Env{}, msg, nil, cosmwasm.GoAPI{}, querier, nil, 1, wasmvmtypes.UFraction{Numerator: 1, Denominator: 1})
			if spec.expErr {
				require.Error(t, gotErr)
				return
			}
			require.NoError(t, gotErr)
			assert.Equal(t, myRsp, gotRsp)
			assert.Equal(t, myGas, gotGas)
		})
	}
}
