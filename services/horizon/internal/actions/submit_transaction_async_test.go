package actions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/stellar/go/protocols/horizon"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go/network"
	proto "github.com/stellar/go/protocols/stellarcore"
	"github.com/stellar/go/services/horizon/internal/corestate"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/support/render/problem"
)

const (
	TxXDR  = "AAAAAAGUcmKO5465JxTSLQOQljwk2SfqAJmZSG6JH6wtqpwhAAABLAAAAAAAAAABAAAAAAAAAAEAAAALaGVsbG8gd29ybGQAAAAAAwAAAAAAAAAAAAAAABbxCy3mLg3hiTqX4VUEEp60pFOrJNxYM1JtxXTwXhY2AAAAAAvrwgAAAAAAAAAAAQAAAAAW8Qst5i4N4Yk6l+FVBBKetKRTqyTcWDNSbcV08F4WNgAAAAAN4Lazj4x61AAAAAAAAAAFAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABLaqcIQAAAEBKwqWy3TaOxoGnfm9eUjfTRBvPf34dvDA0Nf+B8z4zBob90UXtuCqmQqwMCyH+okOI3c05br3khkH0yP4kCwcE"
	TxHash = "3389e9f0f1a65f19736cacf544c2e825313e8447f569233bb8db39aa607c8889"
)

type MockClientWithMetrics struct {
	mock.Mock
}

// SubmitTx mocks the SubmitTransaction method
func (m *MockClientWithMetrics) SubmitTx(ctx context.Context, rawTx string) (*proto.TXResponse, error) {
	args := m.Called(ctx, rawTx)
	return args.Get(0).(*proto.TXResponse), args.Error(1)
}

func createRequest() *http.Request {
	form := url.Values{}
	form.Set("tx", TxXDR)

	request, _ := http.NewRequest(
		"POST",
		"http://localhost:8000/transactions_async",
		strings.NewReader(form.Encode()),
	)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	return request
}

func TestAsyncSubmitTransactionHandler_DisabledTxSub(t *testing.T) {
	handler := AsyncSubmitTransactionHandler{
		DisableTxSub: true,
	}

	request := createRequest()
	w := httptest.NewRecorder()

	_, err := handler.GetResource(w, request)
	assert.NotNil(t, err)
	assert.IsType(t, &problem.P{}, err)
	p := err.(*problem.P)
	assert.Equal(t, "transaction_submission_disabled", p.Type)
	assert.Equal(t, http.StatusForbidden, p.Status)
}

func TestAsyncSubmitTransactionHandler_MalformedTransaction(t *testing.T) {
	handler := AsyncSubmitTransactionHandler{}

	request := createRequest()
	w := httptest.NewRecorder()

	_, err := handler.GetResource(w, request)
	assert.NotNil(t, err)
	assert.IsType(t, &problem.P{}, err)
	p := err.(*problem.P)
	assert.Equal(t, "transaction_malformed", p.Type)
	assert.Equal(t, http.StatusBadRequest, p.Status)
}

func TestAsyncSubmitTransactionHandler_CoreNotSynced(t *testing.T) {
	coreStateGetter := new(coreStateGetterMock)
	coreStateGetter.On("GetCoreState").Return(corestate.State{Synced: false})
	handler := AsyncSubmitTransactionHandler{
		CoreStateGetter:   coreStateGetter,
		NetworkPassphrase: network.PublicNetworkPassphrase,
	}

	request := createRequest()
	w := httptest.NewRecorder()

	_, err := handler.GetResource(w, request)
	assert.NotNil(t, err)
	assert.IsType(t, problem.P{}, err)
	p := err.(problem.P)
	assert.Equal(t, "stale_history", p.Type)
	assert.Equal(t, http.StatusServiceUnavailable, p.Status)
}

