package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go/support/log"
)

func TestIPLogging(t *testing.T) {
	done := log.DefaultLogger.StartTest(log.InfoLevel)

	mux := newMux(Config{UseCloudflareIP: true})
	mux.Get("/", func(w http.ResponseWriter, request *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/", nil)
	ipAddress := "255.128.255.128"
	request.Header.Set("CF-Connecting-IP", ipAddress)
	mux.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	logged := done()
	require.Len(t, logged, 2)
	require.Equal(t, "starting request", logged[0].Message)
	require.Equal(t, ipAddress, logged[0].Data["ip"])
	require.Equal(t, "finished request", logged[1].Message)
	require.Equal(t, ipAddress, logged[1].Data["ip"])
}
