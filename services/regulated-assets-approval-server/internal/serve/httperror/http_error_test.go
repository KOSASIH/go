package httperror

import (
	"net/http"
	"testing"

	"github.com/stellar/go/clients/horizonclient"
	hProtocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/support/render/problem"
	"github.com/stretchr/testify/require"
)

func TestParseHorizonError(t *testing.T) {
	err := ParseHorizonError(nil)
	require.Nil(t, err)

	err = ParseHorizonError(errors.New("some error"))
	require.EqualError(t, err, "error submitting transaction: some error")

	horizonError := horizonclient.Error{
		Problem: problem.P{
			Type:   "bad_request",
			Title:  "Bad Request",
			Status: http.StatusBadRequest,
			Extras: map[string]interface{}{
				"result_codes": hProtocol.TransactionResultCodes{
					TransactionCode:      "tx_code_here",
					InnerTransactionCode: "",
					OperationCodes: []string{
						"op_success",
						"op_bad_auth",
					},
				},
			},
		},
	}
	err = ParseHorizonError(horizonError)
	require.EqualError(t, err, "error submitting transaction: problem: bad_request. full details: , &{TransactionCode:tx_code_here InnerTransactionCode: OperationCodes:[op_success op_bad_auth]}\n: horizon error: \"Bad Request\" (tx_code_here, op_success, op_bad_auth) - check horizon.Error.Problem for more information")
}
