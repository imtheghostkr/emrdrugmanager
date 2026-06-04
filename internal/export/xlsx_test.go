package export

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/imtheghostkr/emrdrugmanager/internal/drug"
)

func TestOrderPlanXLSXExportsWorkbook(t *testing.T) {
	plan := drug.OrderPlan{
		From:       "20260501",
		To:         "20260530",
		TargetDays: 45,
		Summary:    drug.OrderPlanSummary{NeedCount: 1, UrgentCount: 1, RecommendedTotal: 10},
		Rows: []drug.OrderPlanRow{{
			Urgency:             "긴급",
			Category:            "향정/마약류",
			Ingredient:          "알프라졸람",
			Dosage:              "0.25mg",
			RepresentativeName:  "알프라낙스정0.25밀리그람",
			RecommendedOrderQty: 10,
			CurrentStockQty:     5,
			CoverageDays:        3,
			TargetStockQty:      15,
			ShortageQty:         10,
			StockSource:         "NIMS계산",
			MedfeeCode:          "653001570",
		}},
	}
	data, err := OrderPlanXLSX(plan)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string]bool{}
	for _, file := range zr.File {
		parts[file.Name] = true
	}
	for _, want := range []string{"xl/workbook.xml", "xl/worksheets/sheet1.xml", "xl/worksheets/sheet2.xml", "xl/worksheets/sheet3.xml"} {
		if !parts[want] {
			t.Fatalf("missing xlsx part %s", want)
		}
	}
}
