package tracing

import (
	"testing"
	"time"

	"github.com/cometbft/cometbft/libs/rand"
	"github.com/cosmos/cosmos-sdk/types/address"
	"github.com/cosmos/cosmos-sdk/x/evidence"
	evidencekeeper "github.com/cosmos/cosmos-sdk/x/evidence/keeper"
	evidencetypes "github.com/cosmos/cosmos-sdk/x/evidence/types"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/types/module"
)

func TestTracingGRPCServer_RegisterService(t *testing.T) {
	tracerEnabled = true
	ctx, marshaler, storeKey := createMinTestInput(t)
	keeper := evidencekeeper.NewKeeper(marshaler, storeKey, nil, nil)
	keeper.SetRouter(evidencetypes.NewRouter())
	anyModule := evidence.NewAppModule(*keeper)
	evidencetypes.RegisterInterfaces(marshaler.InterfaceRegistry())
	mm := NewTraceModuleManager(module.NewManager(anyModule), marshaler)
	msgServiceRouter := baseapp.NewMsgServiceRouter()
	msgServiceRouter.SetInterfaceRegistry(marshaler.InterfaceRegistry())
	queryRouter := baseapp.NewGRPCQueryRouter()
	queryRouter.SetInterfaceRegistry(marshaler.InterfaceRegistry())
	mm.RegisterServices(module.NewConfigurator(marshaler, msgServiceRouter, queryRouter))

	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	// when
	msgSend, err := evidencetypes.NewMsgSubmitEvidence(rand.Bytes(address.Len), &evidencetypes.Equivocation{
		Height:           1,
		Time:             time.Now(),
		Power:            1,
		ConsensusAddress: "anyValidAddress",
	})
	require.NoError(t, err)
	require.NoError(t, msgSend.ValidateBasic()) // sanity check to fail fast here and not in the handler
	h := msgServiceRouter.Handler(msgSend)
	_, err = h(ctx, msgSend)

	// then
	// require.ErrorIs(t, err, evidencetypes.ErrNoEvidenceHandlerExists) // sdk 47
	require.Error(t, err) // sdk 45
	spans := tracer.FinishedSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "service", spans[0].OperationName)
	assert.Contains(t, spans[0].Tags(), tagErrored)
}
