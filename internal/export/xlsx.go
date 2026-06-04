package export

import (
	"bytes"
	"fmt"

	"github.com/imtheghostkr/emrdrugmanager/internal/drug"
	"github.com/xuri/excelize/v2"
)

func OrderPlanXLSX(plan drug.OrderPlan) ([]byte, error) {
	f := excelize.NewFile()
	summary := "요약"
	f.SetSheetName("Sheet1", summary)
	f.NewSheet("주문필요")
	f.NewSheet("전체상세")

	summaryRows := [][]any{
		{"항목", "값"},
		{"처방기간", plan.From + " ~ " + plan.To},
		{"목표 비축일", plan.TargetDays},
		{"주문 필요 품목", plan.Summary.NeedCount},
		{"긴급 품목", plan.Summary.UrgentCount},
		{"권장주문 합계", plan.Summary.RecommendedTotal},
	}
	writeRows(f, summary, summaryRows)
	writePlanRows(f, "주문필요", filterNeeded(plan.Rows))
	writePlanRows(f, "전체상세", plan.Rows)
	for _, sheet := range []string{summary, "주문필요", "전체상세"} {
		_ = f.SetColWidth(sheet, "A", "N", 16)
		_ = f.AutoFilter(sheet, "A1:N1", nil)
	}
	buf := bytes.NewBuffer(nil)
	if err := f.Write(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writePlanRows(f *excelize.File, sheet string, rows []drug.OrderPlanRow) {
	data := [][]any{{
		"긴급도", "구분", "성분명", "용량", "약품명", "권장주문량", "현재재고", "재고일수",
		"목표재고", "부족량", "일평균", "월평균", "재고출처", "약품코드",
	}}
	for _, row := range rows {
		data = append(data, []any{
			row.Urgency, row.Category, row.Ingredient, row.Dosage, row.RepresentativeName,
			row.RecommendedOrderQty, row.CurrentStockQty, row.CoverageDays, row.TargetStockQty,
			row.ShortageQty, row.DailyUsageQty, row.Avg30dUsage, row.StockSource, row.MedfeeCode,
		})
	}
	writeRows(f, sheet, data)
}

func writeRows(f *excelize.File, sheet string, rows [][]any) {
	for r, row := range rows {
		for c, value := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			_ = f.SetCellValue(sheet, cell, value)
		}
	}
	style, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"2563EB"}, Pattern: 1},
	})
	if len(rows) > 0 && len(rows[0]) > 0 {
		end, _ := excelize.CoordinatesToCellName(len(rows[0]), 1)
		_ = f.SetCellStyle(sheet, "A1", end, style)
	}
}

func filterNeeded(rows []drug.OrderPlanRow) []drug.OrderPlanRow {
	out := make([]drug.OrderPlanRow, 0)
	for _, row := range rows {
		if row.RecommendedOrderQty > 0 {
			out = append(out, row)
		}
	}
	return out
}

func FileName(from, to string) string {
	return fmt.Sprintf("drug_order_plan_%s_%s.xlsx", from, to)
}
