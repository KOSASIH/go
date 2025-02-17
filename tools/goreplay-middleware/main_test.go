package main

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcess(t *testing.T) {
	// For 1 type, it returns the same msg to stdout
	payload := "1 ID\nGET /ledgers HTTP/1.1\r\nHost: horizon.stellar.org\r\n\r\n"
	stdin := strings.NewReader(hex.EncodeToString([]byte(payload)))

	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	processAll(stdin, &stderr, &stdout)

	decodedOut, err := hex.DecodeString(strings.TrimRight(stdout.String(), "\n"))
	assert.NoError(t, err)
	assert.Equal(t, payload, string(decodedOut))
	assert.Equal(t, "", stderr.String())

	// For 2 type, save the original response
	payload = "2 ID\nHeader: true\r\n\r\nBody"
	stdin = strings.NewReader(hex.EncodeToString([]byte(payload)))

	stdout = bytes.Buffer{}
	stderr = bytes.Buffer{}
	processAll(stdin, &stderr, &stdout)

	assert.Len(t, pendingRequests, 1)
	assert.Equal(t, "", stdout.String())
	assert.Equal(t, "", stderr.String())

	// For 2 type, save the original response
	payload = "3 ID\nHeader: true\r\n\r\nBody"
	stdin = strings.NewReader(hex.EncodeToString([]byte(payload)))

	stdout = bytes.Buffer{}
	stderr = bytes.Buffer{}
	processAll(stdin, &stderr, &stdout)

	assert.Len(t, pendingRequests, 0)
	assert.Equal(t, "", stdout.String())
	assert.Equal(t, "", stderr.String())
}
