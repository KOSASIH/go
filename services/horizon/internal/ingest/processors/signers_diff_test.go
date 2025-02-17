package processors

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/xdr"
)

func TestAccountSignersDiff(t *testing.T) {
	sponsor, err := xdr.AddressToAccountId("GBADGWKHSUFOC4C7E3KXKINZSRX5KPHUWHH67UGJU77LEORGVLQ3BN3B")
	assert.NoError(t, err)
	newSponsor, err := xdr.AddressToAccountId("GB2Y6D5QFDJSCR6GSBO5D2LOLGZI4RVPRGZSSPLIFWNJZ7SL73TOMXAQ")
	assert.NoError(t, err)

	for _, testCase := range []struct {
		name              string
		input             ingest.Change
		removed           []string
		signersAdded      map[string]int32
		sponsorsPerSigner map[string]string
	}{
		{
			"account added without master weight",
			ingest.Change{
				Type: xdr.LedgerEntryTypeAccount,
				Pre:  nil,
				Post: &xdr.LedgerEntry{
					LastModifiedLedgerSeq: 10,
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
						},
					},
				},
			},
			[]string{},
			map[string]int32{},
			map[string]string{},
		},
		{
			"account removed with master weight",
			ingest.Change{
				Type: xdr.LedgerEntryTypeAccount,
				Pre: &xdr.LedgerEntry{
					LastModifiedLedgerSeq: 10,
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							// Master weight = 1
							Thresholds: [4]byte{1, 1, 1, 1},
						},
					},
				},
				Post: nil,
			},
			[]string{"GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"},
			nil,
			nil,
		},
		{
			"account removed without master key",
			ingest.Change{
				Type: xdr.LedgerEntryTypeAccount,
				Pre: &xdr.LedgerEntry{
					LastModifiedLedgerSeq: 10,
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							// Master weight = 0
							Thresholds: [4]byte{0, 1, 1, 1},
						},
					},
				},
				Post: nil,
			},
			[]string{},
			nil,
			nil,
		},
		{
			"master key removed",
			ingest.Change{
				Type: xdr.LedgerEntryTypeAccount,
				Pre: &xdr.LedgerEntry{
					LastModifiedLedgerSeq: 10,
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							// Master weight = 1
							Thresholds: [4]byte{1, 1, 1, 1},
						},
					},
				},
				Post: &xdr.LedgerEntry{
					LastModifiedLedgerSeq: 10,
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							// Master weight = 0
							Thresholds: [4]byte{0, 1, 1, 1},
						},
					},
				},
			},
			[]string{"GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"},
			map[string]int32{},
			map[string]string{},
		},
		{
			"master key added",
			ingest.Change{
				Type: xdr.LedgerEntryTypeAccount,
				Pre: &xdr.LedgerEntry{
					LastModifiedLedgerSeq: 10,
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							// Master weight = 0
							Thresholds: [4]byte{0, 1, 1, 1},
						},
					},
				},
				Post: &xdr.LedgerEntry{
					LastModifiedLedgerSeq: 10,
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							// Master weight = 1
							Thresholds: [4]byte{1, 1, 1, 1},
						},
					},
				},
			},
			[]string{},
			map[string]int32{
				"GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML": 1,
			},
			map[string]string{},
		},
		{
			"signer added",
			ingest.Change{
				Type: xdr.LedgerEntryTypeAccount,
				Pre: &xdr.LedgerEntry{
					LastModifiedLedgerSeq: 10,
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							Signers:   []xdr.Signer{},
						},
					},
				},
				Post: &xdr.LedgerEntry{
					LastModifiedLedgerSeq: 10,
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							Signers: []xdr.Signer{
								{
									Key:    xdr.MustSigner("GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX"),
									Weight: 1,
								},
							},
						},
					},
				},
			},
			[]string{},
			map[string]int32{
				"GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX": 1,
			},
			map[string]string{},
		},
		{
			"signer removed",
			ingest.Change{
				Type: xdr.LedgerEntryTypeAccount,
				Pre: &xdr.LedgerEntry{
					LastModifiedLedgerSeq: 10,
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							Signers: []xdr.Signer{
								{
									Key:    xdr.MustSigner("GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX"),
									Weight: 1,
								},
							},
						},
					},
				},
				Post: &xdr.LedgerEntry{
					LastModifiedLedgerSeq: 10,
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							Signers:   []xdr.Signer{},
						},
					},
				},
			},
			[]string{"GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX"},
			map[string]int32{},
			map[string]string{},
		},
		{
			"signer weight changed",
			ingest.Change{
				Type: xdr.LedgerEntryTypeAccount,
				Pre: &xdr.LedgerEntry{
					LastModifiedLedgerSeq: 10,
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							Signers: []xdr.Signer{
								{
									Key:    xdr.MustSigner("GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX"),
									Weight: 1,
								},
							},
						},
					},
				},
				Post: &xdr.LedgerEntry{
					LastModifiedLedgerSeq: 10,
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							Signers: []xdr.Signer{
								{
									Key:    xdr.MustSigner("GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX"),
									Weight: 2,
								},
							},
						},
					},
				},
			},
			[]string{"GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX"},
			map[string]int32{
				"GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX": 2,
			},
			map[string]string{},
		},
		{
			"sponsor added",
			ingest.Change{
				Type: xdr.LedgerEntryTypeAccount,
				Pre: &xdr.LedgerEntry{
					LastModifiedLedgerSeq: 10,
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							Signers: []xdr.Signer{
								{
									Key:    xdr.MustSigner("GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX"),
									Weight: 1,
								},
							},
						},
					},
				},
				Post: &xdr.LedgerEntry{
					LastModifiedLedgerSeq: 10,
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							Signers: []xdr.Signer{
								{
									Key:    xdr.MustSigner("GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX"),
									Weight: 1,
								},
							},
							Ext: xdr.AccountEntryExt{
								V1: &xdr.AccountEntryExtensionV1{
									Ext: xdr.AccountEntryExtensionV1Ext{
										V2: &xdr.AccountEntryExtensionV2{
											SignerSponsoringIDs: []xdr.SponsorshipDescriptor{
												&sponsor,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			[]string{"GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX"},
			map[string]int32{
				"GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX": 1,
			},
			map[string]string{
				"GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX": sponsor.Address(),
			},
		},
		{
			"sponsor removed",
			ingest.Change{
				Type: xdr.LedgerEntryTypeAccount,
				Pre: &xdr.LedgerEntry{
					LastModifiedLedgerSeq: 10,
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							Signers: []xdr.Signer{
								{
									Key:    xdr.MustSigner("GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX"),
									Weight: 1,
								},
							},
							Ext: xdr.AccountEntryExt{
								V1: &xdr.AccountEntryExtensionV1{
									Ext: xdr.AccountEntryExtensionV1Ext{
										V2: &xdr.AccountEntryExtensionV2{
											SignerSponsoringIDs: []xdr.SponsorshipDescriptor{
												&sponsor,
											},
										},
									},
								},
							},
						},
					},
				},
				Post: &xdr.LedgerEntry{
					LastModifiedLedgerSeq: 10,
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							Signers: []xdr.Signer{
								{
									Key:    xdr.MustSigner("GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX"),
									Weight: 1,
								},
							},
						},
					},
				},
			},
			[]string{"GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX"},
			map[string]int32{"GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX": 1},
			map[string]string{},
		},
		{
			"sponsor updated",
			ingest.Change{
				Type: xdr.LedgerEntryTypeAccount,
				Pre: &xdr.LedgerEntry{
					LastModifiedLedgerSeq: 10,
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							Signers: []xdr.Signer{
								{
									Key:    xdr.MustSigner("GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX"),
									Weight: 1,
								},
							},
							Ext: xdr.AccountEntryExt{
								V1: &xdr.AccountEntryExtensionV1{
									Ext: xdr.AccountEntryExtensionV1Ext{
										V2: &xdr.AccountEntryExtensionV2{
											SignerSponsoringIDs: []xdr.SponsorshipDescriptor{
												&sponsor,
											},
										},
									},
								},
							},
						},
					},
				},
				Post: &xdr.LedgerEntry{
					LastModifiedLedgerSeq: 10,
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							Signers: []xdr.Signer{
								{
									Key:    xdr.MustSigner("GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX"),
									Weight: 1,
								},
							},
							Ext: xdr.AccountEntryExt{
								V1: &xdr.AccountEntryExtensionV1{
									Ext: xdr.AccountEntryExtensionV1Ext{
										V2: &xdr.AccountEntryExtensionV2{
											SignerSponsoringIDs: []xdr.SponsorshipDescriptor{
												&newSponsor,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			[]string{"GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX"},
			map[string]int32{
				"GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX": 1,
			},
			map[string]string{
				"GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX": newSponsor.Address(),
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			removed, signersAdded, sponsorsPerSigner := accountSignersDiff(testCase.input)
			assert.ElementsMatch(t, testCase.removed, removed)
			assert.Equal(t, testCase.signersAdded, signersAdded)
			assert.Equal(t, testCase.sponsorsPerSigner, sponsorsPerSigner)
		})
	}
}