func TestAsyncSubmitTransactionHandler_TransactionSubmissionFailed(t *testing.T) {
	coreStateGetter := new(coreStateGetterMock)
	coreStateGetter.On("GetCoreState").Return(corestate.State{Synced: true})

	MockClientWithMetrics := &MockClientWithMetrics{}
	MockClientWithMetrics.On("SubmitTx", context.Background(), TxXDR).Return(&proto.TXResponse{}, errors.Errorf("submission error"))

	handler := AsyncSubmitTransactionHandler{
		CoreStateGetter:   coreStateGetter,
		NetworkPassphrase: network.PublicNetworkPassphrase,
		ClientWithMetrics: MockClientWithMetrics,
	}

	request := createRequest()
	w := httptest.NewRecorder()

	_, err := handler.GetResource(w, request)
	assert.NotNil(t, err)
	assert.IsType(t, &problem.P{}, err)
	p := err.(*problem.P)
	assert.Equal(t, "transaction_submission_failed", p.Type)
	assert.Equal(t, http.StatusInternalServerError, p.Status)
}

func TestAsyncSubmitTransactionHandler_TransactionSubmissionException(t *testing.T) {
	coreStateGetter := new(coreStateGetterMock)
	coreStateGetter.On("GetCoreState").Return(corestate.State{Synced: true})

	MockClientWithMetrics := &MockClientWithMetrics{}
	MockClientWithMetrics.On("SubmitTx", context.Background(), TxXDR).Return(&proto.TXResponse{
		Exception: "some-exception",
	}, nil)

	handler := AsyncSubmitTransactionHandler{
		CoreStateGetter:   coreStateGetter,
		NetworkPassphrase: network.PublicNetworkPassphrase,
		ClientWithMetrics: MockClientWithMetrics,
	}

	request := createRequest()
	w := httptest.NewRecorder()

	_, err := handler.GetResource(w, request)
	assert.NotNil(t, err)
	assert.IsType(t, &problem.P{}, err)
	p := err.(*problem.P)
	assert.Equal(t, "transaction_submission_exception", p.Type)
	assert.Equal(t, http.StatusInternalServerError, p.Status)
}

func TestAsyncSubmitTransactionHandler_TransactionStatusResponse(t *testing.T) {
	coreStateGetter := new(coreStateGetterMock)
	coreStateGetter.On("GetCoreState").Return(corestate.State{Synced: true})

	successCases := []struct {
		mockCoreResponse *proto.TXResponse
		expectedResponse horizon.AsyncTransactionSubmissionResponse
	}{
		{
			mockCoreResponse: &proto.TXResponse{
				Exception:        "",
				Error:            "test-error",
				Status:           proto.TXStatusError,
				DiagnosticEvents: "test-diagnostic-events",
			},
			expectedResponse: horizon.AsyncTransactionSubmissionResponse{
				ErrorResultXDR:           "test-error",
				DeprecatedErrorResultXDR: "test-error",
				TxStatus:                 proto.TXStatusError,
				Hash:                     TxHash,
			},
		},
		{
			mockCoreResponse: &proto.TXResponse{
				Status: proto.TXStatusPending,
			},
			expectedResponse: horizon.AsyncTransactionSubmissionResponse{
				TxStatus: proto.TXStatusPending,
				Hash:     TxHash,
			},
		},
		{
			mockCoreResponse: &proto.TXResponse{
				Status: proto.TXStatusDuplicate,
			},
			expectedResponse: horizon.AsyncTransactionSubmissionResponse{
				TxStatus: proto.TXStatusDuplicate,
				Hash:     TxHash,
			},
		},
		{
			mockCoreResponse: &proto.TXResponse{
				Status: proto.TXStatusTryAgainLater,
			},
			expectedResponse: horizon.AsyncTransactionSubmissionResponse{
				TxStatus: proto.TXStatusTryAgainLater,
				Hash:     TxHash,
			},
		},
	}

	for _, testCase := range successCases {
		MockClientWithMetrics := &MockClientWithMetrics{}
		MockClientWithMetrics.On("SubmitTx", context.Background(), TxXDR).Return(testCase.mockCoreResponse, nil)

		handler := AsyncSubmitTransactionHandler{
			NetworkPassphrase: network.PublicNetworkPassphrase,
			ClientWithMetrics: MockClientWithMetrics,
			CoreStateGetter:   coreStateGetter,
		}

		request := createRequest()
		w := httptest.NewRecorder()

		resp, err := handler.GetResource(w, request)
		assert.NoError(t, err)
		assert.Equal(t, resp, testCase.expectedResponse)
	}
}
