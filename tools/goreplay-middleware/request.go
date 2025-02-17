package main

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/buger/goreplay/proto"
)

var horizonURLs = regexp.MustCompile(`https:\/\/.*?(stellar\.org|127.0.0.1:8000)`)
var findResultMetaXDR = regexp.MustCompile(`"result_meta_xdr":[ ]?"([^"]*)",`)

// removeRegexps contains a list of regular expressions that, when matched,
// will be changed to an empty string. This is done to exclude known
// differences in responses between two Horizon version.
//
// Let's say that next Horizon version adds a new bool field:
// `is_authorized` on account balances list. You want to remove this
// field so it's not reported for each `/accounts/{id}` response.
var removeRegexps = []*regexp.Regexp{}

type replace struct {
	regexp *regexp.Regexp
	repl   string
}

// replaceRegexps works like removeRegexps but replaces data
var replaceRegexps = []replace{}

type Request struct {
	Headers          []byte
	OriginalResponse []byte
	MirroredResponse []byte
}

func (r *Request) OriginalBody() string {
	return string(proto.Body(r.OriginalResponse))
}

func (r *Request) MirroredBody() string {
	return string(proto.Body(r.MirroredResponse))
}

func (r *Request) IsIgnored() bool {
	if len(r.OriginalResponse) == 0 {
		return true
	}

	originalLatestLedgerHeader := proto.Header(r.OriginalResponse, []byte("Latest-Ledger"))
	mirroredLatestLedgerHeader := proto.Header(r.MirroredResponse, []byte("Latest-Ledger"))

	if !bytes.Equal(originalLatestLedgerHeader, mirroredLatestLedgerHeader) {
		return true
	}

	// Responses below are not supported but support can be added with some effort
	originalTransferEncodingHeader := proto.Header(r.OriginalResponse, []byte("Transfer-Encoding"))
	mirroredTransferEncodingHeader := proto.Header(r.MirroredResponse, []byte("Transfer-Encoding"))
	if len(originalTransferEncodingHeader) > 0 ||
		len(mirroredTransferEncodingHeader) > 0 {
		return true
	}

	acceptEncodingHeader := proto.Header(r.Headers, []byte("Accept-Encoding"))
	if strings.Contains(string(acceptEncodingHeader), "gzip") {
		return true
	}

	acceptHeader := proto.Header(r.Headers, []byte("Accept"))
	return strings.Contains(string(acceptHeader), "event-stream")
}

func (r *Request) ResponseEquals() bool {
	originalBody := proto.Body(r.OriginalResponse)
	mirroredBody := proto.Body(r.MirroredResponse)

	return normalizeResponseBody(originalBody) == normalizeResponseBody(mirroredBody)
}

// normalizeResponseBody normalizes body to allow byte-byte comparison like removing
// URLs from _links or tx meta. May require updating on new releases.
func normalizeResponseBody(body []byte) string {
	normalizedBody := string(body)
	// `result_meta_xdr` can differ between core instances (confirmed this with core team)
	normalizedBody = findResultMetaXDR.ReplaceAllString(normalizedBody, "")
	// Remove Horizon URL from the _links
	normalizedBody = horizonURLs.ReplaceAllString(normalizedBody, "")

	for _, reg := range removeRegexps {
		normalizedBody = reg.ReplaceAllString(normalizedBody, "")
	}

	for _, reg := range replaceRegexps {
		normalizedBody = reg.regexp.ReplaceAllString(normalizedBody, reg.repl)
	}

	return normalizedBody
}
