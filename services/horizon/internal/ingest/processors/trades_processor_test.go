//lint:file-ignore U1001 Ignore all unused code, staticcheck doesn't understand testify/suite

package processors

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/guregu/null"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/toid"
	"github.com/stellar/go/xdr"
)

type TradeProcessorTestSuiteLedger struct {
	suite.Suite
	processor              *TradeProcessor
	mockSession            *db.MockSession
	accountLoader          history.AccountLoaderStub
	lpLoader               history.LiquidityPoolLoaderStub
	assetLoader            history.AssetLoaderStub
	mockBatchInsertBuilder *history.MockTradeBatchInsertBuilder

	unmuxedSourceAccount       xdr.AccountId
	unmuxedOpSourceAccount     xdr.AccountId
	sourceAccount              xdr.MuxedAccount
	opSourceAccount            xdr.MuxedAccount
	strictReceiveTrade         xdr.ClaimAtom
	strictReceiveTradeLP       xdr.ClaimAtom
	strictSendTrade            xdr.ClaimAtom
	strictSendTradeLP          xdr.ClaimAtom
	buyOfferTrade              xdr.ClaimAtom
	sellOfferTrade             xdr.ClaimAtom
	passiveSellOfferTrade      xdr.ClaimAtom
	otherPassiveSellOfferTrade xdr.ClaimAtom
	allTrades                  []xdr.ClaimAtom
	sellPrices                 []xdr.Price

	assets []xdr.Asset

	lpToID             map[xdr.PoolId]int64
	unmuxedAccountToID map[string]int64
	assetToID          map[history.AssetKey]history.Asset

	txs []ingest.LedgerTransaction
	lcm xdr.LedgerCloseMeta
}

func TestTradeProcessorTestSuiteLedger(t *testing.T) {
	suite.Run(t, new(TradeProcessorTestSuiteLedger))
}

