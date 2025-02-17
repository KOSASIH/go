-- +migrate Up
ALTER TABLE history_transactions ADD ledger_bounds                   int8range;     -- xdr.Uint32s
ALTER TABLE history_transactions ADD min_account_sequence            bigint;        -- xdr.SequenceNumber -> int64
ALTER TABLE history_transactions ADD min_account_sequence_age        varchar(20);   -- xdr.TimePoint -> uint64 -> longest uint64 number
ALTER TABLE history_transactions ADD min_account_sequence_ledger_gap bigint;        -- xdr.Int32
ALTER TABLE history_transactions ADD extra_signers                   text[];

ALTER TABLE accounts ADD sequence_ledger integer;
ALTER TABLE accounts ADD sequence_time bigint;

-- CAP-40 signed payload strkeys can be 165 characters long, see
-- strkey/main.go:maxEncodedSize. But we'll use text here, so we don't need to
-- adjust it *ever again*.
ALTER TABLE accounts_signers
  ALTER COLUMN signer TYPE text;

-- +migrate Down
ALTER TABLE history_transactions DROP ledger_bounds;
ALTER TABLE history_transactions DROP min_account_sequence;
ALTER TABLE history_transactions DROP min_account_sequence_age;
ALTER TABLE history_transactions DROP min_account_sequence_ledger_gap;
ALTER TABLE history_transactions DROP extra_signers;

ALTER TABLE accounts DROP sequence_ledger;
ALTER TABLE accounts DROP sequence_time;

-- we cannot restore the original type of varying(64) because there might be some
-- rows with signers that are too long.
ALTER TABLE accounts_signers
  ALTER COLUMN signer TYPE character varying(165);
