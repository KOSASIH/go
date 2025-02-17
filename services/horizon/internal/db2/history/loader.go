package history

import (
	"context"
	"database/sql/driver"
	"fmt"
	"sort"
	"strings"

	"github.com/lib/pq"

	"github.com/stellar/go/support/collections/set"
	"github.com/stellar/go/support/db"
)

var errSealed = fmt.Errorf("cannot register more entries to Loader after calling Exec()")

// ConcurrencyMode is used to configure the level of thread-safety for a loader
type ConcurrencyMode int

func (cm ConcurrencyMode) String() string {
	switch cm {
	case ConcurrentInserts:
		return "ConcurrentInserts"
	case ConcurrentDeletes:
		return "ConcurrentDeletes"
	default:
		return "unknown"
	}
}

const (
	_ ConcurrencyMode = iota
	// ConcurrentInserts configures the loader to maintain safety when there are multiple loaders
	// inserting into the same table concurrently. This ConcurrencyMode is suitable for parallel reingestion.
	// Note while ConcurrentInserts is enabled it is not safe to have deletes occurring concurrently on the
	// same table.
	ConcurrentInserts
	// ConcurrentDeletes configures the loader to maintain safety when there is another thread which is invoking
	// reapLookupTable() to delete rows from the same table concurrently. This ConcurrencyMode is suitable for
	// live ingestion when reaping of lookup tables is enabled.
	// Note while ConcurrentDeletes is enabled it is not safe to have multiple threads inserting concurrently to the
	// same table.
	ConcurrentDeletes
)

// LoaderStats describes the result of executing a history lookup id Loader
type LoaderStats struct {
	// Total is the number of elements registered to the Loader
	Total int
	// Inserted is the number of elements inserted into the lookup table
	Inserted int
}

type loader[K comparable, T any] struct {
	sealed          bool
	set             set.Set[K]
	ids             map[K]int64
	stats           LoaderStats
	name            string
	table           string
	columnsForKeys  func([]K) []columnValues
	mappingFromRow  func(T) (K, int64)
	less            func(K, K) bool
	concurrencyMode ConcurrencyMode
}

type future[K comparable, T any] struct {
	key    K
	loader *loader[K, T]
}

// Value implements the database/sql/driver Valuer interface.
func (f future[K, T]) Value() (driver.Value, error) {
	return f.loader.GetNow(f.key)
}

// GetFuture registers the given key into the Loader and
// returns a future which will hold the history id for
// the key after Exec() is called.
func (l *loader[K, T]) GetFuture(key K) future[K, T] {
	if l.sealed {
		panic(errSealed)
	}

	l.set.Add(key)
	return future[K, T]{
		key:    key,
		loader: l,
	}
}

// GetNow returns the history id for the given key.
// GetNow should only be called on values which were registered by
// GetFuture() calls. Also, Exec() must be called before any GetNow
// call can succeed.
func (l *loader[K, T]) GetNow(key K) (int64, error) {
	if !l.sealed {
		return 0, fmt.Errorf(`invalid loader state,  
		Exec was not called yet to properly seal and resolve %v id`, key)
	}
	if internalID, ok := l.ids[key]; !ok {
		return 0, fmt.Errorf(`loader key %v was not found`, key)
	} else {
		return internalID, nil
	}
}

// Exec will look up all the history ids for the keys registered in the Loader.
// If there are no history ids for a given set of keys, Exec will insert rows
// into the corresponding history table to establish a mapping between each key and its history id.
func (l *loader[K, T]) Exec(ctx context.Context, session db.SessionInterface) error {
	l.sealed = true
	if len(l.set) == 0 {
		return nil
	}
	q := &Q{session}
	keys := make([]K, 0, len(l.set))
	for key := range l.set {
		keys = append(keys, key)
	}
	// sort entries before inserting rows to prevent deadlocks on acquiring a ShareLock
	// https://github.com/stellar/go/issues/2370
	sort.Slice(keys, func(i, j int) bool {
		return l.less(keys[i], keys[j])
	})

	if l.concurrencyMode == ConcurrentInserts {
		// if there are other ingestion transactions running concurrently,
		// we need to first insert the records (with a ON CONFLICT DO NOTHING
		// clause). Then, we can query for the remaining records.
		// This order (insert first and then query) is important because
		// if multiple concurrent transactions try to insert the same record
		// only one of them will succeed and the other transactions will omit
		// the record from the RETURNING set.
		if count, err := l.insert(ctx, q, keys); err != nil {
			return err
		} else {
			l.stats.Total += count
			l.stats.Inserted += count
		}

		if count, err := l.query(ctx, q, keys, false); err != nil {
			return err
		} else {
			l.stats.Total += count
		}
	} else if l.concurrencyMode == ConcurrentDeletes {
		// if the lookup table reaping transaction is running concurrently,
		// we need to lock the rows from the lookup table to ensure that
		// the reaper cannot run until after the ingestion transaction has
		// been committed.
		if count, err := l.query(ctx, q, keys, true); err != nil {
			return err
		} else {
			l.stats.Total += count
		}

		// insert whatever records were not found from l.query()
		if count, err := l.insert(ctx, q, keys); err != nil {
			return err
		} else {
			l.stats.Total += count
			l.stats.Inserted += count
		}
	} else {
		return fmt.Errorf("concurrency mode %v is invalid", l.concurrencyMode)
	}

	return nil
}

