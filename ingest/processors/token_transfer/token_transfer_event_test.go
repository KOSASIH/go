package token_transfer

import (
	"github.com/stellar/go/ingest/address"
	"github.com/stellar/go/ingest/asset"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"testing"
	"time"
)

func newTestAddress() *address.Address {
	return &address.Address{
		AddressType: address.AddressType_ADDRESS_TYPE_ACCOUNT,
		StrKey:      "GBRX5D3FLJ72FHYFVFF2BOICRVDF7FESIAT6GQ4K3ST2MXXJJZC24H2F",
	}
}

func newTestAsset() *asset.Asset {
	return &asset.Asset{
		AssetType: &asset.Asset_IssuedAsset{
			IssuedAsset: &asset.IssuedAsset{
				AssetCode: "USDC",
				Issuer:    "GA5ZSEJYB37JRC5AVCIA5MOP4RHTM335X2KGX3IHOJAPP5RE34K4KZVN",
			},
		},
	}
}

func newTestEventMeta() *EventMeta {
	return &EventMeta{
		LedgerSequence: 12345,
		ClosedAt:       timestamppb.New(time.Now()),
		TxHash:         "abc123xyz",
	}
}

func TestEventSerialization(t *testing.T) {
	tests := []struct {
		// test fixtureName
		fixtureName string
		// Setup the test fixture
		fixtureSetupFn func() (*TokenTransferEvent, *EventMeta, *asset.Asset, *address.Address, string)
		// Function to assert to see if data matches
		assertFn func(event *TokenTransferEvent) proto.Message
	}{
		{
			fixtureName: "Transfer",
			fixtureSetupFn: func() (*TokenTransferEvent, *EventMeta, *asset.Asset, *address.Address, string) {
				from := newTestAddress()
				to := newTestAddress()
				amount := "1000"
				meta := newTestEventMeta()
				token := newTestAsset()
				event := NewTransferEvent(meta, from, to, amount, token)
				return event, meta, token, from, amount
			},
			assertFn: func(event *TokenTransferEvent) proto.Message {
				return event.GetTransfer()
			},
		},
		{
			fixtureName: "Mint",
			fixtureSetupFn: func() (*TokenTransferEvent, *EventMeta, *asset.Asset, *address.Address, string) {
				to := newTestAddress()
				amount := "500"
				meta := newTestEventMeta()
				token := newTestAsset()
				event := NewMintEvent(meta, to, amount, token)
				return event, meta, token, to, amount
			},
			assertFn: func(event *TokenTransferEvent) proto.Message {
				return event.GetMint()
			},
		},
		{
			fixtureName: "Burn",
			fixtureSetupFn: func() (*TokenTransferEvent, *EventMeta, *asset.Asset, *address.Address, string) {
				from := newTestAddress()
				amount := "200"
				meta := newTestEventMeta()
				token := newTestAsset()
				event := NewBurnEvent(meta, from, amount, token)
				return event, meta, token, from, amount
			},
			assertFn: func(event *TokenTransferEvent) proto.Message {
				return event.GetBurn()
			},
		},
		{
			fixtureName: "Clawback",
			fixtureSetupFn: func() (*TokenTransferEvent, *EventMeta, *asset.Asset, *address.Address, string) {
				from := newTestAddress()
				amount := "300"
				meta := newTestEventMeta()
				token := newTestAsset()
				event := NewClawbackEvent(meta, from, amount, token)
				return event, meta, token, from, amount
			},
			assertFn: func(event *TokenTransferEvent) proto.Message {
				return event.GetClawback()
			},
		},
		{
			fixtureName: "Fee",
			fixtureSetupFn: func() (*TokenTransferEvent, *EventMeta, *asset.Asset, *address.Address, string) {
				from := newTestAddress()
				amount := "50"
				token := newTestAsset()
				event := NewFeeEvent(12345, time.Now(), "abc123xyz", from, amount, token)
				return event, nil, token, from, amount // No meta for Fee event
			},
			assertFn: func(event *TokenTransferEvent) proto.Message {
				return event.GetFee()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.fixtureName, func(t *testing.T) {
			event, _, _, _, _ := tt.fixtureSetupFn()

			data, err := proto.Marshal(event)
			assert.NoError(t, err)

			var deserializedEvent TokenTransferEvent
			err = proto.Unmarshal(data, &deserializedEvent)
			assert.NoError(t, err)

			// Common assertions
			assert.True(t, proto.Equal(event.Meta, deserializedEvent.Meta))
			assert.True(t, proto.Equal(event.Asset, deserializedEvent.Asset))

			// Event-specific assertions via the provided getter function
			assert.True(t, proto.Equal(tt.assertFn(event), tt.assertFn(&deserializedEvent)))
		})
	}
}
