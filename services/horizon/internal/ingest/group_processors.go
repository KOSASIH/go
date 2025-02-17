package ingest

import (
	"context"
	"time"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/ingest/processors"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

type runDurations map[string]time.Duration

func (d runDurations) AddRunDuration(name string, startTime time.Time) {
	d[name] += time.Since(startTime)
}

type groupChangeProcessors struct {
	processors             []horizonChangeProcessor
	processorsRunDurations runDurations
}

func newGroupChangeProcessors(processors []horizonChangeProcessor) *groupChangeProcessors {
	return &groupChangeProcessors{
		processors:             processors,
		processorsRunDurations: make(map[string]time.Duration),
	}
}

func (g groupChangeProcessors) Name() string {
	return "groupChangeProcessors"
}

func (g groupChangeProcessors) ProcessChange(ctx context.Context, change ingest.Change) error {
	for _, p := range g.processors {
		startTime := time.Now()
		if err := p.ProcessChange(ctx, change); err != nil {
			return errors.Wrapf(err, "error in %T.ProcessChange", p)
		}
		g.processorsRunDurations.AddRunDuration(p.Name(), startTime)
	}
	return nil
}

func (g groupChangeProcessors) Commit(ctx context.Context) error {
	for _, p := range g.processors {
		startTime := time.Now()
		if err := p.Commit(ctx); err != nil {
			return errors.Wrapf(err, "error in %T.Commit", p)
		}
		g.processorsRunDurations.AddRunDuration(p.Name(), startTime)
	}
	return nil
}

type groupLoaders struct {
	lazyLoaders  []horizonLazyLoader
	runDurations runDurations
	stats        map[string]history.LoaderStats
}

func newGroupLoaders(lazyLoaders []horizonLazyLoader) groupLoaders {
	return groupLoaders{
		lazyLoaders:  lazyLoaders,
		runDurations: make(map[string]time.Duration),
		stats:        make(map[string]history.LoaderStats),
	}
}

func (g groupLoaders) Flush(ctx context.Context, session db.SessionInterface, execInTx bool) error {
	if execInTx {
		if err := session.Begin(ctx); err != nil {
			return err
		}
		defer session.Rollback()
	}

	for _, loader := range g.lazyLoaders {
		startTime := time.Now()
		if err := loader.Exec(ctx, session); err != nil {
			return errors.Wrapf(err, "error during lazy loader resolution, %T.Exec", loader)
		}
		name := loader.Name()
		g.runDurations.AddRunDuration(name, startTime)
		g.stats[name] = loader.Stats()
	}

	if execInTx {
		if err := session.Commit(); err != nil {
			return err
		}
	}
	return nil
}

type groupTransactionProcessors struct {
	processors                []horizonTransactionProcessor
	processorsRunDurations    runDurations
	transactionStatsProcessor *processors.StatsLedgerTransactionProcessor
	tradeProcessor            *processors.TradeProcessor
}

// build the group processor for all tx processors
// processors - list of processors this should include StatsLedgerTransactionProcessor and TradeProcessor
// transactionStatsProcessor - provide a direct reference to the stats processor that is in processors or nil,
//
//	group processing will reset stats as needed
//
// tradeProcessor - provide a direct reference to the trades processor in processors or nil,
//
//	so group processing will reset stats as needed
func newGroupTransactionProcessors(processors []horizonTransactionProcessor,
	transactionStatsProcessor *processors.StatsLedgerTransactionProcessor,
	tradeProcessor *processors.TradeProcessor,
) *groupTransactionProcessors {

	return &groupTransactionProcessors{
		processors:                processors,
		processorsRunDurations:    make(map[string]time.Duration),
		transactionStatsProcessor: transactionStatsProcessor,
		tradeProcessor:            tradeProcessor,
	}
}

func (g groupTransactionProcessors) IsEmpty() bool {
	return len(g.processors) == 0
}

func (g groupTransactionProcessors) ProcessTransaction(lcm xdr.LedgerCloseMeta, tx ingest.LedgerTransaction) error {
	for _, p := range g.processors {
		startTime := time.Now()
		if err := p.ProcessTransaction(lcm, tx); err != nil {
			return errors.Wrapf(err, "error in %T.ProcessTransaction", p)
		}
		g.processorsRunDurations.AddRunDuration(p.Name(), startTime)
	}
	return nil
}

func (g groupTransactionProcessors) Flush(ctx context.Context, session db.SessionInterface) error {
	for _, p := range g.processors {
		startTime := time.Now()
		if err := p.Flush(ctx, session); err != nil {
			return errors.Wrapf(err, "error in %T.Flush", p)
		}
		g.processorsRunDurations.AddRunDuration(p.Name(), startTime)
	}
	return nil
}

func (g *groupTransactionProcessors) ResetStats() {
	g.processorsRunDurations = make(map[string]time.Duration)
	if g.tradeProcessor != nil {
		g.tradeProcessor.ResetStats()
	}
	if g.transactionStatsProcessor != nil {
		g.transactionStatsProcessor.ResetStats()
	}
}

type groupTransactionFilterers struct {
	filterers []processors.LedgerTransactionFilterer
	runDurations
	droppedTransactions int64
}

func newGroupTransactionFilterers(filterers []processors.LedgerTransactionFilterer) *groupTransactionFilterers {
	return &groupTransactionFilterers{
		filterers:    filterers,
		runDurations: make(map[string]time.Duration),
	}
}

func (g *groupTransactionFilterers) Name() string {
	return "groupTransactionFilterers"
}

func (g *groupTransactionFilterers) FilterTransaction(ctx context.Context, tx ingest.LedgerTransaction) (bool, bool, error) {
	filtersEnabled := false

	for _, f := range g.filterers {
		startTime := time.Now()
		filterEnabled, include, err := f.FilterTransaction(ctx, tx)
		if !filterEnabled {
			continue
		}

		filtersEnabled = true
		if err != nil {
			return true, false, errors.Wrapf(err, "error in %T.FilterTransaction", f)
		}
		g.AddRunDuration(f.Name(), startTime)
		if include {
			return true, true, nil
		}
	}

	if filtersEnabled {
		g.droppedTransactions++
		return true, false, nil
	}
	return false, true, nil
}

func (g *groupTransactionFilterers) ResetStats() {
	g.droppedTransactions = 0
	g.runDurations = make(map[string]time.Duration)
}
