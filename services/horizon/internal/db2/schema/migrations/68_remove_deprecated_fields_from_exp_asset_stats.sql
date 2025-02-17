-- +migrate Up

ALTER TABLE exp_asset_stats DROP CONSTRAINT valid_num_accounts;

ALTER TABLE exp_asset_stats
    DROP COLUMN num_accounts;

ALTER TABLE exp_asset_stats
    DROP COLUMN amount;

-- +migrate Down
ALTER TABLE exp_asset_stats
    ADD COLUMN num_accounts INTEGER NOT NULL DEFAULT 0;

ALTER TABLE exp_asset_stats
    ADD COLUMN amount TEXT NOT NULL DEFAULT '';

ALTER TABLE exp_asset_stats ADD CONSTRAINT valid_num_accounts CHECK (num_accounts >= 0) NOT VALID;