// Stats returns the number of addresses registered in the Loader and the number of rows
// inserted into the history table.
func (l *loader[K, T]) Stats() LoaderStats {
	return l.stats
}

func (l *loader[K, T]) Name() string {
	return l.name
}

func (l *loader[K, T]) filter(keys []K) []K {
	if len(l.ids) == 0 {
		return keys
	}

	remaining := make([]K, 0, len(keys))
	for _, key := range keys {
		if _, ok := l.ids[key]; ok {
			continue
		}
		remaining = append(remaining, key)
	}
	return remaining
}

func (l *loader[K, T]) updateMap(rows []T) {
	for _, row := range rows {
		key, id := l.mappingFromRow(row)
		l.ids[key] = id
	}
}

func (l *loader[K, T]) insert(ctx context.Context, q *Q, keys []K) (int, error) {
	keys = l.filter(keys)
	if len(keys) == 0 {
		return 0, nil
	}

	var rows []T
	err := bulkInsert(
		ctx,
		q,
		l.table,
		l.columnsForKeys(keys),
		&rows,
	)
	if err != nil {
		return 0, err
	}

	l.updateMap(rows)
	return len(rows), nil
}

func (l *loader[K, T]) query(ctx context.Context, q *Q, keys []K, lockRows bool) (int, error) {
	keys = l.filter(keys)
	if len(keys) == 0 {
		return 0, nil
	}
	var suffix string
	if lockRows {
		suffix = "ORDER BY id ASC FOR KEY SHARE"
	}

	var rows []T
	err := bulkGet(
		ctx,
		q,
		l.table,
		l.columnsForKeys(keys),
		&rows,
		suffix,
	)
	if err != nil {
		return 0, err
	}

	l.updateMap(rows)
	return len(rows), nil
}

type columnValues struct {
	name    string
	dbType  string
	objects []string
}

func bulkInsert(ctx context.Context, q *Q, table string, fields []columnValues, response interface{}) error {
	unnestPart := make([]string, 0, len(fields))
	insertFieldsPart := make([]string, 0, len(fields))
	pqArrays := make([]interface{}, 0, len(fields))

	// In the code below we are building the bulk insert query which looks like:
	//
	// WITH rows AS
	//		(SELECT
	//			/* unnestPart */
	//			unnest(?::type1[]), /* field1 */
	//			unnest(?::type2[]), /* field2 */
	//			...
	//		)
	//	INSERT INTO table (
	//		/* insertFieldsPart */
	//		field1,
	//		field2,
	//		...
	//	)
	//	SELECT * FROM rows ON CONFLICT (field1, field2, ...) DO NOTHING RETURNING *
	//
	// Using unnest allows to get around the maximum limit of 65,535 query parameters,
	// see https://www.postgresql.org/docs/12/limits.html and
	// https://klotzandrew.com/blog/postgres-passing-65535-parameter-limit/
	//
	// Without using unnest we would have to use multiple insert statements to insert
	// all the rows for large datasets.
	for _, field := range fields {
		unnestPart = append(
			unnestPart,
			fmt.Sprintf("unnest(?::%s[]) /* %s */", field.dbType, field.name),
		)
		insertFieldsPart = append(
			insertFieldsPart,
			field.name,
		)
		pqArrays = append(
			pqArrays,
			pq.Array(field.objects),
		)
	}
	columns := strings.Join(insertFieldsPart, ",")

	sql := `
	WITH rows AS
		(SELECT ` + strings.Join(unnestPart, ",") + `)
	INSERT INTO ` + table + `
		(` + columns + `)
	SELECT * FROM rows
	ON CONFLICT (` + columns + `) DO NOTHING
	RETURNING *`

	return q.SelectRaw(
		ctx,
		response,
		sql,
		pqArrays...,
	)
}

func bulkGet(ctx context.Context, q *Q, table string, fields []columnValues, response interface{}, suffix string) error {
	unnestPart := make([]string, 0, len(fields))
	columns := make([]string, 0, len(fields))
	pqArrays := make([]interface{}, 0, len(fields))

	// In the code below we are building the bulk get query which looks like:
	//
	//	SELECT * FROM table WHERE (field1, field2, ...) IN
	//		(SELECT
	//			/* unnestPart */
	//			unnest(?::type1[]), /* field1 */
	//			unnest(?::type2[]), /* field2 */
	//			...
	//		)
	//
	// Using unnest allows to get around the maximum limit of 65,535 query parameters,
	// see https://www.postgresql.org/docs/12/limits.html and
	// https://klotzandrew.com/blog/postgres-passing-65535-parameter-limit/
	//
	// Without using unnest we would have to use multiple select statements to obtain
	// all the rows for large datasets.
	for _, field := range fields {
		unnestPart = append(
			unnestPart,
			fmt.Sprintf("unnest(?::%s[]) /* %s */", field.dbType, field.name),
		)
		columns = append(
			columns,
			field.name,
		)
		pqArrays = append(
			pqArrays,
			pq.Array(field.objects),
		)
	}
	sql := `SELECT * FROM ` + table + ` WHERE (` + strings.Join(columns, ",") + `) IN 
	(SELECT ` + strings.Join(unnestPart, ",") + `) ` + suffix

	return q.SelectRaw(
		ctx,
		response,
		sql,
		pqArrays...,
	)
}
