-- Example SQL Queries for Stellar Asset Balance Indexer
-- Run these queries against your DuckDB database
-- Usage: duckdb your_database.duckdb < queries.sql

-- ============================================================================
-- QUERY 1: Basic Statistics
-- ============================================================================
-- Get overview of indexed data

SELECT
    COUNT(*) as total_trustlines,
    COUNT(DISTINCT account_id) as unique_accounts,
    COUNT(DISTINCT asset_code) as unique_assets,
    COUNT(DISTINCT asset_issuer) as unique_issuers,
    MIN(last_modified_ledger) as earliest_ledger,
    MAX(last_modified_ledger) as latest_ledger
FROM account_balances;

-- ============================================================================
-- QUERY 2: Top Assets by Holder Count
-- ============================================================================
-- Find which assets are most widely held

SELECT
    asset_code,
    asset_issuer,
    COUNT(DISTINCT account_id) as holders,
    SUM(balance) / 10000000.0 as total_balance,
    AVG(balance) / 10000000.0 as avg_balance
FROM account_balances
GROUP BY asset_code, asset_issuer
ORDER BY holders DESC
LIMIT 20;

-- ============================================================================
-- QUERY 3: USDC Balance Distribution
-- ============================================================================
-- Analyze distribution of USDC balances

SELECT
    CASE
        WHEN balance = 0 THEN '0 - Zero balance (trustline only)'
        WHEN balance < 10000000 THEN '1 - Less than 1 USDC'
        WHEN balance < 100000000 THEN '2 - 1 to 10 USDC'
        WHEN balance < 1000000000 THEN '3 - 10 to 100 USDC'
        WHEN balance < 10000000000 THEN '4 - 100 to 1,000 USDC'
        ELSE '5 - More than 1,000 USDC'
    END as balance_range,
    COUNT(*) as num_accounts,
    SUM(balance) / 10000000.0 as total_in_range
FROM account_balances
WHERE asset_code = 'USDC'
GROUP BY balance_range
ORDER BY balance_range;

-- ============================================================================
-- QUERY 4: Top Holders by Asset
-- ============================================================================
-- Find largest holders of a specific asset (change USDC to your asset)

SELECT
    account_id,
    balance / 10000000.0 as balance_units,
    last_modified_ledger
FROM account_balances
WHERE asset_code = 'USDC'
  AND asset_issuer = 'GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5'
ORDER BY balance DESC
LIMIT 100;

-- ============================================================================
-- QUERY 5: Multi-Asset Holders
-- ============================================================================
-- Find accounts that hold many different assets

WITH asset_counts AS (
    SELECT
        account_id,
        COUNT(DISTINCT asset_code) as num_different_assets,
        SUM(CASE WHEN balance > 0 THEN 1 ELSE 0 END) as num_with_balance
    FROM account_balances
    GROUP BY account_id
)
SELECT
    num_different_assets,
    COUNT(*) as num_accounts,
    AVG(num_with_balance) as avg_with_balance
FROM asset_counts
GROUP BY num_different_assets
ORDER BY num_different_assets DESC;

-- ============================================================================
-- QUERY 6: Zero Balance Trustlines
-- ============================================================================
-- Find accounts that have trustlines but zero balance (potential airdrops)

SELECT
    asset_code,
    COUNT(*) as zero_balance_trustlines,
    COUNT(*) * 100.0 / SUM(COUNT(*)) OVER () as percentage
FROM account_balances
WHERE balance = 0
GROUP BY asset_code
ORDER BY zero_balance_trustlines DESC
LIMIT 20;

-- ============================================================================
-- QUERY 7: Trustline Creation Timeline
-- ============================================================================
-- When were trustlines created? (group by 10K ledgers)

SELECT
    (last_modified_ledger / 10000) * 10000 as ledger_bucket,
    COUNT(*) as new_trustlines,
    COUNT(DISTINCT asset_code) as unique_assets
FROM account_balances
GROUP BY ledger_bucket
ORDER BY ledger_bucket;

-- ============================================================================
-- QUERY 8: Asset Concentration
-- ============================================================================
-- Measure concentration: What % of supply do top holders control?

