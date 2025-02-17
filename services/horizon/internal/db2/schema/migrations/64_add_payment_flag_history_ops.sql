-- +migrate Up

ALTER TABLE history_operations ADD is_payment boolean;

-- +migrate Down

ALTER TABLE history_operations DROP COLUMN is_payment;
