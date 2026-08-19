DROP TRIGGER IF EXISTS trg_wallet_journal_balanced ON wallet_ledger_entries;
DROP FUNCTION IF EXISTS enforce_wallet_journal_balance();
DROP TRIGGER IF EXISTS trg_wallet_ledger_entries_immutable ON wallet_ledger_entries;
DROP TRIGGER IF EXISTS trg_wallet_journals_immutable ON wallet_journals;
DROP FUNCTION IF EXISTS prevent_wallet_ledger_mutation();

DROP TABLE IF EXISTS wallet_reconciliation_events;
DROP TABLE IF EXISTS wallet_withdrawal_requests;
DROP TABLE IF EXISTS wallet_balance_snapshots;
DROP TABLE IF EXISTS wallet_ledger_entries;
DROP TABLE IF EXISTS wallet_journals;
DROP TABLE IF EXISTS vendor_wallet_accounts;
DROP TABLE IF EXISTS wallet_fee_policies;
