// Copyright 2016 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

package historyarchive

import (
	"fmt"
	"strings"

	"github.com/stellar/go/support/ordered"
	"golang.org/x/exp/slices"
)

const DefaultCheckpointFrequency = uint32(64)

type Range struct {
	Low  uint32
	High uint32
}

type CheckpointManager struct {
	checkpointFreq uint32
}

// NewCheckpointManager creates a CheckpointManager based on a checkpoint frequency
// (the number of ledgers between ledger checkpoints). If checkpointFrequency is
// 0 DefaultCheckpointFrequency will be used.
func NewCheckpointManager(checkpointFrequency uint32) CheckpointManager {
	if checkpointFrequency == 0 {
		checkpointFrequency = DefaultCheckpointFrequency
	}
	return CheckpointManager{checkpointFrequency}
}

func (c CheckpointManager) GetCheckpointFrequency() uint32 {
	return c.checkpointFreq
}

func (c CheckpointManager) IsCheckpoint(i uint32) bool {
	return (i+1)%c.checkpointFreq == 0
}

// PrevCheckpoint returns the checkpoint ledger preceding `i`.
func (c CheckpointManager) PrevCheckpoint(i uint32) uint32 {
	freq := c.checkpointFreq
	if i < freq {
		return freq - 1
	}
	return (((i + 1) / freq) * freq) - 1
}

// NextCheckpoint returns the checkpoint ledger following `i`.
func (c CheckpointManager) NextCheckpoint(i uint32) uint32 {
	if i == 0 {
		return c.checkpointFreq - 1
	}
	freq := uint64(c.checkpointFreq)
	v := uint64(i)
	n := (((v + freq) / freq) * freq) - 1

	return uint32(ordered.Min(n, 0xffffffff))
}

// GetCheckPoint gets the checkpoint containing information about the given ledger sequence
func (c CheckpointManager) GetCheckpoint(i uint32) uint32 {
	return c.NextCheckpoint(i)
}

// GetCheckpointRange gets the range of the checkpoint containing information for the given ledger sequence
func (c CheckpointManager) GetCheckpointRange(i uint32) Range {
	checkpoint := c.GetCheckpoint(i)
	low := checkpoint - c.checkpointFreq + 1
	if low == 0 {
		// ledger 0 does not exist
		low++
	}
	return Range{
		Low:  low,
		High: checkpoint,
	}
}

func (c CheckpointManager) MakeRange(low uint32, high uint32) Range {
	high = ordered.Max(high, low)
	return Range{
		Low:  c.PrevCheckpoint(low),
		High: c.NextCheckpoint(high),
	}
}

func (r Range) clamp(other Range, cManager CheckpointManager) Range {
	low := ordered.Max(r.Low, other.Low)
	high := ordered.Min(r.High, other.High)
	return cManager.MakeRange(low, high)
}

func (r Range) String() string {
	return fmt.Sprintf("[0x%8.8x, 0x%8.8x]", r.Low, r.High)
}

func (r Range) GenerateCheckpoints(cManager CheckpointManager) chan uint32 {
	ch := make(chan uint32)
	go func() {
		for i := uint64(r.Low); i <= uint64(r.High); i += uint64(cManager.checkpointFreq) {
			ch <- uint32(i)
		}
		close(ch)
	}()
	return ch
}

func (r Range) SizeInCheckPoints(cManager CheckpointManager) int {
	return 1 + (int(r.High-r.Low) / int(cManager.checkpointFreq))
}

func (r Range) collapsedString() string {
	if r.Low == r.High {
		return fmt.Sprintf("0x%8.8x", r.Low)
	}

	return fmt.Sprintf("[0x%8.8x-0x%8.8x]", r.Low, r.High)
}

func (r Range) InRange(sequence uint32) bool {
	return sequence >= r.Low && sequence <= r.High
}

func (r Range) Size() uint32 {
	return 1 + (r.High - r.Low)
}

func fmtRangeList(vs []uint32, cManager CheckpointManager) string {
	slices.Sort(vs)

	s := make([]string, 0, 10)
	var curr *Range

	for _, t := range vs {
		if curr != nil {
			if curr.High+cManager.checkpointFreq == t {
				curr.High = t
				continue
			} else {
				s = append(s, curr.collapsedString())
				curr = nil
			}
		}
		curr = &Range{Low: t, High: t}
	}
	if curr != nil {
		s = append(s, curr.collapsedString())
	}

	return strings.Join(s, ", ")
}