func (s *TradeProcessorTestSuiteLedger) SetupTest() {
	s.mockBatchInsertBuilder = &history.MockTradeBatchInsertBuilder{}

	s.unmuxedSourceAccount = xdr.MustAddress("GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY")
	s.sourceAccount = xdr.MuxedAccount{
		Type: xdr.CryptoKeyTypeKeyTypeMuxedEd25519,
		Med25519: &xdr.MuxedAccountMed25519{
			Id:      0xdeadbeef,
			Ed25519: *s.unmuxedSourceAccount.Ed25519,
		},
	}
	s.unmuxedOpSourceAccount = xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML")
	s.opSourceAccount = xdr.MuxedAccount{
		Type: xdr.CryptoKeyTypeKeyTypeMuxedEd25519,
		Med25519: &xdr.MuxedAccountMed25519{
			Id:      0xcafebabe,
			Ed25519: *s.unmuxedOpSourceAccount.Ed25519,
		},
	}
	s.strictReceiveTrade = xdr.ClaimAtom{
		Type: xdr.ClaimAtomTypeClaimAtomTypeOrderBook,
		OrderBook: &xdr.ClaimOfferAtom{
			SellerId:     xdr.MustAddress("GA2YS6YBWIBUMUJCNYROC5TXYTTUA4TCZF7A4MJ2O4TTGT3LFNWIOMY4"),
			OfferId:      11,
			AssetSold:    xdr.MustNewNativeAsset(),
			AmountSold:   111,
			AmountBought: 211,
			AssetBought:  xdr.MustNewCreditAsset("HUF", s.unmuxedSourceAccount.Address()),
		},
	}
	s.strictReceiveTradeLP = xdr.ClaimAtom{
		Type: xdr.ClaimAtomTypeClaimAtomTypeLiquidityPool,
		LiquidityPool: &xdr.ClaimLiquidityAtom{
			LiquidityPoolId: xdr.PoolId{1, 2, 3},
			AssetSold:       xdr.MustNewCreditAsset("MAD", s.unmuxedSourceAccount.Address()),
			AmountSold:      602,
			AssetBought:     xdr.MustNewCreditAsset("GRE", s.unmuxedSourceAccount.Address()),
			AmountBought:    300,
		},
	}
	s.strictSendTrade = xdr.ClaimAtom{
		Type: xdr.ClaimAtomTypeClaimAtomTypeOrderBook,
		OrderBook: &xdr.ClaimOfferAtom{
			SellerId:     xdr.MustAddress("GALOBQKDZUSAEUDE7F4OYUIQTUZBL62G6TRCXU2ED6SA7TL72MBUQSYJ"),
			OfferId:      12,
			AssetSold:    xdr.MustNewCreditAsset("USD", s.unmuxedSourceAccount.Address()),
			AmountSold:   112,
			AmountBought: 212,
			AssetBought:  xdr.MustNewCreditAsset("RUB", s.unmuxedSourceAccount.Address()),
		},
	}
	s.strictSendTradeLP = xdr.ClaimAtom{
		Type: xdr.ClaimAtomTypeClaimAtomTypeLiquidityPool,
		LiquidityPool: &xdr.ClaimLiquidityAtom{
			LiquidityPoolId: xdr.PoolId{4, 5, 6},
			AssetSold:       xdr.MustNewCreditAsset("WER", s.unmuxedSourceAccount.Address()),
			AmountSold:      67,
			AssetBought:     xdr.MustNewCreditAsset("NIJ", s.unmuxedSourceAccount.Address()),
			AmountBought:    98,
		},
	}
	s.buyOfferTrade = xdr.ClaimAtom{
		Type: xdr.ClaimAtomTypeClaimAtomTypeOrderBook,
		OrderBook: &xdr.ClaimOfferAtom{
			SellerId:     xdr.MustAddress("GCWRLPH5X5A3GABFDLDILZ4RLY6O76AYOIIR5H2PAI6TNZZZNLZWBXSH"),
			OfferId:      13,
			AssetSold:    xdr.MustNewCreditAsset("EUR", s.unmuxedSourceAccount.Address()),
			AmountSold:   113,
			AmountBought: 213,
			AssetBought:  xdr.MustNewCreditAsset("NOK", s.unmuxedSourceAccount.Address()),
		},
	}
	s.sellOfferTrade = xdr.ClaimAtom{
		Type: xdr.ClaimAtomTypeClaimAtomTypeOrderBook,
		OrderBook: &xdr.ClaimOfferAtom{
			SellerId:     xdr.MustAddress("GAVOLNFXVVUJOELN4T3YVSH2FFA3VSP2XN4NJRYF2ZWVCHS77C5KXLHZ"),
			OfferId:      14,
			AssetSold:    xdr.MustNewCreditAsset("PLN", s.unmuxedSourceAccount.Address()),
			AmountSold:   114,
			AmountBought: 214,
			AssetBought:  xdr.MustNewCreditAsset("UAH", s.unmuxedSourceAccount.Address()),
		},
	}
	s.passiveSellOfferTrade = xdr.ClaimAtom{
		Type: xdr.ClaimAtomTypeClaimAtomTypeOrderBook,
		OrderBook: &xdr.ClaimOfferAtom{
			SellerId:     xdr.MustAddress("GDQWI6FKB72DPOJE4CGYCFQZKRPQQIOYXRMZ5KEVGXMG6UUTGJMBCASH"),
			OfferId:      15,
			AssetSold:    xdr.MustNewCreditAsset("SEK", s.unmuxedSourceAccount.Address()),
			AmountSold:   115,
			AmountBought: 215,
			AssetBought:  xdr.MustNewCreditAsset("GBP", s.unmuxedSourceAccount.Address()),
		},
	}
	s.otherPassiveSellOfferTrade = xdr.ClaimAtom{
		Type: xdr.ClaimAtomTypeClaimAtomTypeOrderBook,
		OrderBook: &xdr.ClaimOfferAtom{
			SellerId:     xdr.MustAddress("GCPZFOJON3PSSYUBNT7MCGEDSGP47UTSJSB4XGCVEWEJO4XQ6U4XN3N2"),
			OfferId:      16,
			AssetSold:    xdr.MustNewCreditAsset("CHF", s.unmuxedSourceAccount.Address()),
			AmountSold:   116,
			AmountBought: 216,
			AssetBought:  xdr.MustNewCreditAsset("JPY", s.unmuxedSourceAccount.Address()),
		},
	}

	s.unmuxedAccountToID = map[string]int64{
		s.unmuxedSourceAccount.Address():   1000,
		s.unmuxedOpSourceAccount.Address(): 1001,
	}
	s.assetToID = map[history.AssetKey]history.Asset{}
	s.allTrades = []xdr.ClaimAtom{
		s.strictReceiveTrade,
		s.strictSendTrade,
		s.buyOfferTrade,
		s.sellOfferTrade,
		s.passiveSellOfferTrade,
		s.otherPassiveSellOfferTrade,
		s.strictReceiveTradeLP,
		s.strictSendTradeLP,
	}

	s.assets = []xdr.Asset{}
	s.sellPrices = []xdr.Price{}
	s.lpToID = map[xdr.PoolId]int64{}
	for i, trade := range s.allTrades {
		if trade.Type == xdr.ClaimAtomTypeClaimAtomTypeOrderBook {
			s.unmuxedAccountToID[trade.SellerId().Address()] = int64(1002 + i)
			n := xdr.Int32(i + 1)
			s.sellPrices = append(s.sellPrices, xdr.Price{N: n, D: 100})
		} else {
			s.lpToID[trade.MustLiquidityPool().LiquidityPoolId] = int64(3000 + i)
			s.sellPrices = append(s.sellPrices, xdr.Price{N: xdr.Int32(trade.AmountBought()), D: xdr.Int32(trade.AmountSold())})
		}
		if i%2 == 0 {
			s.assetToID[history.AssetKeyFromXDR(trade.AssetSold())] = history.Asset{ID: int64(10000 + i)}
			s.assetToID[history.AssetKeyFromXDR(trade.AssetBought())] = history.Asset{ID: int64(100 + i)}
		} else {
			s.assetToID[history.AssetKeyFromXDR(trade.AssetSold())] = history.Asset{ID: int64(100 + i)}
			s.assetToID[history.AssetKeyFromXDR(trade.AssetBought())] = history.Asset{ID: int64(10000 + i)}
		}
		s.assets = append(s.assets, trade.AssetSold(), trade.AssetBought())
	}

	s.lcm = xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq: xdr.Uint32(100),
				},
			},
		},
	}

	s.accountLoader = history.NewAccountLoaderStub()
	s.assetLoader = history.NewAssetLoaderStub()
	s.lpLoader = history.NewLiquidityPoolLoaderStub()
	s.processor = NewTradeProcessor(
		s.accountLoader.Loader,
		s.lpLoader.Loader,
		s.assetLoader.Loader,
		s.mockBatchInsertBuilder,
	)
}

func (s *TradeProcessorTestSuiteLedger) TearDownTest() {
	s.mockBatchInsertBuilder.AssertExpectations(s.T())
}

func (s *TradeProcessorTestSuiteLedger) TestIgnoreFailedTransactions() {
	ctx := context.Background()
	err := s.processor.ProcessTransaction(s.lcm, createTransaction(false, 1, 2))
	s.Assert().NoError(err)

	err = s.processor.Flush(ctx, s.mockSession)
	s.Assert().NoError(err)
}

