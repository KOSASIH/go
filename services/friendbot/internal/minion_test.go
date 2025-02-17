package internal

import (
	"sync"
	"testing"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	hProtocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/txnbuild"
	"github.com/stretchr/testify/assert"
)

// This test aims to reproduce the issue found on https://github.com/stellar/go/issues/2271
// in which Minion.Run() will try to send multiple messages to a channel that gets closed
// immediately after receiving one message.
func TestMinion_NoChannelErrors(t *testing.T) {
	mockSubmitTransaction := func(minion *Minion, hclient horizonclient.ClientInterface, tx string) (txn *hProtocol.Transaction, err error) {
		return txn, nil
	}

	mockCheckSequenceRefresh := func(minion *Minion, hclient horizonclient.ClientInterface) (err error) {
		return errors.New("could not refresh sequence")
	}

	mockCheckAccountExists := func(minion *Minion, hclient horizonclient.ClientInterface, destAddress string) (bool, string, error) {
		return false, "0", nil
	}

	// Public key: GD25B4QI6KWVDWXDW25CIM7EKR6A6PBSWE2RCNSAC4NJQDQJXZJYMMKR
	botSeed := "SCWNLYELENPBXN46FHYXETT5LJCYBZD5VUQQVW4KZPHFO2YTQJUWT4D5"
	botKeypair, err := keypair.Parse(botSeed)
	if !assert.NoError(t, err) {
		return
	}
	botAccount := Account{AccountID: botKeypair.Address()}

	// Public key: GD4AGPPDFFHKK3Z2X4XZDRXX6GZQKP4FMLVQ5T55NDEYGG3GIP7BQUHM
	minionSeed := "SDTNSEERJPJFUE2LSDNYBFHYGVTPIWY7TU2IOJZQQGLWO2THTGB7NU5A"
	minionKeypair, err := keypair.Parse(minionSeed)
	if !assert.NoError(t, err) {
		return
	}

	minion := Minion{
		Account: Account{
			AccountID: minionKeypair.Address(),
			Sequence:  1,
		},
		Keypair:              minionKeypair.(*keypair.Full),
		BotAccount:           botAccount,
		BotKeypair:           botKeypair.(*keypair.Full),
		Network:              "Test SDF Network ; September 2015",
		StartingBalance:      "10000.00",
		SubmitTransaction:    mockSubmitTransaction,
		CheckSequenceRefresh: mockCheckSequenceRefresh,
		CheckAccountExists:   mockCheckAccountExists,
		BaseFee:              txnbuild.MinBaseFee,
	}
	fb := &Bot{Minions: []Minion{minion}}

	recipientAddress := "GDJIN6W6PLTPKLLM57UW65ZH4BITUXUMYQHIMAZFYXF45PZVAWDBI77Z"

	// Prior to the bug fix, the following should consistently trigger a panic
	// (send on closed channel)
	numTests := 1000
	var wg sync.WaitGroup
	wg.Add(numTests)

	for i := 0; i < numTests; i++ {
		go func() {
			fb.Pay(recipientAddress)
			wg.Done()
		}()
	}
	wg.Wait()
}

func TestMinion_CorrectNumberOfTxSubmissions(t *testing.T) {
	var (
		numTxSubmits int
		mux          sync.Mutex
	)

	mockSubmitTransaction := func(minion *Minion, hclient horizonclient.ClientInterface, tx string) (txn *hProtocol.Transaction, err error) {
		mux.Lock()
		numTxSubmits++
		mux.Unlock()
		return txn, nil
	}

	mockCheckSequenceRefresh := func(minion *Minion, hclient horizonclient.ClientInterface) (err error) {
		return nil
	}

	mockCheckAccountExists := func(minion *Minion, hclient horizonclient.ClientInterface, destAddress string) (bool, string, error) {
		return false, "0", nil
	}

	// Public key: GD25B4QI6KWVDWXDW25CIM7EKR6A6PBSWE2RCNSAC4NJQDQJXZJYMMKR
	botSeed := "SCWNLYELENPBXN46FHYXETT5LJCYBZD5VUQQVW4KZPHFO2YTQJUWT4D5"
	botKeypair, err := keypair.Parse(botSeed)
	if !assert.NoError(t, err) {
		return
	}
	botAccount := Account{AccountID: botKeypair.Address()}

	// Public key: GD4AGPPDFFHKK3Z2X4XZDRXX6GZQKP4FMLVQ5T55NDEYGG3GIP7BQUHM
	minionSeed := "SDTNSEERJPJFUE2LSDNYBFHYGVTPIWY7TU2IOJZQQGLWO2THTGB7NU5A"
	minionKeypair, err := keypair.Parse(minionSeed)
	if !assert.NoError(t, err) {
		return
	}

	minion := Minion{
		Account: Account{
			AccountID: minionKeypair.Address(),
			Sequence:  1,
		},
		Keypair:              minionKeypair.(*keypair.Full),
		BotAccount:           botAccount,
		BotKeypair:           botKeypair.(*keypair.Full),
		Network:              "Test SDF Network ; September 2015",
		StartingBalance:      "10000.00",
		SubmitTransaction:    mockSubmitTransaction,
		CheckSequenceRefresh: mockCheckSequenceRefresh,
		CheckAccountExists:   mockCheckAccountExists,
		BaseFee:              txnbuild.MinBaseFee,
	}
	fb := &Bot{Minions: []Minion{minion}}

	recipientAddress := "GDJIN6W6PLTPKLLM57UW65ZH4BITUXUMYQHIMAZFYXF45PZVAWDBI77Z"

	numTests := 1000
	var wg sync.WaitGroup
	wg.Add(numTests)

	for i := 0; i < numTests; i++ {
		go func() {
			fb.Pay(recipientAddress)
			wg.Done()
		}()
	}
	wg.Wait()
	assert.Equal(t, numTests, numTxSubmits)
}
