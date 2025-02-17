// Package ledger provides useful utilities concerning ledgers within stellar,
// specifically as a central location to store a cached snapshot of the state of
// both horizon's and stellar-core's views of the ledger.  This package is
// intended to be at the lowest levels of horizon's dependency tree, please keep
// it free of dependencies to other horizon packages.
package ledger

import (
	"github.com/prometheus/client_golang/prometheus"
	"sync"
	"time"
)

// Status represents a snapshot of both horizon's and stellar-core's view of the
// ledger.
type Status struct {
	CoreStatus
	HorizonStatus
}

type CoreStatus struct {
	CoreLatest int32 `db:"core_latest"`
}

type HorizonStatus struct {
	HistoryLatest         int32     `db:"history_latest"`
	HistoryLatestClosedAt time.Time `db:"history_latest_closed_at"`
	HistoryElder          int32     `db:"history_elder"`
	ExpHistoryLatest      uint32    `db:"exp_history_latest"`
}

// State is an in-memory data structure which holds a snapshot of both
// horizon's and stellar-core's view of the network
type State struct {
	sync.RWMutex
	current Status

	Metrics struct {
		HistoryLatestLedgerCounter        prometheus.CounterFunc
		HistoryLatestLedgerClosedAgoGauge prometheus.GaugeFunc
		HistoryElderLedgerCounter         prometheus.CounterFunc
		CoreLatestLedgerCounter           prometheus.CounterFunc
	}
}

type StateInterface interface {
	CurrentStatus() Status
	SetStatus(next Status)
	SetCoreStatus(next CoreStatus)
	SetHorizonStatus(next HorizonStatus)
	RegisterMetrics(registry *prometheus.Registry)
}

// CurrentStatus returns the cached snapshot of ledger state
func (c *State) CurrentStatus() Status {
	c.RLock()
	defer c.RUnlock()
	ret := c.current
	return ret
}

// SetStatus updates the cached snapshot of the ledger state
func (c *State) SetStatus(next Status) {
	c.Lock()
	defer c.Unlock()
	c.current = next
}

// SetCoreStatus updates the cached snapshot of the ledger state of Stellar-Core
func (c *State) SetCoreStatus(next CoreStatus) {
	c.Lock()
	defer c.Unlock()
	c.current.CoreStatus = next
}

// SetHorizonStatus updates the cached snapshot of the ledger state of Horizon
func (c *State) SetHorizonStatus(next HorizonStatus) {
	c.Lock()
	defer c.Unlock()
	c.current.HorizonStatus = next
}

func (c *State) RegisterMetrics(registry *prometheus.Registry) {
	c.Metrics.HistoryLatestLedgerCounter = prometheus.NewCounterFunc(
		prometheus.CounterOpts{Namespace: "horizon", Subsystem: "history", Name: "latest_ledger"},
		func() float64 {
			ls := c.CurrentStatus()
			return float64(ls.HistoryLatest)
		},
	)
	registry.MustRegister(c.Metrics.HistoryLatestLedgerCounter)

	c.Metrics.HistoryLatestLedgerClosedAgoGauge = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: "horizon", Subsystem: "history", Name: "latest_ledger_closed_ago_seconds",
			Help: "seconds since the close of the last ingested ledger",
		},
		func() float64 {
			ls := c.CurrentStatus()
			return time.Since(ls.HistoryLatestClosedAt).Seconds()
		},
	)
	registry.MustRegister(c.Metrics.HistoryLatestLedgerClosedAgoGauge)

	c.Metrics.HistoryElderLedgerCounter = prometheus.NewCounterFunc(
		prometheus.CounterOpts{Namespace: "horizon", Subsystem: "history", Name: "elder_ledger"},
		func() float64 {
			ls := c.CurrentStatus()
			return float64(ls.HistoryElder)
		},
	)
	registry.MustRegister(c.Metrics.HistoryElderLedgerCounter)

	c.Metrics.CoreLatestLedgerCounter = prometheus.NewCounterFunc(
		prometheus.CounterOpts{Namespace: "horizon", Subsystem: "stellar_core", Name: "latest_ledger"},
		func() float64 {
			ls := c.CurrentStatus()
			return float64(ls.CoreLatest)
		},
	)
	registry.MustRegister(c.Metrics.CoreLatestLedgerCounter)
}