func (s *TradeProcessorTestSuiteLedger) mockReadTradeTransactions() []history.InsertTrade {
	ledger := s.lcm.LedgerHeaderHistoryEntry()
	closeTime := time.Unix(int64(ledger.Header.ScpValue.CloseTime), 0).UTC()
	inserts := []history.InsertTrade{
		{
			HistoryOperationID: toid.New(int32(ledger.Header.LedgerSeq), 1, 2).ToInt64(),
			Order:              1,
			LedgerCloseTime:    closeTime,
			BaseAmount:         int64(s.strictReceiveTrade.AmountBought()),
			BaseAccountID:      null.IntFrom(s.unmuxedAccountToID[s.unmuxedOpSourceAccount.Address()]),
			BaseAssetID:        s.assetToID[history.AssetKeyFromXDR(s.strictReceiveTrade.AssetBought())].ID,
			BaseOfferID:        null.IntFrom(EncodeOfferId(uint64(toid.New(int32(ledger.Header.LedgerSeq), 1, 2).ToInt64()), TOIDType)),
			CounterAmount:      int64(s.strictReceiveTrade.AmountSold()),
			CounterAccountID:   null.IntFrom(s.unmuxedAccountToID[s.strictReceiveTrade.SellerId().Address()]),
			CounterAssetID:     s.assetToID[history.AssetKeyFromXDR(s.strictReceiveTrade.AssetSold())].ID,
			CounterOfferID:     null.IntFrom(int64(s.strictReceiveTrade.OfferId())),
			BaseIsSeller:       false,
			BaseIsExact:        null.BoolFrom(false),
			PriceN:             int64(s.sellPrices[0].D),
			PriceD:             int64(s.sellPrices[0].N),
			Type:               history.OrderbookTradeType,
		},
		{
			HistoryOperationID: toid.New(int32(ledger.Header.LedgerSeq), 1, 3).ToInt64(),
			Order:              0,
			LedgerCloseTime:    closeTime,
			CounterAmount:      int64(s.strictSendTrade.AmountBought()),
			CounterAccountID:   null.IntFrom(s.unmuxedAccountToID[s.unmuxedOpSourceAccount.Address()]),
			CounterAssetID:     s.assetToID[history.AssetKeyFromXDR(s.strictSendTrade.AssetBought())].ID,
			CounterOfferID:     null.IntFrom(EncodeOfferId(uint64(toid.New(int32(ledger.Header.LedgerSeq), 1, 3).ToInt64()), TOIDType)),
			BaseAmount:         int64(s.strictSendTrade.AmountSold()),
			BaseAccountID:      null.IntFrom(s.unmuxedAccountToID[s.strictSendTrade.SellerId().Address()]),
			BaseAssetID:        s.assetToID[history.AssetKeyFromXDR(s.strictSendTrade.AssetSold())].ID,
			BaseIsSeller:       true,
			BaseIsExact:        null.BoolFrom(false),
			BaseOfferID:        null.IntFrom(int64(s.strictSendTrade.OfferId())),
			PriceN:             int64(s.sellPrices[1].N),
			PriceD:             int64(s.sellPrices[1].D),
			Type:               history.OrderbookTradeType,
		},
		{
			HistoryOperationID: toid.New(int32(ledger.Header.LedgerSeq), 1, 4).ToInt64(),
			Order:              1,
			LedgerCloseTime:    closeTime,
			BaseOfferID:        null.IntFrom(879136),
			BaseAmount:         int64(s.buyOfferTrade.AmountBought()),
			BaseAccountID:      null.IntFrom(s.unmuxedAccountToID[s.unmuxedOpSourceAccount.Address()]),
			BaseAssetID:        s.assetToID[history.AssetKeyFromXDR(s.buyOfferTrade.AssetBought())].ID,
			CounterAmount:      int64(s.buyOfferTrade.AmountSold()),
			CounterAccountID:   null.IntFrom(s.unmuxedAccountToID[s.buyOfferTrade.SellerId().Address()]),
			CounterAssetID:     s.assetToID[history.AssetKeyFromXDR(s.buyOfferTrade.AssetSold())].ID,
			BaseIsSeller:       false,
			CounterOfferID:     null.IntFrom(int64(s.buyOfferTrade.OfferId())),
			PriceN:             int64(s.sellPrices[2].D),
			PriceD:             int64(s.sellPrices[2].N),
			Type:               history.OrderbookTradeType,
		},
		{
			HistoryOperationID: toid.New(int32(ledger.Header.LedgerSeq), 1, 5).ToInt64(),
			Order:              2,
			LedgerCloseTime:    closeTime,
			CounterAmount:      int64(s.sellOfferTrade.AmountBought()),
			CounterAssetID:     s.assetToID[history.AssetKeyFromXDR(s.sellOfferTrade.AssetBought())].ID,
			CounterAccountID:   null.IntFrom(s.unmuxedAccountToID[s.unmuxedOpSourceAccount.Address()]),
			CounterOfferID:     null.IntFrom(EncodeOfferId(uint64(toid.New(int32(ledger.Header.LedgerSeq), 1, 5).ToInt64()), TOIDType)),
			BaseAmount:         int64(s.sellOfferTrade.AmountSold()),
			BaseAccountID:      null.IntFrom(s.unmuxedAccountToID[s.sellOfferTrade.SellerId().Address()]),
			BaseAssetID:        s.assetToID[history.AssetKeyFromXDR(s.sellOfferTrade.AssetSold())].ID,
			BaseIsSeller:       true,
			BaseOfferID:        null.IntFrom(int64(s.sellOfferTrade.OfferId())),
			PriceN:             int64(s.sellPrices[3].N),
			PriceD:             int64(s.sellPrices[3].D),
			Type:               history.OrderbookTradeType,
		},
		{
			HistoryOperationID: toid.New(int32(ledger.Header.LedgerSeq), 1, 6).ToInt64(),
			Order:              0,
			LedgerCloseTime:    closeTime,
			BaseAmount:         int64(s.passiveSellOfferTrade.AmountBought()),
			BaseAssetID:        s.assetToID[history.AssetKeyFromXDR(s.passiveSellOfferTrade.AssetBought())].ID,
			BaseAccountID:      null.IntFrom(s.unmuxedAccountToID[s.unmuxedSourceAccount.Address()]),
			BaseOfferID:        null.IntFrom(EncodeOfferId(uint64(toid.New(int32(ledger.Header.LedgerSeq), 1, 6).ToInt64()), TOIDType)),
			CounterAmount:      int64(s.passiveSellOfferTrade.AmountSold()),
			CounterAccountID:   null.IntFrom(s.unmuxedAccountToID[s.passiveSellOfferTrade.SellerId().Address()]),
			CounterAssetID:     s.assetToID[history.AssetKeyFromXDR(s.passiveSellOfferTrade.AssetSold())].ID,
			BaseIsSeller:       false,
			CounterOfferID:     null.IntFrom(int64(s.passiveSellOfferTrade.OfferId())),
			PriceN:             int64(s.sellPrices[4].D),
			PriceD:             int64(s.sellPrices[4].N),
			Type:               history.OrderbookTradeType,
		},

		{
			HistoryOperationID: toid.New(int32(ledger.Header.LedgerSeq), 1, 7).ToInt64(),
			Order:              0,
			LedgerCloseTime:    closeTime,

			CounterAmount:    int64(s.otherPassiveSellOfferTrade.AmountBought()),
			CounterAssetID:   s.assetToID[history.AssetKeyFromXDR(s.otherPassiveSellOfferTrade.AssetBought())].ID,
			CounterAccountID: null.IntFrom(s.unmuxedAccountToID[s.unmuxedOpSourceAccount.Address()]),
			CounterOfferID:   null.IntFrom(EncodeOfferId(uint64(toid.New(int32(ledger.Header.LedgerSeq), 1, 7).ToInt64()), TOIDType)),
			BaseAmount:       int64(s.otherPassiveSellOfferTrade.AmountSold()),
			BaseAccountID:    null.IntFrom(s.unmuxedAccountToID[s.otherPassiveSellOfferTrade.SellerId().Address()]),
			BaseAssetID:      s.assetToID[history.AssetKeyFromXDR(s.otherPassiveSellOfferTrade.AssetSold())].ID,
			BaseIsSeller:     true,
			BaseOfferID:      null.IntFrom(int64(s.otherPassiveSellOfferTrade.OfferId())),
			PriceN:           int64(s.sellPrices[5].N),
			PriceD:           int64(s.sellPrices[5].D),
			Type:             history.OrderbookTradeType,
		},
		{
			HistoryOperationID:     toid.New(int32(ledger.Header.LedgerSeq), 1, 8).ToInt64(),
			Order:                  1,
			LedgerCloseTime:        closeTime,
			BaseAmount:             int64(s.strictReceiveTradeLP.AmountBought()),
			BaseAssetID:            s.assetToID[history.AssetKeyFromXDR(s.strictReceiveTradeLP.AssetBought())].ID,
			BaseAccountID:          null.IntFrom(s.unmuxedAccountToID[s.unmuxedOpSourceAccount.Address()]),
			BaseOfferID:            null.IntFrom(EncodeOfferId(uint64(toid.New(int32(ledger.Header.LedgerSeq), 1, 8).ToInt64()), TOIDType)),
			CounterAmount:          int64(s.strictReceiveTradeLP.AmountSold()),
			CounterLiquidityPoolID: null.IntFrom(s.lpToID[s.strictReceiveTradeLP.MustLiquidityPool().LiquidityPoolId]),
			CounterAssetID:         s.assetToID[history.AssetKeyFromXDR(s.strictReceiveTradeLP.AssetSold())].ID,
			BaseIsSeller:           false,
			BaseIsExact:            null.BoolFrom(false),
			LiquidityPoolFee:       null.IntFrom(int64(xdr.LiquidityPoolFeeV18)),
			PriceN:                 int64(s.sellPrices[6].D),
			PriceD:                 int64(s.sellPrices[6].N),
			Type:                   history.LiquidityPoolTradeType,
			RoundingSlippage:       null.IntFrom(0),
		},
		{
			HistoryOperationID:  toid.New(int32(ledger.Header.LedgerSeq), 1, 9).ToInt64(),
			Order:               0,
			LedgerCloseTime:     closeTime,
			CounterAmount:       int64(s.strictSendTradeLP.AmountBought()),
			CounterAssetID:      s.assetToID[history.AssetKeyFromXDR(s.strictSendTradeLP.AssetBought())].ID,
			CounterAccountID:    null.IntFrom(s.unmuxedAccountToID[s.unmuxedOpSourceAccount.Address()]),
			CounterOfferID:      null.IntFrom(EncodeOfferId(uint64(toid.New(int32(ledger.Header.LedgerSeq), 1, 9).ToInt64()), TOIDType)),
			BaseAmount:          int64(s.strictSendTradeLP.AmountSold()),
			BaseLiquidityPoolID: null.IntFrom(s.lpToID[s.strictSendTradeLP.MustLiquidityPool().LiquidityPoolId]),
			BaseAssetID:         s.assetToID[history.AssetKeyFromXDR(s.strictSendTradeLP.AssetSold())].ID,
			BaseIsSeller:        true,
			BaseIsExact:         null.BoolFrom(false),
			LiquidityPoolFee:    null.IntFrom(int64(xdr.LiquidityPoolFeeV18)),
			PriceN:              int64(s.sellPrices[7].N),
			PriceD:              int64(s.sellPrices[7].D),
			Type:                history.LiquidityPoolTradeType,
			RoundingSlippage:    null.IntFrom(0),
		},
	}

	emptyTrade := xdr.ClaimAtom{
		Type: xdr.ClaimAtomTypeClaimAtomTypeOrderBook,
		OrderBook: &xdr.ClaimOfferAtom{
			SellerId:     s.sourceAccount.ToAccountId(),
			OfferId:      123,
			AssetSold:    xdr.MustNewNativeAsset(),
			AmountSold:   0,
			AssetBought:  xdr.MustNewCreditAsset("EUR", s.unmuxedSourceAccount.Address()),
			AmountBought: 0,
		},
	}

	operationResults := []xdr.OperationResult{
		{
			Tr: &xdr.OperationResultTr{
				Type: xdr.OperationTypeBumpSequence,
				BumpSeqResult: &xdr.BumpSequenceResult{
					Code: xdr.BumpSequenceResultCodeBumpSequenceSuccess,
				},
			},
		},
		{
			Tr: &xdr.OperationResultTr{
				Type: xdr.OperationTypePathPaymentStrictReceive,
				PathPaymentStrictReceiveResult: &xdr.PathPaymentStrictReceiveResult{
					Code: xdr.PathPaymentStrictReceiveResultCodePathPaymentStrictReceiveSuccess,
					Success: &xdr.PathPaymentStrictReceiveResultSuccess{
						Offers: []xdr.ClaimAtom{
							emptyTrade,
							s.strictReceiveTrade,
						},
					},
				},
			},
		},
		{
			Tr: &xdr.OperationResultTr{
				Type: xdr.OperationTypePathPaymentStrictSend,
				PathPaymentStrictSendResult: &xdr.PathPaymentStrictSendResult{
					Code: xdr.PathPaymentStrictSendResultCodePathPaymentStrictSendSuccess,
					Success: &xdr.PathPaymentStrictSendResultSuccess{
						Offers: []xdr.ClaimAtom{
							s.strictSendTrade,
							emptyTrade,
						},
					},
				},
			},
		},
		{
			Tr: &xdr.OperationResultTr{
				Type: xdr.OperationTypeManageBuyOffer,
				ManageBuyOfferResult: &xdr.ManageBuyOfferResult{
					Code: xdr.ManageBuyOfferResultCodeManageBuyOfferSuccess,
					Success: &xdr.ManageOfferSuccessResult{
						OffersClaimed: []xdr.ClaimAtom{
							emptyTrade,
							s.buyOfferTrade,
							emptyTrade,
						},
						Offer: xdr.ManageOfferSuccessResultOffer{
							Offer: &xdr.OfferEntry{
								OfferId: 879136,
							},
						},
					},
				},
			},
		},
		{
			Tr: &xdr.OperationResultTr{
				Type: xdr.OperationTypeManageSellOffer,
				ManageSellOfferResult: &xdr.ManageSellOfferResult{
					Code: xdr.ManageSellOfferResultCodeManageSellOfferSuccess,
					Success: &xdr.ManageOfferSuccessResult{
						OffersClaimed: []xdr.ClaimAtom{
							emptyTrade,
							emptyTrade,
							s.sellOfferTrade,
						},
						Offer: xdr.ManageOfferSuccessResultOffer{
							Effect: xdr.ManageOfferEffectManageOfferDeleted,
						},
					},
				},
			},
		},
		{
			Tr: &xdr.OperationResultTr{
				Type: xdr.OperationTypeManageSellOffer,
				ManageSellOfferResult: &xdr.ManageSellOfferResult{
					Code: xdr.ManageSellOfferResultCodeManageSellOfferSuccess,
					Success: &xdr.ManageOfferSuccessResult{
						OffersClaimed: []xdr.ClaimAtom{
							s.passiveSellOfferTrade,
							emptyTrade,
							emptyTrade,
						},
						Offer: xdr.ManageOfferSuccessResultOffer{
							Effect: xdr.ManageOfferEffectManageOfferDeleted,
						},
					},
				},
			},
		},
		{
			Tr: &xdr.OperationResultTr{
				Type: xdr.OperationTypeCreatePassiveSellOffer,
				CreatePassiveSellOfferResult: &xdr.ManageSellOfferResult{
					Code: xdr.ManageSellOfferResultCodeManageSellOfferSuccess,
					Success: &xdr.ManageOfferSuccessResult{
						OffersClaimed: []xdr.ClaimAtom{
							s.otherPassiveSellOfferTrade,
						},
						Offer: xdr.ManageOfferSuccessResultOffer{
							Effect: xdr.ManageOfferEffectManageOfferDeleted,
						},
					},
				},
			},
		},
		{
			Tr: &xdr.OperationResultTr{
				Type: xdr.OperationTypePathPaymentStrictReceive,
				PathPaymentStrictReceiveResult: &xdr.PathPaymentStrictReceiveResult{
					Code: xdr.PathPaymentStrictReceiveResultCodePathPaymentStrictReceiveSuccess,
					Success: &xdr.PathPaymentStrictReceiveResultSuccess{
						Offers: []xdr.ClaimAtom{
							emptyTrade,
							s.strictReceiveTradeLP,
						},
					},
				},
			},
		},
		{
			Tr: &xdr.OperationResultTr{
				Type: xdr.OperationTypePathPaymentStrictSend,
				PathPaymentStrictSendResult: &xdr.PathPaymentStrictSendResult{
					Code: xdr.PathPaymentStrictSendResultCodePathPaymentStrictSendSuccess,
					Success: &xdr.PathPaymentStrictSendResultSuccess{
						Offers: []xdr.ClaimAtom{
							s.strictSendTradeLP,
						},
					},
				},
			},
		},
	}

	operations := []xdr.Operation{
		{
			Body: xdr.OperationBody{
				Type:           xdr.OperationTypeBumpSequence,
				BumpSequenceOp: &xdr.BumpSequenceOp{BumpTo: 30000},
			},
		},
		{
			Body: xdr.OperationBody{
				Type:                       xdr.OperationTypePathPaymentStrictReceive,
				PathPaymentStrictReceiveOp: &xdr.PathPaymentStrictReceiveOp{},
			},
			SourceAccount: &s.opSourceAccount,
		},
		{
			Body: xdr.OperationBody{
				Type:                    xdr.OperationTypePathPaymentStrictSend,
				PathPaymentStrictSendOp: &xdr.PathPaymentStrictSendOp{},
			},
			SourceAccount: &s.opSourceAccount,
		},
		{
			Body: xdr.OperationBody{
				Type:             xdr.OperationTypeManageBuyOffer,
				ManageBuyOfferOp: &xdr.ManageBuyOfferOp{},
			},
			SourceAccount: &s.opSourceAccount,
		},
		{
			Body: xdr.OperationBody{
				Type:              xdr.OperationTypeManageSellOffer,
				ManageSellOfferOp: &xdr.ManageSellOfferOp{},
			},
			SourceAccount: &s.opSourceAccount,
		},
		{
			Body: xdr.OperationBody{
				Type:                     xdr.OperationTypeCreatePassiveSellOffer,
				CreatePassiveSellOfferOp: &xdr.CreatePassiveSellOfferOp{},
			},
		},
		{
			Body: xdr.OperationBody{
				Type:                     xdr.OperationTypeCreatePassiveSellOffer,
				CreatePassiveSellOfferOp: &xdr.CreatePassiveSellOfferOp{},
			},
			SourceAccount: &s.opSourceAccount,
		},
		{
			Body: xdr.OperationBody{
				Type:                       xdr.OperationTypePathPaymentStrictReceive,
				PathPaymentStrictReceiveOp: &xdr.PathPaymentStrictReceiveOp{},
			},
			SourceAccount: &s.opSourceAccount,
		},
		{
			Body: xdr.OperationBody{
				Type:                    xdr.OperationTypePathPaymentStrictSend,
				PathPaymentStrictSendOp: &xdr.PathPaymentStrictSendOp{},
			},
			SourceAccount: &s.opSourceAccount,
		},
	}

	tx := ingest.LedgerTransaction{
		Result: xdr.TransactionResultPair{
			Result: xdr.TransactionResult{
				Result: xdr.TransactionResultResult{
					Code:    xdr.TransactionResultCodeTxSuccess,
					Results: &operationResults,
				},
			},
		},
		Envelope: xdr.TransactionEnvelope{
			Type: xdr.EnvelopeTypeEnvelopeTypeTx,
			V1: &xdr.TransactionV1Envelope{
				Tx: xdr.Transaction{
					SourceAccount: s.sourceAccount,
					Operations:    operations,
				},
			},
		},
		Index:      1,
		FeeChanges: []xdr.LedgerEntryChange{},
		UnsafeMeta: xdr.TransactionMeta{
			V: 2,
			V2: &xdr.TransactionMetaV2{
				Operations: []xdr.OperationMeta{
					{
						Changes: xdr.LedgerEntryChanges{},
					},
				},
			},
		},
	}

	for i, trade := range s.allTrades {
		var changes xdr.LedgerEntryChanges
		if trade.Type == xdr.ClaimAtomTypeClaimAtomTypeOrderBook {
			changes = xdr.LedgerEntryChanges{
				xdr.LedgerEntryChange{
					Type: xdr.LedgerEntryChangeTypeLedgerEntryState,
					State: &xdr.LedgerEntry{
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeOffer,
							Offer: &xdr.OfferEntry{
								Price:    s.sellPrices[i],
								SellerId: trade.SellerId(),
								OfferId:  trade.OfferId(),
							},
						},
					},
				},
				xdr.LedgerEntryChange{
					Type: xdr.LedgerEntryChangeTypeLedgerEntryRemoved,
					Removed: &xdr.LedgerKey{
						Type: xdr.LedgerEntryTypeOffer,
						Offer: &xdr.LedgerKeyOffer{
							SellerId: trade.SellerId(),
							OfferId:  trade.OfferId(),
						},
					},
				},
			}
		} else {
			changes = xdr.LedgerEntryChanges{
				xdr.LedgerEntryChange{
					Type: xdr.LedgerEntryChangeTypeLedgerEntryState,
					State: &xdr.LedgerEntry{
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeLiquidityPool,
							LiquidityPool: &xdr.LiquidityPoolEntry{
								LiquidityPoolId: trade.MustLiquidityPool().LiquidityPoolId,
								Body: xdr.LiquidityPoolEntryBody{
									Type: xdr.LiquidityPoolTypeLiquidityPoolConstantProduct,
									ConstantProduct: &xdr.LiquidityPoolEntryConstantProduct{
										Params: xdr.LiquidityPoolConstantProductParameters{
											AssetA: trade.AssetBought(),
											AssetB: trade.AssetSold(),
											Fee:    xdr.LiquidityPoolFeeV18,
										},
										ReserveA:                 400,
										ReserveB:                 800,
										TotalPoolShares:          40,
										PoolSharesTrustLineCount: 50,
									},
								},
							},
						},
					},
				},
				xdr.LedgerEntryChange{
					Type: xdr.LedgerEntryChangeTypeLedgerEntryUpdated,
					Updated: &xdr.LedgerEntry{
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeLiquidityPool,
							LiquidityPool: &xdr.LiquidityPoolEntry{
								LiquidityPoolId: trade.MustLiquidityPool().LiquidityPoolId,
								Body: xdr.LiquidityPoolEntryBody{
									Type: xdr.LiquidityPoolTypeLiquidityPoolConstantProduct,
									ConstantProduct: &xdr.LiquidityPoolEntryConstantProduct{
										Params: xdr.LiquidityPoolConstantProductParameters{
											AssetA: trade.AssetBought(),
											AssetB: trade.AssetSold(),
											Fee:    xdr.LiquidityPoolFeeV18,
										},
										ReserveA:                 400,
										ReserveB:                 800,
										TotalPoolShares:          40,
										PoolSharesTrustLineCount: 50,
									},
								},
							},
						},
					},
				},
			}
		}
		tx.UnsafeMeta.V2.Operations = append(tx.UnsafeMeta.V2.Operations, xdr.OperationMeta{
			Changes: changes,
		})
	}

	s.txs = []ingest.LedgerTransaction{
		tx,
	}

	return inserts
}

