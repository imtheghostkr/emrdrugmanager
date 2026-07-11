package eghis

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/imtheghostkr/emrdrugmanager/internal/adapters"
	"github.com/imtheghostkr/emrdrugmanager/internal/config"
	"github.com/imtheghostkr/emrdrugmanager/internal/drug"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var requiredTables = []string{
	"h0_mst_drug",
	"h0drug_stock",
	"h1opdin",
	"h2opd_doct_ord",
	"h8_nims_buy",
	"h8_nims_buy_lines",
	"h8_nims_medi",
	"h8_nims_medi_lines",
	"h8_nims_exp",
	"h8_nims_exp_lines",
	"h8_nims_send",
	"h8_nims_send_lines",
}

var coreTables = []string{
	"h0_mst_drug",
	"h0drug_stock",
	"h1opdin",
	"h2opd_doct_ord",
}

type Adapter struct {
	pool *pgxpool.Pool
}

const latestDrugSubquery = `
	SELECT d.medfee_cd,
	       MAX(COALESCE(d.medfee_nm, '')) AS medfee_nm,
	       MAX(COALESCE(d.component, '')) AS component,
	       MAX(COALESCE(d.drug_gb, '')) AS drug_gb,
	       MAX(COALESCE(d.inject_path, '')) AS inject_path
	FROM h0_mst_drug d
	JOIN (
		SELECT medfee_cd, MAX(COALESCE(apply_ymd, '')) AS apply_ymd
		FROM h0_mst_drug
		GROUP BY medfee_cd
	) latest ON latest.medfee_cd = d.medfee_cd AND latest.apply_ymd = COALESCE(d.apply_ymd, '')
	GROUP BY d.medfee_cd
`

// drugLookupJoinSQL is shared by usage queries that need the drug master fields.
// Keep it compatible with the PostgreSQL versions used by deployed eGHIS systems.
const drugLookupJoinSQL = `
	LEFT JOIN (` + latestDrugSubquery + `) d
	  ON d.medfee_cd = COALESCE(NULLIF(h2.ord_cd, ''), NULLIF(h2.medfee_cd, ''), h2.user_cd)
`

const prescriptionUsageQtySQL = `
	CASE
		WHEN COALESCE(NULLIF(h2.inject_path, ''), d.inject_path, '') = '02' THEN
			CASE WHEN COALESCE(h2.cal_qty, 0) > 0 THEN h2.cal_qty ELSE COALESCE(h2.qty, 0) * COALESCE(h2.days, 0) END
		ELSE
			CASE WHEN COALESCE(h2.qty, 0) * COALESCE(h2.days, 0) > 0 THEN COALESCE(h2.qty, 0) * COALESCE(h2.days, 0) ELSE COALESCE(h2.cal_qty, 0) END
	END
`

func New(ctx context.Context, db config.DatabaseConfig, password string) (*Adapter, error) {
	pool, err := pgxpool.New(ctx, db.DSN(password))
	if err != nil {
		return nil, err
	}
	return &Adapter{pool: pool}, nil
}

func (a *Adapter) Close() {
	if a != nil && a.pool != nil {
		a.pool.Close()
	}
}

func (a *Adapter) Name() string { return "eghis" }

func (a *Adapter) TestConnection(ctx context.Context) (adapters.ConnectionReport, error) {
	report := adapters.ConnectionReport{}
	if err := a.pool.Ping(ctx); err != nil {
		return report, err
	}
	report.OK = true
	_ = a.pool.QueryRow(ctx, "SHOW server_version").Scan(&report.ServerVersion)
	_ = a.pool.QueryRow(ctx, "SHOW server_version_num").Scan(&report.ServerVersionN)
	_ = a.pool.QueryRow(ctx, "SELECT current_user").Scan(&report.CurrentUser)
	_ = a.pool.QueryRow(ctx, "SELECT rolsuper FROM pg_roles WHERE rolname = current_user").Scan(&report.IsSuperuser)
	if report.IsSuperuser {
		report.Warnings = append(report.Warnings, "PostgreSQL 관리자 계정입니다. 앱 저장은 차단됩니다.")
	}
	report.MissingTables = a.missingRequiredTables(ctx)
	if len(report.MissingTables) > 0 {
		report.Warnings = append(report.Warnings, "약품 조회에 필요한 일부 테이블 권한 또는 테이블이 없습니다.")
	}
	writable, err := a.hasAnyWritePrivilege(ctx)
	if err != nil {
		report.Warnings = append(report.Warnings, "권한 확인에 실패했습니다. 핵심 테이블 권한과 스키마명을 확인하세요.")
	} else if writable {
		report.Warnings = append(report.Warnings, "읽기 전용 계정이 아닙니다. INSERT/UPDATE/DELETE 권한을 제거하세요.")
	}
	report.IsReadonly = !report.IsSuperuser && err == nil && !writable
	return report, nil
}

