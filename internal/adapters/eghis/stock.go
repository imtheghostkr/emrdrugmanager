package eghis

import (
	"context"
	"fmt"
	"strings"

	"github.com/imtheghostkr/emrdrugmanager/internal/drug"
)

func (a *Adapter) generalStocks(ctx context.Context, codes []string) (map[string]drug.StockBalance, error) {
	out := map[string]drug.StockBalance{}
	firstInByCode := map[string]string{}
	for _, code := range codes {
		out[code] = drug.StockBalance{Code: code, Source: "DB계산"}
	}
	if len(codes) == 0 {
		return out, nil
	}

	rows, err := a.pool.Query(ctx, `
		SELECT code, COALESCE(int_ymd, ''), COALESCE(return_gb, ''), COALESCE(int_qty, 0)
		FROM (
			SELECT medfee_cd AS code, int_ymd, return_gb, int_qty
			FROM h0drug_stock
			WHERE medfee_cd = ANY($1::text[])
			UNION
			SELECT user_cd AS code, int_ymd, return_gb, int_qty
			FROM h0drug_stock
			WHERE user_cd = ANY($1::text[])
		) s
		WHERE COALESCE(code, '') <> ''
	`, codes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var code, intYmd, returnGb string
		var qty float64
		if err := rows.Scan(&code, &intYmd, &returnGb, &qty); err != nil {
			return nil, err
		}
		stock := out[code]
		if strings.TrimSpace(returnGb) == "" {
			stock.ReceivedQty += qty
			if intYmd != "" && (firstInByCode[code] == "" || intYmd < firstInByCode[code]) {
				firstInByCode[code] = intYmd
			}
		} else {
			stock.ReturnDisposalQty += qty
		}
		out[code] = stock
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	minFirstIn := ""
	activeCodes := make([]string, 0, len(firstInByCode))
	for code, firstIn := range firstInByCode {
		if firstIn == "" {
			continue
		}
		activeCodes = append(activeCodes, code)
		if minFirstIn == "" || firstIn < minFirstIn {
			minFirstIn = firstIn
		}
	}
	if len(activeCodes) == 0 {
		finalizeCurrentStock(out)
		return out, nil
	}

	usageRows, err := a.pool.Query(ctx, `
		SELECT code, COALESCE(ord_ymd, ''), COALESCE(recept_no::text, ''), COALESCE(ord_no::text, ''), COALESCE(ord_seq_no::text, ''), usage_qty
		FROM (
			SELECT h2.ord_cd AS code, h2.ord_ymd, h2.recept_no, h2.ord_no, h2.ord_seq_no,
			       CASE WHEN COALESCE(h2.cal_qty, 0) > 0 THEN h2.cal_qty ELSE COALESCE(h2.qty, 0) * COALESCE(h2.days, 0) END AS usage_qty
			FROM h2opd_doct_ord h2
			JOIN h1opdin h1 ON h1.recept_no = h2.recept_no
			WHERE h2.ord_cd = ANY($1::text[])
			  AND h2.ord_ymd >= $2
			  AND COALESCE(h1.close_ymd, '') <> ''
			UNION
			SELECT h2.medfee_cd AS code, h2.ord_ymd, h2.recept_no, h2.ord_no, h2.ord_seq_no,
			       CASE WHEN COALESCE(h2.cal_qty, 0) > 0 THEN h2.cal_qty ELSE COALESCE(h2.qty, 0) * COALESCE(h2.days, 0) END AS usage_qty
			FROM h2opd_doct_ord h2
			JOIN h1opdin h1 ON h1.recept_no = h2.recept_no
			WHERE h2.medfee_cd = ANY($1::text[])
			  AND h2.ord_ymd >= $2
			  AND COALESCE(h1.close_ymd, '') <> ''
			UNION
			SELECT h2.user_cd AS code, h2.ord_ymd, h2.recept_no, h2.ord_no, h2.ord_seq_no,
			       CASE WHEN COALESCE(h2.cal_qty, 0) > 0 THEN h2.cal_qty ELSE COALESCE(h2.qty, 0) * COALESCE(h2.days, 0) END AS usage_qty
			FROM h2opd_doct_ord h2
			JOIN h1opdin h1 ON h1.recept_no = h2.recept_no
			WHERE h2.user_cd = ANY($1::text[])
			  AND h2.ord_ymd >= $2
			  AND COALESCE(h1.close_ymd, '') <> ''
		) orders
		WHERE COALESCE(code, '') <> ''
		  AND COALESCE(usage_qty, 0) > 0
	`, activeCodes, minFirstIn)
	if err != nil {
		return nil, err
	}
	defer usageRows.Close()
	seenUsage := map[string]bool{}
	for usageRows.Next() {
		var code, ordYmd, receptNo, ordNo, ordSeqNo string
		var usageQty float64
		if err := usageRows.Scan(&code, &ordYmd, &receptNo, &ordNo, &ordSeqNo, &usageQty); err != nil {
			return nil, err
		}
		firstIn := firstInByCode[code]
		if firstIn == "" || ordYmd < firstIn {
			continue
		}
		key := strings.Join([]string{code, ordYmd, receptNo, ordNo, ordSeqNo}, "|")
		if seenUsage[key] {
			continue
		}
		seenUsage[key] = true
		stock := out[code]
		stock.InternalUsageQty += usageQty
		out[code] = stock
	}
	if err := usageRows.Err(); err != nil {
		return nil, err
	}
	finalizeCurrentStock(out)
	return out, nil
}

func (a *Adapter) controlledStocks(ctx context.Context, codes []string) (map[string]drug.StockBalance, error) {
	out := map[string]drug.StockBalance{}
	if len(codes) == 0 {
		return out, nil
	}
	productMap, productCodes, err := a.productCodesForMedfees(ctx, codes)
	if err != nil || len(productCodes) == 0 {
		return out, err
	}
	buy, err := a.nimsPackMovements(ctx, "h8_nims_buy", "h8_nims_buy_lines", productCodes)
	if err != nil {
		return nil, err
	}
	exp, err := a.nimsPackMovements(ctx, "h8_nims_exp", "h8_nims_exp_lines", productCodes)
	if err != nil {
		return nil, err
	}
	send, err := a.nimsPackMovements(ctx, "h8_nims_send", "h8_nims_send_lines", productCodes)
	if err != nil {
		return nil, err
	}
	medi, err := a.nimsMedicationMovements(ctx, codes, productCodes)
	if err != nil {
		return nil, err
	}
	for _, code := range codes {
		products := productMap[code]
		if len(products) == 0 {
			continue
		}
		stock := drug.StockBalance{Code: code, Source: "NIMS계산"}
		for _, productCode := range products {
			stock.ReceivedQty += buy[productCode]
			stock.ReturnDisposalQty += exp[productCode] + send[productCode]
			stock.InternalUsageQty += medi[code+"|"+productCode]
		}
		stock.CurrentStockQty = stock.ReceivedQty - stock.ReturnDisposalQty - stock.InternalUsageQty
		out[code] = stock
	}
	return out, nil
}

func finalizeCurrentStock(stocks map[string]drug.StockBalance) {
	for code, stock := range stocks {
		stock.CurrentStockQty = stock.ReceivedQty - stock.ReturnDisposalQty - stock.InternalUsageQty
		stocks[code] = stock
	}
}

func (a *Adapter) productCodesForMedfees(ctx context.Context, codes []string) (map[string][]string, []string, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT DISTINCT user_cd, prduct_cd
		FROM h8_nims_medi_lines
		WHERE user_cd = ANY($1::text[]) AND COALESCE(prduct_cd, '') <> ''
	`, codes)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	out := map[string][]string{}
	seenProducts := map[string]bool{}
	products := make([]string, 0)
	for rows.Next() {
		var code, product string
		if err := rows.Scan(&code, &product); err != nil {
			return nil, nil, err
		}
		out[code] = append(out[code], product)
		if !seenProducts[product] {
			seenProducts[product] = true
			products = append(products, product)
		}
	}
	return out, products, rows.Err()
}

func (a *Adapter) nimsPackMovements(ctx context.Context, headerTable, lineTable string, productCodes []string) (map[string]float64, error) {
	sql := fmt.Sprintf(`
		SELECT l.prduct_cd,
		       COALESCE(SUM(COALESCE(l.min_distb_qy, 0) * COALESCE(l.prd_tot_pce_qy, 0) + COALESCE(l.pce_qy, 0)), 0)
		FROM %s l
		JOIN %s h ON h.usr_rpt_id_no = l.usr_rpt_id_no
		WHERE l.prduct_cd = ANY($1::text[])
		  AND h.sts_cd = '20'
		  AND h.result_cd = '0000'
		  AND COALESCE(h.rpt_ty_cd, '0') <> '1'
		  AND COALESCE(h.usr_rpt_id_no, '') NOT IN (
			SELECT COALESCE(ref_usr_rpt_id_no, '')
			FROM %s
			WHERE sts_cd = '20' AND result_cd = '0000' AND COALESCE(ref_usr_rpt_id_no, '') <> ''
		  )
		GROUP BY l.prduct_cd
	`, safeIdent(lineTable), safeIdent(headerTable), safeIdent(headerTable))
	rows, err := a.pool.Query(ctx, sql, productCodes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]float64{}
	for rows.Next() {
		var code string
		var value float64
		if err := rows.Scan(&code, &value); err != nil {
			return nil, err
		}
		out[code] = value
	}
	return out, rows.Err()
}

func (a *Adapter) nimsMedicationMovements(ctx context.Context, userCodes, productCodes []string) (map[string]float64, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT l.user_cd, l.prduct_cd, COALESCE(SUM(COALESCE(l.pce_qy, 0)), 0)
		FROM h8_nims_medi_lines l
		JOIN h8_nims_medi h ON h.usr_rpt_id_no = l.usr_rpt_id_no
		WHERE l.user_cd = ANY($1::text[])
		  AND l.prduct_cd = ANY($2::text[])
		  AND h.sts_cd = '20'
		  AND h.result_cd = '0000'
		  AND COALESCE(h.rpt_ty_cd, '0') <> '1'
		  AND COALESCE(h.usr_rpt_id_no, '') NOT IN (
			SELECT COALESCE(ref_usr_rpt_id_no, '')
			FROM h8_nims_medi
			WHERE sts_cd = '20' AND result_cd = '0000' AND COALESCE(ref_usr_rpt_id_no, '') <> ''
		  )
		GROUP BY l.user_cd, l.prduct_cd
	`, userCodes, productCodes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]float64{}
	for rows.Next() {
		var userCode, productCode string
		var value float64
		if err := rows.Scan(&userCode, &productCode, &value); err != nil {
			return nil, err
		}
		out[userCode+"|"+productCode] = value
	}
	return out, rows.Err()
}