func (s *TradeProcessorTestSuiteLedger) stubLoaders() {
	for key, id := range s.unmuxedAccountToID {
		s.accountLoader.Insert(key, id)
	}
	for key, id := range s.assetToID {
		s.assetLoader.Insert(key, id.ID)
	}
	for key, id := range s.lpToID {
		s.lpLoader.Insert(PoolIDToString(key), id)
	}
}

func (s *TradeProcessorTestSuiteLedger) TestIngestTradesSucceeds() {
	ctx := context.Background()
	inserts := s.mockReadTradeTransactions()

	for _, tx := range s.txs {
		err := s.processor.ProcessTransaction(s.lcm, tx)
		s.Assert().NoError(err)
	}

	for _, insert := range inserts {
		s.mockBatchInsertBuilder.On("Add", []history.InsertTrade{
			insert,
		}).Return(nil).Once()
	}
	s.mockBatchInsertBuilder.On("Exec", ctx, s.mockSession).Return(nil).Once()
	s.stubLoaders()

	err := s.processor.Flush(ctx, s.mockSession)
	s.Assert().NoError(err)
	s.Assert().Equal(s.processor.GetStats().count, int64(8))
	s.processor.ResetStats()
	s.Assert().Equal(s.processor.GetStats().count, int64(0))
}