func (a *Adapter) missingRequiredTables(ctx context.Context) []string {
	missing := make([]string, 0)
	for _, table := range requiredTables {
		var exists bool
		err := a.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=$1)", table).Scan(&exists)
		if err != nil || !exists {
			missing = append(missing, table)
		}
	}
	return missing
}

func (a *Adapter) hasAnyWritePrivilege(ctx context.Context) (bool, error) {
	for _, table := range coreTables {
		var insertPriv, updatePriv, deletePriv bool
		err := a.pool.QueryRow(ctx,
			"SELECT has_table_privilege(current_user, $1, 'INSERT'), has_table_privilege(current_user, $1, 'UPDATE'), has_table_privilege(current_user, $1, 'DELETE')",
			"public."+table,
		).Scan(&insertPriv, &updatePriv, &deletePriv)
		if err != nil {
			return false, err
		}
		if insertPriv || updatePriv || deletePriv {
			return true, nil
		}
	}
	return false, nil
}

func (a *Adapter) SearchDrugs(ctx context.Context, query string) ([]drug.Drug, error) {
	like := "%" + strings.TrimSpace(query) + "%"
	rows, err := a.pool.Query(ctx, `
		SELECT medfee_cd, MAX(COALESCE(medfee_nm, '')) AS medfee_nm,
		       MAX(COALESCE(component, '')) AS component,
		       MAX(COALESCE(drug_gb, '')) AS drug_gb
		FROM (`+latestDrugSubquery+`) d
		WHERE ($1 = '%%' OR medfee_cd LIKE $1 OR COALESCE(medfee_nm, '') ILIKE $1 OR COALESCE(component, '') ILIKE $1)
		GROUP BY medfee_cd
		ORDER BY MAX(COALESCE(medfee_nm, ''))
		LIMIT 100
	`, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]drug.Drug, 0)
	for rows.Next() {
		var item drug.Drug
		if err := rows.Scan(&item.Code, &item.Name, &item.Component, &item.DrugType); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (a *Adapter) GetDrug(ctx context.Context, code string) (drug.Drug, error) {
	var item drug.Drug
	err := a.pool.QueryRow(ctx, `
		SELECT medfee_cd, MAX(COALESCE(medfee_nm, '')) AS medfee_nm,
		       MAX(COALESCE(component, '')) AS component,
		       MAX(COALESCE(drug_gb, '')) AS drug_gb
		FROM (`+latestDrugSubquery+`) d
		WHERE medfee_cd = $1
		GROUP BY medfee_cd
	`, code).Scan(&item.Code, &item.Name, &item.Component, &item.DrugType)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, fmt.Errorf("drug not found: %s", code)
	}
	return item, err
}

