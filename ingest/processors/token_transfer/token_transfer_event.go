package token_transfer

import (
	"github.com/stellar/go/ingest/address"
	"github.com/stellar/go/ingest/asset"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func NewTransferEvent(meta *EventMeta, from, to *address.Address, amount string, token *asset.Asset) *TokenTransferEvent {
	return &TokenTransferEvent{
		Meta:  meta,
		Asset: token,
		Event: &TokenTransferEvent_Transfer{
			Transfer: &Transfer{
				From:   from,
				To:     to,
				Amount: amount,
			},
		},
	}
}

func NewMintEvent(meta *EventMeta, to *address.Address, amount string, token *asset.Asset) *TokenTransferEvent {
	return &TokenTransferEvent{
		Meta:  meta,
		Asset: token,
		Event: &TokenTransferEvent_Mint{
			Mint: &Mint{
				To:     to,
				Amount: amount,
			},
		},
	}
}

func NewBurnEvent(meta *EventMeta, from *address.Address, amount string, token *asset.Asset) *TokenTransferEvent {
	return &TokenTransferEvent{
		Meta:  meta,
		Asset: token,
		Event: &TokenTransferEvent_Burn{
			Burn: &Burn{
				From:   from,
				Amount: amount,
			},
		},
	}
}

func NewClawbackEvent(meta *EventMeta, from *address.Address, amount string, token *asset.Asset) *TokenTransferEvent {
	return &TokenTransferEvent{
		Meta:  meta,
		Asset: token,
		Event: &TokenTransferEvent_Clawback{
			Clawback: &Clawback{
				From:   from,
				Amount: amount,
			},
		},
	}
}

func NewFeeEvent(ledgerSequence uint32, closedAt time.Time, txHash string, from *address.Address, amount string, token *asset.Asset) *TokenTransferEvent {
	return &TokenTransferEvent{
		Meta: &EventMeta{
			LedgerSequence: ledgerSequence,
			ClosedAt:       timestamppb.New(closedAt),
			TxHash:         txHash,
		},
		Asset: token,
		Event: &TokenTransferEvent_Fee{
			Fee: &Fee{
				From:   from,
				Amount: amount,
			},
		},
	}
}
