# **Testing with Horizon**

Run all the Go monorepo unit tests like so (assuming you are at stellar/go, or run from stellar/go/services/horizon for just the Horizon subset):

```bash
go test ./...
```

or run individual Horizon tests like so, providing the expected arguments:

```bash
go test github.com/stellar/go/services/horizon/...
```

Before running integration tests, you also need to set some environment variables:
```bash
export HORIZON_INTEGRATION_TESTS_ENABLED=true
export HORIZON_INTEGRATION_TESTS_CORE_MAX_SUPPORTED_PROTOCOL=21
export HORIZON_INTEGRATION_TESTS_DOCKER_IMG=stellar/stellar-core:21.3.1-2007.4ede19620.focal
```
Make sure to check [horizon.yml](/.github/workflows/horizon.yml) for the latest core image version.

And then use the following command to run the Horizon integration tests:
```bash
go test -race -timeout 25m -v ./services/horizon/internal/integration/...
```

To run just one specific integration test, e.g. like `TestTxSub`:
```bash
go test -run TestTxsub -race -timeout 5m -v ./...
```

Authoring tests to assert coverage is key importance, to facilitate best experience for writing tests within Horizon packages, there are some conventions to be aware of:

## **Best Practices**
* For unit tests:
  * Adhere to [idiomatic go testing](https://go.dev/doc/tutorial/add-a-test) for 
    baseline

  * Try to maintain a `_test.go` file co-located in same folder as the `.go`
    sourcefile. 

  * Try to limit target code path under test. Use mocks on dependencies as much as 
    possible. Take dependencies into account during implementations, to encapsulate dependencies as much as possible through functional, interface, packaging, so they can be mocked out.

  * Assert on functional output/results, try to avoid assert on other aspects as that 
    tends to lead to brittle tests.

  * Do not use `services/horizon/internal/test/scenarios` DB setups, that framework is deprecated and will be EOL. 

  * For multi-table db seeding as part of test setup, use the newer notion of 'fixtures' for sql batch datasets. A 'fixture' is just a helper function that programatically loads DB from a hardcoded set of seed data and uses the session interface to do so. Refer to `services/horizon/internal/db2/history/trade_scenario.go` for example of a 'fixture' dataset.   
  
* For integration tests, they should be located in services/horizon/integration package. Tests located in this package will only run when `HORIZON_INTEGRATION_TESTS_ENABLED=true` is present in environment.

## **Leverage Scaffolding for Test Cases**
* Mocked DB unit tests that avoid needing a live db connection: 

  * `db/mock_session.go` has pre-defined mocks of all standard SessionInterface. `services/horizon/internal/httpx/stream_handler_test.go` is good example of mocking out just low level db session interface where sql statements are executed.

  * `services/horizon/internal/db2/history/mock_q_*.go`. This is a great set of mocked out horizon queries. Since this layer is mocked out, no need to deal with mocking any lower, i.e. the db session interfaces.  `services/horizon/internal/ingest/processors/accounts_processor_test.go` is a good reference example of using these mocked out db query interfaces from tests.

* Live DB unit tests, if you don't want to spend time mocking out sql results, there is lightweight db helper scaffolding framework available from `services/horizon/internal/test` package, it'll wire up session interface to a real connection that it initiates to `postgres://localhost:5432/horizon?sslmode=disable`, so your test won't have any boilerplate for setup.

  * `services/horizon/internal/db2/history/account_data_test.go` is a good example test that uses this live db scaffolding package. As part of test setup, it first inserts test data into tables using the same session interface.

  * If your test requires alot of repetitive test data loaded up front, then consider using a 'fixture' function instead to reduce duplicated code and encourage reuse of the fixture across other tests. Good example of 'fixture' is `TradeScenario` function in `services/horizon/internal/db2/history/trade_scenario.go`. Refer to `services/horizon/internal/db2/history/trade_test.go` for example of test that uses the 'fixture' function.

* Live DB with Mocked Web unit tests, mainly for exercising web server controllers in `services/horizon/internal/actions` for a given url path. The tests for controllers are all located at `services/horizon/internal/actions_*_test.go`. A good example of a test that mocks up the web layer but has a live DB is `services/horizon/internal/actions_path_test.go`.

* Live DB/Web unit tests, this can be used in any test case, but should be used sparingly, as it basically incurs same resources as integration test, but it will be run as part of unit tests. Try to consider whether any mocked levels of unit test can exercise the same target code path first. A good example of test that runs live web and db is `services/horizon/internal/actions_data_test.go`. Note how it uses `StartHTTPTestWithoutScenario` which explicitly avoids usage of deprecated scenarios framework. The test first inserts data into DB through session interfaces, then executes the web layer and checks responses. 

* Live DB/Web integration tests, there is a helper scaffolding framework from `services/horizon/internal/test/integration`:

  * `services/horizon/internal/integration/clawback_test.go` is good example of integration test that uses the scaffolding. 
  
  * integration tests only execute when `HORIZON_INTEGRATION_TESTS_ENABLED=true` is present as environment variable.