func (a *Adapter) GetUsage(ctx context.Context, from, to string, opts adapters.QueryOptions) ([]drug.UsageRow, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT
			COALESCE(NULLIF(n.user_cd, ''), NULLIF(h2.ord_cd, ''), NULLIF(h2.medfee_cd, ''), h2.user_cd) AS code,
			COALESCE(STRING_AGG(DISTINCT COALESCE(NULLIF(h2.medfee_cd, ''), NULLIF(h2.ord_cd, ''), NULLIF(h2.user_cd, '')), ', '), '') AS insurance_code,
			MAX(COALESCE(NULLIF(d.medfee_nm, ''), NULLIF(h2.medfee_nm, ''), '')) AS name,
			MAX(COALESCE(d.component, '')) AS component,
			MAX(COALESCE(d.drug_gb, '')) AS drug_gb,
			SUM(`+prescriptionUsageQtySQL+`) AS usage_qty,
			COUNT(*) AS order_count,
			CASE WHEN MAX(n.user_cd) IS NULL THEN '일반약' ELSE '향정/마약류' END AS category
		FROM h2opd_doct_ord h2
		JOIN h1opdin h1 ON h1.recept_no = h2.recept_no
		`+drugLookupJoinSQL+`
		LEFT JOIN (
			SELECT ord_ymd, ord_no, ord_seq_no, MAX(user_cd) AS user_cd
			FROM h8_nims_medi_lines
			WHERE ord_ymd BETWEEN $1 AND $2
			GROUP BY ord_ymd, ord_no, ord_seq_no
		) n ON h2.ord_ymd = n.ord_ymd AND h2.ord_no = n.ord_no AND h2.ord_seq_no = n.ord_seq_no
			  AND (n.user_cd = h2.user_cd OR n.user_cd = h2.ord_cd OR n.user_cd = h2.medfee_cd)
		WHERE h2.ord_ymd BETWEEN $1 AND $2
		  AND (h2.ord_cd LIKE '6%' OR h2.medfee_cd LIKE '6%' OR h2.user_cd LIKE '6%' OR n.user_cd IS NOT NULL)
		  AND (`+prescriptionUsageQtySQL+`) > 0
		  AND ($3 = false OR (COALESCE(h2.inout_gb, '') <> 'O' AND BTRIM(COALESCE(h2.walkout_yn, '')) <> 'Y'))
		  AND ($4 = false OR COALESCE(NULLIF(h2.inject_path, ''), d.inject_path, '') <> '02')
		GROUP BY COALESCE(NULLIF(n.user_cd, ''), NULLIF(h2.ord_cd, ''), NULLIF(h2.medfee_cd, ''), h2.user_cd)
	`, from, to, opts.ExcludeOutside, opts.ExcludeInjection)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]drug.UsageRow, 0)
	for rows.Next() {
		var item drug.UsageRow
		if err := rows.Scan(&item.Code, &item.InsuranceCode, &item.Name, &item.Component, &item.DrugType, &item.UsageQty, &item.OrderCount, &item.Category); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (a *Adapter) GetUsageByCode(ctx context.Context, code, from, to string, opts adapters.QueryOptions) (drug.UsageRow, error) {
	code = strings.TrimSpace(code)
	var item drug.UsageRow
	err := a.pool.QueryRow(ctx, `
		SELECT
			$1 AS code,
			COALESCE(STRING_AGG(DISTINCT COALESCE(NULLIF(h2.medfee_cd, ''), NULLIF(h2.ord_cd, ''), NULLIF(h2.user_cd, '')), ', '), '') AS insurance_code,
			COALESCE(MAX(COALESCE(NULLIF(d.medfee_nm, ''), NULLIF(h2.medfee_nm, ''))), '') AS name,
			COALESCE(MAX(COALESCE(d.component, '')), '') AS component,
			COALESCE(MAX(COALESCE(d.drug_gb, '')), '') AS drug_gb,
			COALESCE(SUM(`+prescriptionUsageQtySQL+`), 0) AS usage_qty,
			COUNT(*) AS order_count,
			CASE WHEN MAX(n.user_cd) IS NULL THEN '일반약' ELSE '향정/마약류' END AS category
		FROM h2opd_doct_ord h2
		JOIN h1opdin h1 ON h1.recept_no = h2.recept_no
		`+drugLookupJoinSQL+`
		LEFT JOIN (
			SELECT DISTINCT ord_ymd, ord_no, ord_seq_no, user_cd
			FROM h8_nims_medi_lines
			WHERE ord_ymd BETWEEN $2 AND $3
		) n ON h2.ord_ymd = n.ord_ymd AND h2.ord_no = n.ord_no AND h2.ord_seq_no = n.ord_seq_no AND h2.user_cd = n.user_cd
		WHERE h2.ord_ymd BETWEEN $2 AND $3
		  AND (h2.user_cd = $1 OR h2.ord_cd = $1 OR h2.medfee_cd = $1)
		  AND (`+prescriptionUsageQtySQL+`) > 0
		  AND ($4 = false OR (COALESCE(h2.inout_gb, '') <> 'O' AND BTRIM(COALESCE(h2.walkout_yn, '')) <> 'Y'))
		  AND ($5 = false OR COALESCE(NULLIF(h2.inject_path, ''), d.inject_path, '') <> '02')
	`, code, from, to, opts.ExcludeOutside, opts.ExcludeInjection).Scan(&item.Code, &item.InsuranceCode, &item.Name, &item.Component, &item.DrugType, &item.UsageQty, &item.OrderCount, &item.Category)
	if err != nil {
		return drug.UsageRow{}, err
	}
	if item.Category == "" {
		item.Category = "일반약"
	}
	return item, nil
}

func (a *Adapter) GetStock(ctx context.Context, code string) (drug.StockBalance, error) {
	stocks, err := a.GetStocks(ctx, []string{code})
	if err != nil {
		return drug.StockBalance{}, err
	}
	if stock, ok := stocks[code]; ok {
		return stock, nil
	}
	return drug.StockBalance{Code: code, Source: "DB계산"}, nil
}

func (a *Adapter) GetStocks(ctx context.Context, codes []string) (map[string]drug.StockBalance, error) {
	normalized := uniqueCodes(codes)
	out, err := a.generalStocks(ctx, normalized)
	if err != nil {
		return nil, err
	}
	controlled, err := a.controlledStocks(ctx, normalized)
	if err == nil {
		for code, stock := range controlled {
			out[code] = stock
		}
	}
	if err := a.attachStockMetadata(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (a *Adapter) attachStockMetadata(ctx context.Context, stocks map[string]drug.StockBalance) error {
	codes := make([]string, 0, len(stocks))
	for code := range stocks {
		codes = append(codes, code)
	}
	if len(codes) == 0 {
		return nil
	}
	rows, err := a.pool.Query(ctx, `
		WITH code_mapping AS (
			SELECT medfee_cd AS code, medfee_cd AS master_code
			FROM h0drug_stock
			WHERE medfee_cd = ANY($1::text[]) AND COALESCE(medfee_cd, '') <> ''
			UNION
			SELECT user_cd AS code, COALESCE(NULLIF(medfee_cd, ''), user_cd) AS master_code
			FROM h0drug_stock
			WHERE user_cd = ANY($1::text[]) AND COALESCE(user_cd, '') <> ''
			UNION
			SELECT medfee_cd AS code, medfee_cd AS master_code
			FROM (`+latestDrugSubquery+`) d
			WHERE medfee_cd = ANY($1::text[])
		)
		SELECT m.code,
		       COALESCE(MAX(NULLIF(d.medfee_nm, '')), '') AS medfee_nm,
		       COALESCE(MAX(NULLIF(d.component, '')), '') AS component,
		       COALESCE(MAX(NULLIF(d.drug_gb, '')), '') AS drug_gb
		FROM code_mapping m
		LEFT JOIN (`+latestDrugSubquery+`) d ON d.medfee_cd = m.master_code
		GROUP BY m.code
	`, codes)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var code, name, component, drugType string
		if err := rows.Scan(&code, &name, &component, &drugType); err != nil {
			return err
		}
		stock, ok := stocks[code]
		if !ok {
			continue
		}
		stock.Name = name
		stock.Component = component
		stock.DrugType = drugType
		stocks[code] = stock
	}
	return rows.Err()
}

func (a *Adapter) GetAllStocks(ctx context.Context) ([]drug.StockBalance, error) {
	codes, err := a.stockCodes(ctx)
	if err != nil {
		return nil, err
	}
	stocks, err := a.GetStocks(ctx, codes)
	if err != nil {
		return nil, err
	}
	out := make([]drug.StockBalance, 0, len(stocks))
	for _, code := range codes {
		if stock, ok := stocks[code]; ok {
			out = append(out, stock)
		}
	}
	return out, nil
}

func (a *Adapter) stockCodes(ctx context.Context) ([]string, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT DISTINCT code
		FROM (
			SELECT medfee_cd AS code FROM h0drug_stock WHERE COALESCE(medfee_cd, '') <> ''
			UNION
			SELECT user_cd AS code FROM h0drug_stock WHERE COALESCE(user_cd, '') <> ''
			UNION
			SELECT user_cd AS code FROM h8_nims_medi_lines WHERE COALESCE(user_cd, '') <> ''
		) codes
		ORDER BY code
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		out = append(out, code)
	}
	return out, rows.Err()
}

func uniqueCodes(codes []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0)
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code != "" && !seen[code] {
			seen[code] = true
			out = append(out, code)
		}
	}
	sort.Strings(out)
	return out
}
