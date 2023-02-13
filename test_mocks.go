package tracing

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	channeltypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v7/modules/core/05-port/types"
	"github.com/cosmos/ibc-go/v7/modules/core/exported"
)

type MockIBCModule struct {
	OnAcknowledgementPacketFn func(ctx sdk.Context, packet channeltypes.Packet, acknowledgement []byte, relayer sdk.AccAddress) error
	OnRecvPacketFn            func(ctx sdk.Context, packet channeltypes.Packet, relayer sdk.AccAddress) exported.Acknowledgement
}

func (m MockIBCModule) OnAcknowledgementPacket(ctx sdk.Context, packet channeltypes.Packet, acknowledgement []byte, relayer sdk.AccAddress) error {
	if m.OnAcknowledgementPacketFn == nil {
		panic("not expected to be called")
	}
	return m.OnAcknowledgementPacketFn(ctx, packet, acknowledgement, relayer)
}

func (m MockIBCModule) OnRecvPacket(ctx sdk.Context, packet channeltypes.Packet, relayer sdk.AccAddress) exported.Acknowledgement {
	if m.OnRecvPacketFn == nil {
		panic("not expected to be called")
	}
	return m.OnRecvPacketFn(ctx, packet, relayer)
}

func (m MockIBCModule) OnChanOpenInit(ctx sdk.Context, order channeltypes.Order, connectionHops []string, portID string, channelID string, channelCap *capabilitytypes.Capability, counterparty channeltypes.Counterparty, version string) (string, error) {
	panic("implement me")
}

func (m MockIBCModule) OnChanOpenTry(ctx sdk.Context, order channeltypes.Order, connectionHops []string, portID, channelID string, channelCap *capabilitytypes.Capability, counterparty channeltypes.Counterparty, counterpartyVersion string) (version string, err error) {
	panic("implement me")
}

func (m MockIBCModule) OnChanOpenAck(ctx sdk.Context, portID, channelID string, counterpartyChannelID string, counterpartyVersion string) error {
	panic("implement me")
}

func (m MockIBCModule) OnChanOpenConfirm(ctx sdk.Context, portID, channelID string) error {
	panic("implement me")
}

func (m MockIBCModule) OnChanCloseInit(ctx sdk.Context, portID, channelID string) error {
	panic("implement me")
}

func (m MockIBCModule) OnChanCloseConfirm(ctx sdk.Context, portID, channelID string) error {
	panic("implement me")
}

func (m MockIBCModule) OnTimeoutPacket(ctx sdk.Context, packet channeltypes.Packet, relayer sdk.AccAddress) error {
	panic("implement me")
}

var _ porttypes.IBCModule = &MockIBCModule{}
