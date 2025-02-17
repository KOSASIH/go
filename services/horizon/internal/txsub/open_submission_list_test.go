//lint:file-ignore U1001 Ignore all unused code, staticcheck doesn't understand testify/suite

package txsub

import (
	"context"
	"testing"
	"time"

	"github.com/stellar/go/services/horizon/internal/test"
	"github.com/stellar/go/support/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type SubmissionListTestSuite struct {
	suite.Suite
	list      OpenSubmissionList
	realList  *submissionList
	listeners []chan Result
	hashes    []string
	ctx       context.Context
}

func (suite *SubmissionListTestSuite) SetupTest() {
	suite.list = NewDefaultSubmissionList()
	suite.realList = suite.list.(*submissionList)
	suite.hashes = []string{
		"0000000000000000000000000000000000000000000000000000000000000000",
		"0000000000000000000000000000000000000000000000000000000000000001",
	}
	suite.listeners = []chan Result{
		make(chan Result, 1),
		make(chan Result, 1),
	}
	suite.ctx = test.Context()
}

func (suite *SubmissionListTestSuite) TestSubmissionList_Add() {
	// adds an entry to the submission list when a new hash is used
	suite.list.Add(suite.hashes[0], suite.listeners[0])
	sub := suite.realList.submissions[suite.hashes[0]]
	assert.Equal(suite.T(), suite.hashes[0], sub.Hash)
	assert.WithinDuration(suite.T(), sub.SubmittedAt, time.Now(), 1*time.Second)

	// drop the send side of the channel by casting to listener
	var l Listener = suite.listeners[0]
	assert.Equal(suite.T(), l, sub.Listeners[0])

}

func (suite *SubmissionListTestSuite) TestSubmissionList_AddListener() {
	// adds a listener to an existing entry when a hash is used with a new listener
	suite.list.Add(suite.hashes[0], suite.listeners[0])
	sub := suite.realList.submissions[suite.hashes[0]]
	st := sub.SubmittedAt
	<-time.After(20 * time.Millisecond)
	suite.list.Add(suite.hashes[0], suite.listeners[1])

	// increases the size of the listener
	assert.Equal(suite.T(), 2, len(sub.Listeners))
	// doesn't update the submitted at time
	assert.Equal(suite.T(), true, st == sub.SubmittedAt)

	// Panics when the listener is not buffered
	// panics when the listener is not buffered
	assert.Panics(suite.T(), func() {
		suite.list.Add(suite.hashes[0], make(Listener))
	})
}

func (suite *SubmissionListTestSuite) TestSubmissionList_Finish() {

	suite.list.Add(suite.hashes[0], suite.listeners[0])
	suite.list.Add(suite.hashes[0], suite.listeners[1])
	r := Result{Err: errors.New("test error")}
	suite.list.Finish(suite.hashes[0], r)

	// Wries to every listener
	r1, ok1 := <-suite.listeners[0]

	assert.Equal(suite.T(), r, r1)
	assert.True(suite.T(), ok1)

	r2, ok2 := <-suite.listeners[1]
	assert.Equal(suite.T(), r, r2)
	assert.True(suite.T(), ok2)

	// Removes the entry
	_, ok := suite.realList.submissions[suite.hashes[0]]
	assert.False(suite.T(), ok)

	// Closes every ledger
	_, _ = <-suite.listeners[0]
	_, more := <-suite.listeners[0]
	assert.False(suite.T(), more)

	_, _ = <-suite.listeners[1]
	_, more = <-suite.listeners[1]
	assert.False(suite.T(), more)
}

func (suite *SubmissionListTestSuite) TestSubmissionList_Clean() {

	suite.list.Add(suite.hashes[0], suite.listeners[0])
	<-time.After(200 * time.Millisecond)
	suite.list.Add(suite.hashes[1], suite.listeners[1])
	left := suite.list.Clean(200 * time.Millisecond)

	assert.Equal(suite.T(), 1, left)

	// removes submissions older than the maxAge provided
	_, ok := suite.realList.submissions[suite.hashes[0]]
	assert.False(suite.T(), ok)

	// leaves submissions that are younger than the maxAge provided
	_, ok = suite.realList.submissions[suite.hashes[1]]
	assert.True(suite.T(), ok)

	// closes any cleaned listeners
	assert.Equal(suite.T(), 1, len(suite.listeners[0]))
	<-suite.listeners[0]
	select {
	case _, stillOpen := <-suite.listeners[0]:
		assert.False(suite.T(), stillOpen)
	default:
		panic("cleaned listener is still open")
	}
}

// Tests that Pending works as expected
func (suite *SubmissionListTestSuite) TestSubmissionList_Pending() {
	assert.Equal(suite.T(), 0, len(suite.list.Pending()))
	suite.list.Add(suite.hashes[0], suite.listeners[0])
	assert.Equal(suite.T(), 1, len(suite.list.Pending()))
	suite.list.Add(suite.hashes[1], suite.listeners[1])
	assert.Equal(suite.T(), 2, len(suite.list.Pending()))
}

func TestSubmissionListTestSuite(t *testing.T) {
	suite.Run(t, new(SubmissionListTestSuite))
}
