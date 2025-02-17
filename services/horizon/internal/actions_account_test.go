package horizon

import (
	"testing"

	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/ingest"
	"github.com/stellar/go/xdr"
)

func TestAccountActions_InvalidID(t *testing.T) {
	ht := StartHTTPTestWithoutScenario(t)
	defer ht.Finish()

	// Makes StateMiddleware happy
	q := history.Q{ht.HorizonSession()}
	err := q.UpdateLastLedgerIngest(ht.Ctx, 100)
	ht.Assert.NoError(err)
	err = q.UpdateIngestVersion(ht.Ctx, ingest.CurrentVersion)
	ht.Assert.NoError(err)

	ht.Assert.NoError(q.Begin(ht.Ctx))
	ledgerBatch := q.NewLedgerBatchInsertBuilder()
	err = ledgerBatch.Add(xdr.LedgerHeaderHistoryEntry{
		Header: xdr.LedgerHeader{
			LedgerSeq: 100,
		},
	}, 0, 0, 0, 0, 0)
	ht.Assert.NoError(err)
	ht.Assert.NoError(ledgerBatch.Exec(ht.Ctx, q))
	ht.Assert.NoError(q.Commit())

	// existing account
	w := ht.Get(
		"/accounts/=cr%FF%98%CB%F3%AF%E72%D85%FE%28%15y%8Fz%C4Ng%CE%98h%02%2A:%B6%FF%B9%CF%92%88O%91%10d&S%7C%9Bi%D4%CFI%28%CFo",
	)
	ht.Assert.Equal(400, w.Code)
}