WITH ranked_holders AS (
    SELECT
        asset_code,
        asset_issuer,
        account_id,
        balance,
        SUM(balance) OVER (PARTITION BY asset_code, asset_issuer) as total_balance,
        ROW_NUMBER() OVER (PARTITION BY asset_code, asset_issuer ORDER BY balance DESC) as rank
    FROM account_balances
    WHERE balance > 0
)
SELECT
    asset_code,
    COUNT(*) as total_holders,
    SUM(CASE WHEN rank <= 10 THEN balance ELSE 0 END) * 100.0 / MAX(total_balance) as top10_percentage,
    SUM(CASE WHEN rank <= 100 THEN balance ELSE 0 END) * 100.0 / MAX(total_balance) as top100_percentage
FROM ranked_holders
GROUP BY asset_code, asset_issuer, total_balance
HAVING COUNT(*) >= 10
ORDER BY total_holders DESC;

-- ============================================================================
-- QUERY 9: Accounts with Specific Asset Combinations
-- ============================================================================
-- Find accounts that hold both USDC AND AQUA (or any two assets)

SELECT
    a.account_id,
    a.balance / 10000000.0 as usdc_balance,
    b.balance / 10000000.0 as aqua_balance
FROM account_balances a
JOIN account_balances b ON a.account_id = b.account_id
WHERE a.asset_code = 'USDC'
  AND b.asset_code = 'AQUA'
ORDER BY a.balance DESC
LIMIT 100;

-- ============================================================================
-- QUERY 10: Asset Issuer Analysis
-- ============================================================================
-- Find assets with multiple issuers (e.g., multiple USDC tokens)

SELECT
    asset_code,
    COUNT(DISTINCT asset_issuer) as num_issuers,
    COUNT(DISTINCT account_id) as total_holders,
    SUM(balance) / 10000000.0 as total_balance
FROM account_balances
GROUP BY asset_code
HAVING COUNT(DISTINCT asset_issuer) > 1
ORDER BY total_holders DESC;

-- ============================================================================
-- BONUS QUERY: Export Results to CSV
-- ============================================================================
-- To export any query results to CSV, use:
--
-- .mode csv
-- .output my_results.csv
-- SELECT ... your query here ...
-- .output stdout
-- .mode column

-- Example: Export top 1000 USDC holders
-- .mode csv
-- .output top_usdc_holders.csv
-- SELECT account_id, balance / 10000000.0 as balance_usdc
-- FROM account_balances
-- WHERE asset_code = 'USDC'
-- ORDER BY balance DESC
-- LIMIT 1000;
-- .output stdout

-- ============================================================================
-- BONUS QUERY: Create Summary Views
-- ============================================================================
-- Create reusable views for common queries

CREATE OR REPLACE VIEW asset_summary AS
SELECT
    asset_code,
    asset_issuer,
    COUNT(*) as total_trustlines,
    SUM(CASE WHEN balance > 0 THEN 1 ELSE 0 END) as holders_with_balance,
    SUM(balance) / 10000000.0 as total_balance,
    MAX(balance) / 10000000.0 as max_balance,
    AVG(balance) / 10000000.0 as avg_balance
FROM account_balances
GROUP BY asset_code, asset_issuer;

-- Now you can query the view:
-- SELECT * FROM asset_summary ORDER BY total_trustlines DESC;

-- ============================================================================
-- Usage Tips
-- ============================================================================
--
-- 1. Copy specific queries and run them in DuckDB CLI
-- 2. Modify WHERE clauses to filter for your assets
-- 3. Adjust LIMIT values for more/fewer results
-- 4. Use .mode csv and .output to export results
-- 5. Create views for frequently used queries
--
-- Connect to database:
--   duckdb your_database.duckdb
--
-- Run a specific query:
--   duckdb your_database.duckdb < queries.sql
--
-- Interactive mode tips:
--   .tables                   - List all tables
--   .schema account_balances  - Show table schema
--   .mode column              - Pretty print results
--   .mode csv                 - CSV output
--   .timer on                 - Show query timing
--
-- ============================================================================