func (s *TradeProcessorTestSuiteLedger) TestBatchAddError() {
	ctx := context.Background()
	s.mockReadTradeTransactions()

	for _, tx := range s.txs {
		err := s.processor.ProcessTransaction(s.lcm, tx)
		s.Assert().NoError(err)
	}

	s.stubLoaders()
	s.mockBatchInsertBuilder.On("Add", mock.AnythingOfType("[]history.InsertTrade")).
		Return(fmt.Errorf("batch add error")).Once()

	err := s.processor.Flush(ctx, s.mockSession)
	s.Assert().EqualError(err, "Error adding trade to batch: batch add error")
}

func (s *TradeProcessorTestSuiteLedger) TestBatchExecError() {
	ctx := context.Background()
	insert := s.mockReadTradeTransactions()

	for _, tx := range s.txs {
		err := s.processor.ProcessTransaction(s.lcm, tx)
		s.Assert().NoError(err)
	}

	s.stubLoaders()
	s.mockBatchInsertBuilder.On("Add", mock.AnythingOfType("[]history.InsertTrade")).
		Return(nil).Times(len(insert))
	s.mockBatchInsertBuilder.On("Exec", ctx, s.mockSession).Return(fmt.Errorf("exec error")).Once()

	err := s.processor.Flush(ctx, s.mockSession)
	s.Assert().EqualError(err, "Error flushing operation batch: exec error")
}

