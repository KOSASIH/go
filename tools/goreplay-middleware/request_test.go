package main

import (
	"testing"
)

func TestNormalizeResponseBody(t *testing.T) {
	//nolint:all
	resp := `{
	"_links": {
	  "self": {
		"href": "https://horizon-testnet.stellar.org/transactions?cursor=\u0026limit=1\u0026order=desc"
	  },
	  "next": {
		"href": "https://horizon-testnet.stellar.org/transactions?cursor=2892102128111616\u0026limit=1\u0026order=desc"
	  },
	  "prev": {
		"href": "https://horizon-testnet.stellar.org/transactions?cursor=2892102128111616\u0026limit=1\u0026order=asc"
	  }
	},
	"_embedded": {
	  "records": [
		{
		  "_links": {
			"self": {
			  "href": "https://horizon-testnet.stellar.org/transactions/cfc6395b98ed0c5ad63bd808e6b5ec994c3ccc67814945d3c652659b5077da18"
			},
			"account": {
			  "href": "https://horizon-testnet.stellar.org/accounts/GBW7BTQDKVMB62NXB3Z55NZCENRBD4O4OE7YBKBOG4K4DM4QK76L2DJD"
			},
			"ledger": {
			  "href": "https://horizon-testnet.stellar.org/ledgers/673370"
			},
			"operations": {
			  "href": "https://horizon-testnet.stellar.org/transactions/cfc6395b98ed0c5ad63bd808e6b5ec994c3ccc67814945d3c652659b5077da18/operations{?cursor,limit,order}",
			  "templated": true
			},
			"effects": {
			  "href": "https://horizon-testnet.stellar.org/transactions/cfc6395b98ed0c5ad63bd808e6b5ec994c3ccc67814945d3c652659b5077da18/effects{?cursor,limit,order}",
			  "templated": true
			},
			"precedes": {
			  "href": "https://horizon-testnet.stellar.org/transactions?order=asc\u0026cursor=2892102128111616"
			},
			"succeeds": {
			  "href": "https://horizon-testnet.stellar.org/transactions?order=desc\u0026cursor=2892102128111616"
			},
			"transaction": {
			  "href": "https://horizon-testnet.stellar.org/transactions/cfc6395b98ed0c5ad63bd808e6b5ec994c3ccc67814945d3c652659b5077da18"
			}
		  },
		  "id": "cfc6395b98ed0c5ad63bd808e6b5ec994c3ccc67814945d3c652659b5077da18",
		  "paging_token": "2892102128111616",
		  "successful": true,
		  "hash": "cfc6395b98ed0c5ad63bd808e6b5ec994c3ccc67814945d3c652659b5077da18",
		  "ledger": 673370,
		  "created_at": "2022-08-02T12:34:28Z",
		  "source_account": "GBW7BTQDKVMB62NXB3Z55NZCENRBD4O4OE7YBKBOG4K4DM4QK76L2DJD",
		  "source_account_sequence": "2892097833140225",
		  "fee_account": "GBW7BTQDKVMB62NXB3Z55NZCENRBD4O4OE7YBKBOG4K4DM4QK76L2DJD",
		  "fee_charged": "10000000",
		  "max_fee": "10000000",
		  "operation_count": 1,
		  "envelope_xdr": "AAAAAgAAAABt8M4DVVgfabcO8963IiNiEfHccT+AqC43FcGzkFf8vQCYloAACkZZAAAAAQAAAAEAAAAAAAAAAAAAAABi6RnwAAAAAAAAAAEAAAAAAAAACAAAAABKBB+2UBMP/abwcm/M1TXO+/JQWhPwkalgqizKmXyRIQAAAAAAAAABkFf8vQAAAEDmHHsYzqrzruPqKTFS0mcBPkpVkiF4ykNCJdAf/meM9NDYk+Eg/LD2B2epHdQZDC8TvecyiMQACjrb+Bdb+2YD",
		  "result_xdr": "AAAAAACYloAAAAAAAAAAAQAAAAAAAAAIAAAAAAAAABdH3lGAAAAAAA==",
		  "result_meta_xdr": "AAAAAgAAAAIAAAADAApGWgAAAAAAAAAAbfDOA1VYH2m3DvPetyIjYhHx3HE/gKguNxXBs5BX/L0AAAAXR95RgAAKRlkAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAApGWgAAAAAAAAAAbfDOA1VYH2m3DvPetyIjYhHx3HE/gKguNxXBs5BX/L0AAAAXR95RgAAKRlkAAAABAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAAAAAAAAAAAAwAAAAAACkZaAAAAAGLpGdQAAAAAAAAAAQAAAAQAAAADAApGWgAAAAAAAAAAbfDOA1VYH2m3DvPetyIjYhHx3HE/gKguNxXBs5BX/L0AAAAXR95RgAAKRlkAAAABAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAAAAAAAAAAAAwAAAAAACkZaAAAAAGLpGdQAAAAAAAAAAgAAAAAAAAAAbfDOA1VYH2m3DvPetyIjYhHx3HE/gKguNxXBs5BX/L0AAAADAApGTwAAAAAAAAAASgQftlATD/2m8HJvzNU1zvvyUFoT8JGpYKosypl8kSEMj1jFrrjxTQAAAM4AAAABAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAAAAAAAAAAAAwAAAAAAAAhlAAAAAGKzGdkAAAAAAAAAAQAKRloAAAAAAAAAAEoEH7ZQEw/9pvByb8zVNc778lBaE/CRqWCqLMqZfJEhDI9Y3PaXQs0AAADOAAAAAQAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAAAAAAAAAAAAAMAAAAAAAAIZQAAAABisxnZAAAAAAAAAAA=",
		  "fee_meta_xdr": "AAAAAgAAAAMACkZZAAAAAAAAAABt8M4DVVgfabcO8963IiNiEfHccT+AqC43FcGzkFf8vQAAABdIdugAAApGWQAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEACkZaAAAAAAAAAABt8M4DVVgfabcO8963IiNiEfHccT+AqC43FcGzkFf8vQAAABdH3lGAAApGWQAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAA==",
		  "memo_type": "none",
		  "signatures": [
			"5hx7GM6q867j6ikxUtJnAT5KVZIheMpDQiXQH/5njPTQ2JPhIPyw9gdnqR3UGQwvE73nMojEAAo62/gXW/tmAw=="
		  ],
		  "valid_after": "1970-01-01T00:00:00Z",
		  "valid_before": "2022-08-02T12:34:56Z",
		  "preconditions": {
			"timebounds": {
			  "min_time": "0",
			  "max_time": "1659443696"
			}
		  }
		}
	  ]
	}
  }`
	normalizedResp := normalizeResponseBody([]byte(resp))
	t.Log(normalizedResp)
}
