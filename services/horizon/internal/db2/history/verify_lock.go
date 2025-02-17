package history

import (
	"context"

	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
)

const (
	// stateVerificationLockId is the objid for the advisory lock acquired during
	// state verification. The value is arbitrary. The only requirement is that
	// all ingesting nodes use the same value which is why it's hard coded here.`1
	stateVerificationLockId = 73897213
	// reaperLockId is the objid for the advisory lock acquired during
	// reaping of history tables. The value is arbitrary. The only requirement is that
	// all ingesting nodes use the same value which is why it's hard coded here.
	reaperLockId = 944670730
	// lookupTableReaperLockId is the objid for the advisory lock acquired during
	// reaping of lookup tables. The value is arbitrary. The only requirement is that
	// all ingesting nodes use the same value which is why it's hard coded here.
	lookupTableReaperLockId = 329518896
)

// TryStateVerificationLock attempts to acquire the state verification lock
// which gives the ingesting node exclusive access to perform state verification.
// TryStateVerificationLock returns true if the lock was acquired or false if the
// lock could not be acquired because it is held by another node.
func (q *Q) TryStateVerificationLock(ctx context.Context) (bool, error) {
	return q.tryAdvisoryLock(ctx, stateVerificationLockId)
}

// TryReaperLock attempts to acquire the reaper lock
// which gives the ingesting node exclusive access to perform reaping.
// TryReaperLock returns true if the lock was acquired or false if the
// lock could not be acquired because it is held by another node.
func (q *Q) TryReaperLock(ctx context.Context) (bool, error) {
	return q.tryAdvisoryLock(ctx, reaperLockId)
}

func (q *Q) TryLookupTableReaperLock(ctx context.Context) (bool, error) {
	return q.tryAdvisoryLock(ctx, lookupTableReaperLockId)
}

func (q *Q) tryAdvisoryLock(ctx context.Context, lockId int) (bool, error) {
	if tx := q.GetTx(); tx == nil {
		return false, errors.New("cannot be called outside of a transaction")
	}

	var acquired []bool
	err := q.SelectRaw(
		context.WithValue(ctx, &db.QueryTypeContextKey, db.AdvisoryLockQueryType),
		&acquired,
		"SELECT pg_try_advisory_xact_lock(?)",
		lockId,
	)
	if err != nil {
		return false, errors.Wrap(err, "error acquiring advisory lock for state verification")
	}
	if len(acquired) != 1 {
		return false, errors.New("invalid response from advisory lock")
	}
	return acquired[0], nil
}