func TestTradeProcessor_ProcessTransaction_MuxedAccount(t *testing.T) {
	unmuxed := xdr.MustAddress("GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2")
	muxed := xdr.MuxedAccount{
		Type: xdr.CryptoKeyTypeKeyTypeMuxedEd25519,
		Med25519: &xdr.MuxedAccountMed25519{
			Id:      0xdeadbeefdeadbeef,
			Ed25519: *unmuxed.Ed25519,
		},
	}
	tx := createTransaction(true, 1, 2)
	tx.Index = 1
	tx.Envelope.Operations()[0].Body = xdr.OperationBody{
		Type: xdr.OperationTypePayment,
		PaymentOp: &xdr.PaymentOp{
			Destination: muxed,
			Asset:       xdr.Asset{Type: xdr.AssetTypeAssetTypeNative},
			Amount:      100,
		},
	}
}

func TestTradeProcessor_RoundingSlippage_Big(t *testing.T) {
	s := &TradeProcessorTestSuiteLedger{}
	s.SetT(t)
	s.SetupTest()
	s.mockReadTradeTransactions()

	assetDeposited := xdr.MustNewCreditAsset("MAD", s.unmuxedSourceAccount.Address())
	assetDisbursed := xdr.MustNewCreditAsset("GRE", s.unmuxedSourceAccount.Address())
	poolId, err := xdr.NewPoolId(assetDisbursed, assetDeposited, xdr.LiquidityPoolFeeV18)
	s.Assert().NoError(err)
	trade := xdr.ClaimAtom{
		Type: xdr.ClaimAtomTypeClaimAtomTypeLiquidityPool,
		LiquidityPool: &xdr.ClaimLiquidityAtom{
			LiquidityPoolId: poolId,
			AssetBought:     assetDeposited,
			AmountBought:    1,
			AssetSold:       assetDisbursed,
			AmountSold:      1,
		},
	}
	tx, err := createTransactionForTrade(trade, 3740000000, 162020000000)
	s.Assert().NoError(err)
	opIdx := 0
	change, err := s.processor.liquidityPoolChange(tx, opIdx, trade)
	s.Assert().NoError(err)

	result, err := s.processor.roundingSlippage(tx, opIdx, trade, change)
	s.Assert().NoError(err)
	s.Assert().True(result.Valid)
	s.Assert().Equal(null.IntFrom(4229), result)
}

