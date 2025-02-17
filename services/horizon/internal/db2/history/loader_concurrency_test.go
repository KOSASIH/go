package history

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/services/horizon/internal/test"
)

func TestLoaderConcurrentInserts(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	s1 := tt.HorizonSession()
	s2 := s1.Clone()

	for _, testCase := range []struct {
		mode ConcurrencyMode
		pass bool
	}{
		{ConcurrentInserts, true},
		{ConcurrentDeletes, false},
	} {
		t.Run(fmt.Sprintf("%v", testCase.mode), func(t *testing.T) {
			var addresses []string
			for i := 0; i < 10; i++ {
				addresses = append(addresses, keypair.MustRandom().Address())
			}

			l1 := NewAccountLoader(testCase.mode)
			for _, address := range addresses {
				l1.GetFuture(address)
			}

			for i := 0; i < 5; i++ {
				addresses = append(addresses, keypair.MustRandom().Address())
			}

			l2 := NewAccountLoader(testCase.mode)
			for _, address := range addresses {
				l2.GetFuture(address)
			}

			assert.NoError(t, s1.Begin(context.Background()))
			assert.NoError(t, l1.Exec(context.Background(), s1))

			assert.NoError(t, s2.Begin(context.Background()))
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-time.After(time.Second * 3)
				assert.NoError(t, s1.Commit())
			}()
			// l2.Exec(context.Background(), s2) will block until s1
			// is committed because s1 and s2 both attempt to insert common
			// accounts and, since s1 executed first, s2 must wait until
			// s1 terminates.
			assert.NoError(t, l2.Exec(context.Background(), s2))
			assert.NoError(t, s2.Commit())
			wg.Wait()

			assert.Equal(t, LoaderStats{
				Total:    10,
				Inserted: 10,
			}, l1.Stats())

			if testCase.pass {
				assert.Equal(t, LoaderStats{
					Total:    15,
					Inserted: 5,
				}, l2.Stats())
			} else {
				assert.Equal(t, LoaderStats{
					Total:    5,
					Inserted: 5,
				}, l2.Stats())
				return
			}

			q := &Q{s1}
			for _, address := range addresses[:10] {
				l1Id, err := l1.GetNow(address)
				assert.NoError(t, err)

				l2Id, err := l2.GetNow(address)
				assert.NoError(t, err)
				assert.Equal(t, l1Id, l2Id)

				var account Account
				assert.NoError(t, q.AccountByAddress(context.Background(), &account, address))
				assert.Equal(t, account.ID, l1Id)
				assert.Equal(t, account.Address, address)
			}

			for _, address := range addresses[10:] {
				l2Id, err := l2.GetNow(address)
				assert.NoError(t, err)

				_, err = l1.GetNow(address)
				assert.ErrorContains(t, err, "was not found")

				var account Account
				assert.NoError(t, q.AccountByAddress(context.Background(), &account, address))
				assert.Equal(t, account.ID, l2Id)
				assert.Equal(t, account.Address, address)
			}
		})
	}
}

func TestLoaderConcurrentDeletes(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	s1 := tt.HorizonSession()
	s2 := s1.Clone()

	for _, testCase := range []struct {
		mode ConcurrencyMode
		pass bool
	}{
		{ConcurrentInserts, false},
		{ConcurrentDeletes, true},
	} {
		t.Run(fmt.Sprintf("%v", testCase.mode), func(t *testing.T) {
			var addresses []string
			for i := 0; i < 10; i++ {
				addresses = append(addresses, keypair.MustRandom().Address())
			}

			loader := NewAccountLoader(testCase.mode)
			for _, address := range addresses {
				loader.GetFuture(address)
			}
			assert.NoError(t, loader.Exec(context.Background(), s1))

			var ids []int64
			for _, address := range addresses {
				id, err := loader.GetNow(address)
				assert.NoError(t, err)
				ids = append(ids, id)
			}

			loader = NewAccountLoader(testCase.mode)
			for _, address := range addresses {
				loader.GetFuture(address)
			}

			assert.NoError(t, s1.Begin(context.Background()))
			assert.NoError(t, loader.Exec(context.Background(), s1))

			assert.NoError(t, s2.Begin(context.Background()))
			q2 := &Q{s2}

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-time.After(time.Second * 3)

				q1 := &Q{s1}
				for _, address := range addresses {
					id, err := loader.GetNow(address)
					assert.NoError(t, err)

					var account Account
					err = q1.AccountByAddress(context.Background(), &account, address)
					if testCase.pass {
						assert.NoError(t, err)
						assert.Equal(t, account.ID, id)
						assert.Equal(t, account.Address, address)
					} else {
						assert.ErrorContains(t, err, sql.ErrNoRows.Error())
					}
				}
				assert.NoError(t, s1.Commit())
			}()

			// the reaper should block until s1 has been committed because s1 has locked
			// the orphaned rows
			deletedCount, err := q2.reapLookupTable(context.Background(), "history_accounts", ids, 1000)
			assert.NoError(t, err)
			assert.Equal(t, int64(len(addresses)), deletedCount)
			assert.NoError(t, s2.Commit())

			wg.Wait()
		})
	}
}
