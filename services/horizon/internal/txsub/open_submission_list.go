package txsub

import (
	"fmt"
	"sync"
	"time"

	"github.com/stellar/go/support/log"
)

// NewDefaultSubmissionList returns a list that manages open submissions purely
// in memory.
func NewDefaultSubmissionList() OpenSubmissionList {
	return &submissionList{
		submissions: map[string]*openSubmission{},
		log:         log.DefaultLogger.WithField("service", "txsub.submissionList"),
	}
}

// openSubmission tracks a slice of channels that should be emitted to when we
// know the result for the transactions with the provided hash
type openSubmission struct {
	Hash        string
	SubmittedAt time.Time
	Listeners   []Listener
}

type submissionList struct {
	sync.Mutex
	submissions map[string]*openSubmission // hash => `*openSubmission`
	log         *log.Entry
}

func (s *submissionList) Add(hash string, l Listener) {
	s.Lock()
	defer s.Unlock()

	if cap(l) == 0 {
		panic("Unbuffered listener cannot be added to OpenSubmissionList")
	}

	os, ok := s.submissions[hash]

	if !ok {
		os = &openSubmission{
			Hash:        hash,
			SubmittedAt: time.Now(),
			Listeners:   []Listener{},
		}
		s.submissions[hash] = os
		s.log.WithField("hash", hash).Info("Created a new submission for a transaction")
	} else {
		s.log.WithField("hash", hash).Info("Adding listener to existing submission")
	}

	os.Listeners = append(os.Listeners, l)
}

func (s *submissionList) Finish(hash string, r Result) {
	s.Lock()
	defer s.Unlock()

	os, ok := s.submissions[hash]
	if !ok {
		return
	}

	s.log.WithFields(log.F{
		"hash":      hash,
		"listeners": len(os.Listeners),
		"result":    fmt.Sprintf("%+v", r),
	}).Info("Sending submission result to listeners")

	for _, l := range os.Listeners {
		l <- r
		close(l)
	}

	delete(s.submissions, hash)
}

func (s *submissionList) Clean(maxAge time.Duration) int {
	s.Lock()
	defer s.Unlock()

	for _, os := range s.submissions {
		if time.Since(os.SubmittedAt) > maxAge {
			s.log.WithFields(log.F{
				"hash":      os.Hash,
				"listeners": len(os.Listeners),
			}).Warn("Cleared submission due to timeout")
			r := Result{Err: ErrTimeout}
			delete(s.submissions, os.Hash)
			for _, l := range os.Listeners {
				l <- r
				close(l)
			}
		}
	}

	return len(s.submissions)
}

func (s *submissionList) Pending() []string {
	s.Lock()
	defer s.Unlock()
	results := make([]string, 0, len(s.submissions))

	for hash := range s.submissions {
		results = append(results, hash)
	}

	return results
}
