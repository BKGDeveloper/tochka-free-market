package marketplace

import (
	"time"
)

type CurrentTransactionStatus struct {
	Amount                float64
	BuyerUsername         string
	CreatedAt             time.Time
	CurrentAmount         float64
	CurrentShippingStatus string
	CurrentStatus         string
	Description           string
	DisputeUuid           string
	NumberOfMessages      int
	Type                  string
	UpdatedAt             time.Time
	Uuid                  string
	VendorUsername        string
}

type CurrentTransactionStatuses []CurrentTransactionStatus

type ViewCurrentTransactionStatus struct {
	*CurrentTransactionStatus
	UpdatedAtStr string
	CreatedAtStr string
	IsDisputed   bool
}

/*
	View Models
*/

func (ts CurrentTransactionStatus) ViewCurrentTransactionStatus(lang string) ViewCurrentTransactionStatus {
	return ViewCurrentTransactionStatus{
		CurrentTransactionStatus: &ts,
		IsDisputed:               (ts.DisputeUuid != ""),
		UpdatedAtStr:             ts.UpdatedAt.Format("02.01.2006 15:04"),
		CreatedAtStr:             ts.CreatedAt.Format("02.01.2006 15:04"),
	}
}

func (ctss CurrentTransactionStatuses) ViewCurrentTransactionStatuses(lang string) []ViewCurrentTransactionStatus {
	var vctss []ViewCurrentTransactionStatus
	for _, cts := range ctss {
		vcts := cts.ViewCurrentTransactionStatus(lang)
		vctss = append(vctss, vcts)
	}

	return vctss
}

/*
	Database Queries
*/

func FindCurrentTransactionStatuses(userUuid, status string, filterFailed bool, page, pageSize int,
) CurrentTransactionStatuses {
	var transactions []CurrentTransactionStatus
	qry := database.
		Table("v_current_cummulative_transaction_statuses")

	if filterFailed {
		qry = qry.Where("current_status <> ?", "FAILED")
	}

	if userUuid != "" {
		qry = qry.Where("(seller_uuid=? OR buyer_uuid=?)", userUuid, userUuid)
	}

	if status != "" {
		qry = qry.Where("current_status = ?", status)
	}

	if pageSize != 0 {
		qry = qry.Offset(page * pageSize).Limit(pageSize)
	}

	qry.Find(&transactions)
	return CurrentTransactionStatuses(transactions)
}

func CountCurrentTransactionStatuses(userUuid, status string, filterFailed bool) int {
	var count int
	qry := database.
		Table("v_current_transaction_statuses")

	if filterFailed {
		qry = qry.Where("current_status <> ?", "FAILED")
	}

	if userUuid != "" {
		qry = qry.Where("(seller_uuid=? OR buyer_uuid=?)", userUuid, userUuid)
	}

	if userUuid != "" {
		qry = qry.Where("(seller_uuid=? OR buyer_uuid=?)", userUuid, userUuid)
	}

	if status != "" {
		qry = qry.Where("current_status = ?", status)
	}

	qry.Count(&count)
	return count
}

