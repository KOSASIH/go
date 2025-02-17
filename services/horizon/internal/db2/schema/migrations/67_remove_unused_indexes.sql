-- +migrate Up notransaction

DROP INDEX IF EXISTS accounts_home_domain;
DROP INDEX IF EXISTS accounts_inflation_destination;
DROP INDEX IF EXISTS asset_by_issuer;
DROP INDEX IF EXISTS claimabable_balances_by_claimants;
DROP INDEX IF EXISTS htrd_time_lookup;
DROP INDEX IF EXISTS index_history_effects_on_type;
DROP INDEX IF EXISTS index_history_ledgers_on_closed_at;
DROP INDEX IF EXISTS index_history_ledgers_on_importer_version;
DROP INDEX IF EXISTS index_history_ledgers_on_ledger_hash;
DROP INDEX IF EXISTS index_history_ledgers_on_previous_ledger_hash;
DROP INDEX IF EXISTS index_history_operation_claimable_balances_on_ids;
DROP INDEX IF EXISTS index_history_operations_on_type;
DROP INDEX IF EXISTS index_history_transaction_claimable_balances_on_ids;
DROP INDEX IF EXISTS trade_effects_by_order_book;
DROP INDEX IF EXISTS trust_lines_by_issuer;

-- +migrate Down notransaction
CREATE INDEX IF NOT EXISTS accounts_home_domain ON accounts USING BTREE(home_domain);
CREATE INDEX IF NOT EXISTS accounts_inflation_destination ON accounts USING BTREE(inflation_destination);
CREATE INDEX IF NOT EXISTS asset_by_issuer ON history_assets USING btree (asset_issuer);
CREATE INDEX IF NOT EXISTS claimabable_balances_by_claimants ON claimable_balances USING gin(claimants jsonb_path_ops);
CREATE INDEX IF NOT EXISTS htrd_time_lookup ON history_trades USING BTREE(ledger_closed_at);
CREATE INDEX IF NOT EXISTS index_history_effects_on_type ON history_effects USING btree (type);
CREATE INDEX IF NOT EXISTS index_history_ledgers_on_closed_at ON history_ledgers USING btree (closed_at);
CREATE INDEX IF NOT EXISTS index_history_ledgers_on_importer_version ON history_ledgers USING btree (importer_version);
CREATE UNIQUE INDEX IF NOT EXISTS index_history_ledgers_on_ledger_hash ON history_ledgers USING btree (ledger_hash);
CREATE UNIQUE INDEX IF NOT EXISTS index_history_ledgers_on_previous_ledger_hash ON history_ledgers USING btree (previous_ledger_hash);
CREATE UNIQUE INDEX IF NOT EXISTS index_history_operation_claimable_balances_on_ids ON history_operation_claimable_balances USING btree (history_operation_id , history_claimable_balance_id);
CREATE INDEX IF NOT EXISTS index_history_operations_on_type ON history_operations USING btree (type);
CREATE UNIQUE INDEX IF NOT EXISTS index_history_transaction_claimable_balances_on_ids ON history_transaction_claimable_balances USING btree (history_transaction_id , history_claimable_balance_id);
CREATE INDEX IF NOT EXISTS trade_effects_by_order_book ON history_effects USING btree (((details ->> 'sold_asset_type'::text)), ((details ->> 'sold_asset_code'::text)), ((details ->> 'sold_asset_issuer'::text)), ((details ->> 'bought_asset_type'::text)), ((details ->> 'bought_asset_code'::text)), ((details ->> 'bought_asset_issuer'::text))) WHERE (type = 33);
CREATE INDEX IF NOT EXISTS trust_lines_by_issuer ON trust_lines USING BTREE(asset_issuer);
