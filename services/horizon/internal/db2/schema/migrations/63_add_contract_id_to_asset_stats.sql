-- +migrate Up

ALTER TABLE exp_asset_stats ADD COLUMN contract_id BYTEA UNIQUE;

-- +migrate Down

ALTER TABLE exp_asset_stats DROP COLUMN contract_id;
