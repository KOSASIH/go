package history

import (
	"context"
)

func (i *accountSignersBatchInsertBuilder) Add(signer AccountSigner) error {
	return i.builder.Row(map[string]interface{}{
		"account_id": signer.Account,
		"signer":     signer.Signer,
		"weight":     signer.Weight,
		"sponsor":    signer.Sponsor,
	})
}

func (i *accountSignersBatchInsertBuilder) Exec(ctx context.Context) error {
	return i.builder.Exec(ctx, i.session, i.table)
}

func (i *accountSignersBatchInsertBuilder) Len() int {
	return i.builder.Len()
}