func TestTradeProcessor_RoundingSlippage_Small(t *testing.T) {
	s := &TradeProcessorTestSuiteLedger{}
	s.SetT(t)
	s.SetupTest()
	s.mockReadTradeTransactions()

	assetDeposited := xdr.MustNewCreditAsset("MAD", s.unmuxedSourceAccount.Address())
	assetDisbursed := xdr.MustNewCreditAsset("GRE", s.unmuxedSourceAccount.Address())
	poolId, err := xdr.NewPoolId(assetDisbursed, assetDeposited, xdr.LiquidityPoolFeeV18)
	s.Assert().NoError(err)
	trade := xdr.ClaimAtom{
		Type: xdr.ClaimAtomTypeClaimAtomTypeLiquidityPool,
		LiquidityPool: &xdr.ClaimLiquidityAtom{
			LiquidityPoolId: poolId,
			AssetBought:     assetDeposited,
			AmountBought:    11,
			AssetSold:       assetDisbursed,
			AmountSold:      20,
		},
	}
	tx, err := createTransactionForTrade(trade, 200, 400)
	s.Assert().NoError(err)
	opIdx := 0
	change, err := s.processor.liquidityPoolChange(tx, opIdx, trade)
	s.Assert().NoError(err)

	result, err := s.processor.roundingSlippage(tx, opIdx, trade, change)
	s.Assert().NoError(err)
	s.Assert().True(result.Valid)
	s.Assert().Equal(null.IntFrom(4), result)
}