func setupTransactionStatusesView() {

	database.Exec("DROP VIEW IF EXISTS v_transaction_statuses CASCADE")
	database.Exec(`
		CREATE VIEW v_transaction_statuses as (
			SELECT 
				ts.*, 
				ts1.amount AS min_amount, 
				ts2.amount AS max_amount, 
				ts1.status as min_status, 
				ts2.status as max_status
			FROM (
				SELECT ts.transaction_uuid, MAX(ts.time) max_timestamp, MIN(ts.time) as min_timestamp
				FROM transaction_statuses ts
				GROUP BY transaction_uuid
			) ts
			JOIN transaction_statuses ts1 on ts1.transaction_uuid=ts.transaction_uuid AND ts1.time = min_timestamp
			JOIN transaction_statuses ts2 on ts2.transaction_uuid=ts.transaction_uuid AND ts2.time = max_timestamp
	);`)

	database.Exec("DROP VIEW IF EXISTS v_shipping_statuses CASCADE;")
	database.Exec(`
		CREATE VIEW v_shipping_statuses as (
			SELECT 
				ss.*, 
				ss1.status as max_status
			FROM (
				SELECT transaction_uuid, MAX(time) max_timestamp, MIN(time) as min_timestamp
				FROM shipping_statuses
				GROUP BY transaction_uuid
			) ss
			JOIN shipping_statuses ss1 on ss1.transaction_uuid=ss.transaction_uuid AND ss1.time = max_timestamp
	);`)

	database.Exec("DROP VIEW IF EXISTS v_current_transaction_statuses CASCADE;")
	database.Exec(`
		CREATE VIEW v_current_transaction_statuses AS (
			WITH 
				vv_shipping_statuses AS (select * FROM v_shipping_statuses),
				vv_transaction_statuses AS (select * from v_transaction_statuses),
				vv_thread_counts AS (select * from v_thread_counts)
			select
				t.uuid,
				t.description,
				t.type,
				t.package_uuid,
				t.seller_uuid,
				t.buyer_uuid,
				t.dispute_uuid,
				t.bitcoin_transaction_uuid,
				t.bitcoin_cash_transaction_uuid,
				t.ethereum_transaction_uuid,
				vts.max_status as current_status,
				vts.max_amount as current_amount,
				vts.max_timestamp as updated_at,
				vts.min_timestamp as created_at,
				COALESCE(vss.max_status, 'DISPATCH PENDING') as current_shipping_status,
				COALESCE(vt.number_of_messages, 0) as number_of_messages,
				u1.username as vendor_username,
				u2.username as buyer_username
			from
				transactions t
			inner join 
				vv_transaction_statuses vts on t.uuid = vts.transaction_uuid
			inner join 
				users u1 on u1.uuid=t.seller_uuid
			inner join 
				users u2 on u2.uuid=t.buyer_uuid
			left outer join 
				vv_shipping_statuses vss on t.uuid = vss.transaction_uuid
			left outer join 
				vv_thread_counts vt on vt.parent_uuid = 'transaction-' || t.uuid
			ORDER BY
				created_at DESC
	);`)

	database.Exec("DROP VIEW IF EXISTS v_current_bitcoin_transaction_statuses CASCADE;")
	database.Exec(`
		CREATE VIEW v_current_bitcoin_transaction_statuses AS (
			SELECT 
				vcts.*, 
				btx.amount, 
				btx.is_multisig, 
				btx.buyer_public_key, 
				btx.seller_public_key, 
				btx.market_public_key, 
				btx.market_private_key
			FROM v_current_transaction_statuses vcts
			JOIN bitcoin_transactions btx on vcts.uuid=btx.uuid
			WHERE vcts.type='bitcoin'
			ORDER BY created_at DESC
	);`)

	database.Exec("DROP VIEW IF EXISTS v_current_bitcoin_cash_transaction_statuses CASCADE;")
	database.Exec(`
		CREATE VIEW v_current_bitcoin_cash_transaction_statuses AS (
			SELECT 
				vcts.*, 
				btx.amount, 
				btx.is_multisig, 
				btx.buyer_public_key, 
				btx.seller_public_key, 
				btx.market_public_key, 
				btx.market_private_key
			FROM v_current_transaction_statuses vcts
			JOIN bitcoin_cash_transactions btx on vcts.uuid=btx.uuid
			WHERE vcts.type='bitcoin_cash'
			ORDER BY created_at DESC
	);`)

	database.Exec("DROP VIEW IF EXISTS v_current_ethereum_transaction_statuses CASCADE;")
	database.Exec(`
		CREATE VIEW v_current_ethereum_transaction_statuses AS (
			SELECT 
				vcts.*, 
				etx.amount
			FROM v_current_transaction_statuses vcts
			JOIN ethereum_transactions etx on vcts.uuid=etx.uuid
			WHERE vcts.type='ethereum'
			ORDER BY created_at DESC
	);`)

	database.Exec("DROP VIEW IF EXISTS v_current_cummulative_transaction_statuses CASCADE;")
	database.Exec(`
		CREATE VIEW v_current_cummulative_transaction_statuses AS (
			SELECT * FROM (
				SELECT 
					uuid, 
					type, 
					description, 
					current_amount, 
					current_status, 
					current_shipping_status,
					number_of_messages,
					amount,
					buyer_username,
					vendor_username,
					dispute_uuid,
					package_uuid,
					seller_uuid,
					buyer_uuid,
					updated_at,
					created_at
				FROM v_current_bitcoin_transaction_statuses
				UNION
				SELECT 
					uuid, 
					type, 
					description, 
					current_amount, 
					current_status, 
					current_shipping_status,
					number_of_messages,
					amount,
					buyer_username,
					vendor_username,
					dispute_uuid,
					package_uuid,
					seller_uuid,
					buyer_uuid,
					updated_at,
					created_at
				FROM v_current_bitcoin_cash_transaction_statuses
				UNION
				SELECT 
					uuid, 
					type, 
					description, 
					current_amount, 
					current_status, 
					current_shipping_status,
					number_of_messages,
					amount,
					buyer_username,
					vendor_username,
					dispute_uuid,
					package_uuid,
					seller_uuid,
					buyer_uuid,
					updated_at,
					created_at
				FROM v_current_ethereum_transaction_statuses
				UNION
				SELECT 
					uuid, 
					type, 
					description, 
					current_amount, 
					current_status, 
					current_shipping_status,
					number_of_messages,
					amount,
					buyer_username,
					vendor_username,
					dispute_uuid,
					package_uuid,
					seller_uuid,
					buyer_uuid,
					updated_at,
					created_at
				FROM v_current_bitcoin_cash_transaction_statuses
			) united_txs
			ORDER BY created_at DESC
	);`)

	database.Exec("DROP MATERIALIZED VIEW IF EXISTS vm_current_bitcoin_transaction_statuses CASCADE;")
	database.Exec(`
		CREATE MATERIALIZED VIEW vm_current_bitcoin_transaction_statuses AS (
			select 
				* 
			FROM 
				v_current_bitcoin_transaction_statuses
	);`)

	database.Exec("DROP MATERIALIZED VIEW IF EXISTS vm_current_bitcoin_cash_transaction_statuses CASCADE;")
	database.Exec(`
		CREATE MATERIALIZED VIEW vm_current_bitcoin_cash_transaction_statuses AS (
			select 
				* 
			FROM 
				v_current_bitcoin_cash_transaction_statuses
	);`)

	database.Exec("DROP MATERIALIZED VIEW IF EXISTS vm_current_ethereum_transaction_statuses CASCADE;")
	database.Exec(`
		CREATE MATERIALIZED VIEW vm_current_ethereum_transaction_statuses AS (
			select 
				* 
			FROM 
				v_current_ethereum_transaction_statuses
	);`)
}