func (a *Adapter) productCodesForMedfee(ctx context.Context, code string) ([]string, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT DISTINCT prduct_cd
		FROM h8_nims_medi_lines
		WHERE user_cd = $1 AND COALESCE(prduct_cd, '') <> ''
	`, code)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var item string
		if err := rows.Scan(&item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (a *Adapter) nimsPackMovement(ctx context.Context, headerTable, lineTable, productCode string) (float64, error) {
	sql := fmt.Sprintf(`
		SELECT COALESCE(SUM(COALESCE(l.min_distb_qy, 0) * COALESCE(l.prd_tot_pce_qy, 0) + COALESCE(l.pce_qy, 0)), 0)
		FROM %s l
		JOIN %s h ON h.usr_rpt_id_no = l.usr_rpt_id_no
		WHERE l.prduct_cd = $1
		  AND h.sts_cd = '20'
		  AND h.result_cd = '0000'
		  AND COALESCE(h.rpt_ty_cd, '0') <> '1'
		  AND COALESCE(h.usr_rpt_id_no, '') NOT IN (
			SELECT COALESCE(ref_usr_rpt_id_no, '')
			FROM %s
			WHERE sts_cd = '20' AND result_cd = '0000' AND COALESCE(ref_usr_rpt_id_no, '') <> ''
		  )
	`, safeIdent(lineTable), safeIdent(headerTable), safeIdent(headerTable))
	var value float64
	err := a.pool.QueryRow(ctx, sql, productCode).Scan(&value)
	return value, err
}

func (a *Adapter) nimsMedicationMovement(ctx context.Context, productCode, userCode string) (float64, error) {
	var value float64
	err := a.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(COALESCE(l.pce_qy, 0)), 0)
		FROM h8_nims_medi_lines l
		JOIN h8_nims_medi h ON h.usr_rpt_id_no = l.usr_rpt_id_no
		WHERE l.prduct_cd = $1
		  AND l.user_cd = $2
		  AND h.sts_cd = '20'
		  AND h.result_cd = '0000'
		  AND COALESCE(h.rpt_ty_cd, '0') <> '1'
		  AND COALESCE(h.usr_rpt_id_no, '') NOT IN (
			SELECT COALESCE(ref_usr_rpt_id_no, '')
			FROM h8_nims_medi
			WHERE sts_cd = '20' AND result_cd = '0000' AND COALESCE(ref_usr_rpt_id_no, '') <> ''
		  )
	`, productCode, userCode).Scan(&value)
	return value, err
}

func safeIdent(value string) string {
	allowed := map[string]bool{
		"h8_nims_buy":        true,
		"h8_nims_buy_lines":  true,
		"h8_nims_exp":        true,
		"h8_nims_exp_lines":  true,
		"h8_nims_send":       true,
		"h8_nims_send_lines": true,
	}
	if !allowed[value] || strings.ContainsAny(value, " ;\t\r\n") {
		panic("unsafe identifier")
	}
	return value
}
