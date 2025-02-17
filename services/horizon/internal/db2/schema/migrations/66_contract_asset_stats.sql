-- +migrate Up
CREATE TABLE contract_asset_stats (
     contract_id BYTEA PRIMARY KEY,
     stat       JSONB NOT NULL
);

CREATE TABLE contract_asset_balances (
     key_hash BYTEA PRIMARY KEY,
     asset_contract_id BYTEA NOT NULL,
     amount       numeric(39,0) NOT NULL, -- 39 digits is sufficient for a 128 bit integer
     expiration_ledger integer NOT NULL
);

CREATE INDEX "contract_asset_balances_by_expiration" ON contract_asset_balances USING btree (expiration_ledger);

-- +migrate Down
DROP TABLE contract_asset_stats cascade;
DROP TABLE contract_asset_balances cascade;