func setupVendorTxStatsViews() {
	database.Exec("DROP VIEW IF EXISTS v_vendor_bitcoin_tx_stats CASCADE;")
	database.Exec(`
		CREATE VIEW v_vendor_bitcoin_tx_stats AS (
			SELECT 
				seller_uuid, 
				count(*) as tx_number, 
				sum(amount) as tx_volume 
			FROM 
				v_current_bitcoin_transaction_statuses
			WHERE 
				current_status NOT IN ('CANCELLED', 'FAILED', 'PENDING') 
			GROUP BY 
				seller_uuid
	);`)

	database.Exec("DROP VIEW IF EXISTS v_vendor_bitcoin_cash_tx_stats CASCADE;")
	database.Exec(`
		CREATE VIEW v_vendor_bitcoin_cash_tx_stats AS (
			SELECT 
				seller_uuid, 
				count(*) as tx_number, 
				sum(amount) as tx_volume 
			FROM 
				v_current_bitcoin_cash_transaction_statuses
			WHERE 
				current_status NOT IN ('CANCELLED', 'FAILED', 'PENDING') 
			GROUP BY 
				seller_uuid
	);`)

	database.Exec("DROP VIEW IF EXISTS v_vendor_ethereum_tx_stats CASCADE;")
	database.Exec(`
		CREATE VIEW v_vendor_ethereum_tx_stats AS (
			SELECT 
				seller_uuid, 
				count(*) as tx_number, 
				sum(amount) as tx_volume 
			FROM 
				v_current_ethereum_transaction_statuses
			WHERE 
				current_status NOT IN ('CANCELLED', 'FAILED', 'PENDING') 
			GROUP BY 
				seller_uuid
	);`)
}

func setupItemTxStatsViews() {
	database.Exec("DROP VIEW IF EXISTS v_item_bitcoin_tx_stats CASCADE;")
	database.Exec(`
		CREATE VIEW v_item_bitcoin_tx_stats AS (
			SELECT 
				packages.item_uuid, 
				count(*) as tx_number, 
				sum(amount) as tx_volume 
			FROM 
				v_current_bitcoin_transaction_statuses
			JOIN
				packages on packages.uuid = v_current_bitcoin_transaction_statuses.package_uuid
			WHERE 
				current_status NOT IN ('CANCELLED', 'FAILED', 'PENDING') 
			GROUP BY 
				packages.item_uuid
			ORDER BY tx_volume DESC
	);`)

	database.Exec("DROP VIEW IF EXISTS v_item_bitcoin_cash_tx_stats CASCADE;")
	database.Exec(`
		CREATE VIEW v_item_bitcoin_cash_tx_stats AS (
			SELECT 
				packages.item_uuid, 
				count(*) as tx_number, 
				sum(amount) as tx_volume 
			FROM 
				v_current_bitcoin_cash_transaction_statuses
			JOIN
				packages on packages.uuid = v_current_bitcoin_cash_transaction_statuses.package_uuid
			WHERE 
				current_status NOT IN ('CANCELLED', 'FAILED', 'PENDING') 
			GROUP BY 
				packages.item_uuid
			ORDER BY tx_volume DESC
	);`)

	database.Exec("DROP VIEW IF EXISTS v_item_ethereum_tx_stats CASCADE;")
	database.Exec(`
		CREATE VIEW v_item_ethereum_tx_stats AS (
			SELECT 
				packages.item_uuid, 
				count(*) as tx_number, 
				sum(amount) as tx_volume 
			FROM 
				v_current_ethereum_transaction_statuses
			JOIN
				packages on packages.uuid = v_current_ethereum_transaction_statuses.package_uuid
			WHERE 
				current_status NOT IN ('CANCELLED', 'FAILED', 'PENDING') 
			GROUP BY 
				packages.item_uuid
			ORDER BY tx_volume DESC
	);`)
}