// TODO: Use a builder or something here to simplify.
func createTransactionForTrade(trade xdr.ClaimAtom, reserveA, reserveB int64) (ingest.LedgerTransaction, error) {
	source := xdr.MustMuxedAddress("GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY")
	destination := source

	pool := makePool(trade.AssetBought(), trade.AssetSold(), reserveA, reserveB)

	poolLedgerEntry := func(reserveA, reserveB xdr.Int64) *xdr.LedgerEntry {
		return &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type:          xdr.LedgerEntryTypeLiquidityPool,
				LiquidityPool: &pool,
			},
		}
	}

	return ingest.LedgerTransaction{
		Result: xdr.TransactionResultPair{
			TransactionHash: xdr.Hash{},
			Result: xdr.TransactionResult{
				Result: xdr.TransactionResultResult{
					Code:            xdr.TransactionResultCodeTxSuccess,
					InnerResultPair: &xdr.InnerTransactionResultPair{},
					Results:         &[]xdr.OperationResult{},
				},
			},
		},
		Envelope: xdr.TransactionEnvelope{
			Type: xdr.EnvelopeTypeEnvelopeTypeTx,
			V1: &xdr.TransactionV1Envelope{
				Tx: xdr.Transaction{
					SourceAccount: source,
					Operations: []xdr.Operation{
						{
							Body: xdr.OperationBody{
								Type: xdr.OperationTypePathPaymentStrictReceive,
								PathPaymentStrictReceiveOp: &xdr.PathPaymentStrictReceiveOp{
									SendAsset:   trade.AssetBought(),
									SendMax:     trade.AmountBought(),
									Destination: destination,
									DestAsset:   trade.AssetSold(),
									DestAmount:  trade.AmountSold(),
									Path: []xdr.Asset{
										trade.AssetBought(),
										trade.AssetSold(),
									},
								},
							},
						},
					},
				},
			},
		},
		UnsafeMeta: xdr.TransactionMeta{
			V: 2,
			V2: &xdr.TransactionMetaV2{
				Operations: []xdr.OperationMeta{
					{
						Changes: xdr.LedgerEntryChanges{
							{
								Type: xdr.LedgerEntryChangeTypeLedgerEntryState,
								State: poolLedgerEntry(
									xdr.Int64(reserveA),
									xdr.Int64(reserveB),
								),
							},
							{
								Type: xdr.LedgerEntryChangeTypeLedgerEntryUpdated,
								Updated: poolLedgerEntry(
									xdr.Int64(reserveA)+trade.AmountBought(),
									xdr.Int64(reserveB)-trade.AmountSold(),
								),
							},
						},
					},
				},
			},
		},
	}, nil
}

func makePool(A, B xdr.Asset, a, b int64) xdr.LiquidityPoolEntry {
	if !A.LessThan(B) {
		B, A = A, B
		b, a = a, b
	}

	poolId, _ := xdr.NewPoolId(A, B, xdr.LiquidityPoolFeeV18)
	return xdr.LiquidityPoolEntry{
		LiquidityPoolId: poolId,
		Body: xdr.LiquidityPoolEntryBody{
			Type: xdr.LiquidityPoolTypeLiquidityPoolConstantProduct,
			ConstantProduct: &xdr.LiquidityPoolEntryConstantProduct{
				Params: xdr.LiquidityPoolConstantProductParameters{
					AssetA: A,
					AssetB: B,
					Fee:    xdr.LiquidityPoolFeeV18,
				},
				ReserveA:                 xdr.Int64(a),
				ReserveB:                 xdr.Int64(b),
				TotalPoolShares:          123,
				PoolSharesTrustLineCount: 456,
			},
		},
	}
}
