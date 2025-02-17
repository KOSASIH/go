package txnbuild

import (
	"testing"
)

func TestBeginSponsoringFutureReservesRoundTrip(t *testing.T) {
	beginSponsoring := &BeginSponsoringFutureReserves{
		SponsoredID: newKeypair1().Address(),
	}

	testOperationsMarshalingRoundtrip(t, []Operation{beginSponsoring}, false)
}
