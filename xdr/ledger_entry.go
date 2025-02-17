package xdr

import "fmt"

// LedgerKey implements the `Keyer` interface
func (entry *LedgerEntry) LedgerKey() (LedgerKey, error) {
	return entry.Data.LedgerKey()
}

// SponsoringID return SponsorshipDescriptor for a given ledger entry
func (entry *LedgerEntry) SponsoringID() SponsorshipDescriptor {
	var sponsor SponsorshipDescriptor
	if entry.Ext.V == 1 && entry.Ext.V1 != nil {
		sponsor = entry.Ext.V1.SponsoringId
	}
	return sponsor
}

// Normalize overwrites LedgerEntry with all the extensions set to default values
// (if extension is not present).
// This is helpful to compare two ledger entries that are the same but for one of
// them extensions are not set.
// Returns the same entry.
func (entry *LedgerEntry) Normalize() *LedgerEntry {
	// If ledgerEntry is V0, create ext=1 and add a nil sponsor
	if entry.Ext.V == 0 {
		entry.Ext = LedgerEntryExt{
			V: 1,
			V1: &LedgerEntryExtensionV1{
				SponsoringId: nil,
			},
		}
	}

	switch entry.Data.Type {
	case LedgerEntryTypeAccount:
		accountEntry := entry.Data.Account
		// Account can have ext=0. For those, create ext=1 with 0 liabilities.
		if accountEntry.Ext.V == 0 {
			accountEntry.Ext.V = 1
			accountEntry.Ext.V1 = &AccountEntryExtensionV1{
				Liabilities: Liabilities{
					Buying:  0,
					Selling: 0,
				},
			}
		}
		// if AccountEntryExtensionV1Ext is v=0, then create v2 with 0 values
		if accountEntry.Ext.V1.Ext.V == 0 {
			accountEntry.Ext.V1.Ext.V = 2
			accountEntry.Ext.V1.Ext.V2 = &AccountEntryExtensionV2{
				NumSponsored:        Uint32(0),
				NumSponsoring:       Uint32(0),
				SignerSponsoringIDs: make([]SponsorshipDescriptor, len(accountEntry.Signers)),
			}
		}
		// if AccountEntryExtensionV2Ext is v=0, then create v3 with 0 values
		if accountEntry.Ext.V1.Ext.V2.Ext.V == 0 {
			accountEntry.Ext.V1.Ext.V2.Ext.V = 3
			accountEntry.Ext.V1.Ext.V2.Ext.V3 = &AccountEntryExtensionV3{
				SeqLedger: Uint32(0),
				SeqTime:   TimePoint(0),
			}
		}

		signerSponsoringIDs := accountEntry.Ext.V1.Ext.V2.SignerSponsoringIDs

		// Map sponsors (signer => SponsorshipDescriptor)
		sponsorsMap := make(map[string]SponsorshipDescriptor)
		for i, signer := range accountEntry.Signers {
			sponsorsMap[signer.Key.Address()] = signerSponsoringIDs[i]
		}

		// Sort signers
		accountEntry.Signers = SortSignersByKey(accountEntry.Signers)

		// Recreate sponsors for sorted signers
		signerSponsoringIDs = make([]SponsorshipDescriptor, len(accountEntry.Signers))
		for i, signer := range accountEntry.Signers {
			signerSponsoringIDs[i] = sponsorsMap[signer.Key.Address()]
		}

		accountEntry.Ext.V1.Ext.V2.SignerSponsoringIDs = signerSponsoringIDs
	case LedgerEntryTypeTrustline:
		// Trust line can have ext=0. For those, create ext=1
		// with 0 liabilities.
		trustLineEntry := entry.Data.TrustLine
		if trustLineEntry.Ext.V == 0 {
			trustLineEntry.Ext.V = 1
			trustLineEntry.Ext.V1 = &TrustLineEntryV1{
				Liabilities: Liabilities{
					Buying:  0,
					Selling: 0,
				},
			}
		} else if trustLineEntry.Ext.V == 1 {
			// horizon doesn't make use of TrustLineEntryExtensionV2.liquidityPoolUseCount
			// so clear out those fields to make state verifier pass
			trustLineEntry.Ext.V1.Ext.V = 0
			trustLineEntry.Ext.V1.Ext.V2 = nil
		}
	case LedgerEntryTypeClaimableBalance:
		claimableBalanceEntry := entry.Data.ClaimableBalance
		claimableBalanceEntry.Claimants = SortClaimantsByDestination(claimableBalanceEntry.Claimants)

		if claimableBalanceEntry.Ext.V == 0 {
			claimableBalanceEntry.Ext.V = 1
			claimableBalanceEntry.Ext.V1 = &ClaimableBalanceEntryExtensionV1{
				Flags: 0,
			}
		}
	}

	return entry
}

func (data *LedgerEntryData) SetContractData(entry *ContractDataEntry) error {
	*data = LedgerEntryData{
		Type:         LedgerEntryTypeContractData,
		ContractData: entry,
	}
	return nil
}

func (data *LedgerEntryData) SetContractCode(entry *ContractCodeEntry) error {
	*data = LedgerEntryData{
		Type:         LedgerEntryTypeContractCode,
		ContractCode: entry,
	}
	return nil
}

// LedgerKey implements the `Keyer` interface
func (data *LedgerEntryData) LedgerKey() (LedgerKey, error) {
	var key LedgerKey
	switch data.Type {
	case LedgerEntryTypeAccount:
		if err := key.SetAccount(data.Account.AccountId); err != nil {
			return key, err
		}
	case LedgerEntryTypeTrustline:
		if err := key.SetTrustline(data.TrustLine.AccountId, data.TrustLine.Asset); err != nil {
			return key, err
		}
	case LedgerEntryTypeContractData:
		if err := key.SetContractData(
			data.ContractData.Contract,
			data.ContractData.Key,
			data.ContractData.Durability); err != nil {
			return key, err
		}
	case LedgerEntryTypeContractCode:
		if err := key.SetContractCode(data.ContractCode.Hash); err != nil {
			return key, err
		}
	case LedgerEntryTypeData:
		if err := key.SetData(data.Data.AccountId, string(data.Data.DataName)); err != nil {
			return key, err
		}
	case LedgerEntryTypeOffer:
		if err := key.SetOffer(data.Offer.SellerId, uint64(data.Offer.OfferId)); err != nil {
			return key, err
		}
	case LedgerEntryTypeLiquidityPool:
		if err := key.SetLiquidityPool(data.LiquidityPool.LiquidityPoolId); err != nil {
			return key, err
		}
	case LedgerEntryTypeClaimableBalance:
		if err := key.SetClaimableBalance(data.ClaimableBalance.BalanceId); err != nil {
			return key, err
		}
	case LedgerEntryTypeConfigSetting:
		if err := key.SetConfigSetting(data.ConfigSetting.ConfigSettingId); err != nil {
			return key, err
		}
	case LedgerEntryTypeTtl:
		if err := key.SetTtl(data.Ttl.KeyHash); err != nil {
			return key, err
		}
	default:
		return key, fmt.Errorf("unknown ledger entry type %d", data.Type)
	}

	return key, nil
}
