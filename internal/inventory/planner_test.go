package inventory

import (
	"testing"

	"github.com/imtheghostkr/emrdrugmanager/internal/drug"
)

func TestBuildOrderPlanCalculatesShortageAndUrgency(t *testing.T) {
	usage := []drug.UsageRow{
		{Code: "653001570", Name: "알프라낙스정0.25밀리그람 (0.25㎎)", Component: "알프라졸람", Category: "향정/마약류", UsageQty: 90, OrderCount: 3},
	}
	stocks := map[string]drug.StockBalance{
		"653001570": {Code: "653001570", CurrentStockQty: 10, Source: "NIMS계산"},
	}

	plan := BuildOrderPlan("20260501", "20260530", 45, 30, usage, stocks)
	if len(plan.Rows) != 1 {
		t.Fatalf("expected one row, got %d", len(plan.Rows))
	}
	row := plan.Rows[0]
	if row.DailyUsageQty != 3 {
		t.Fatalf("daily usage = %v, want 3", row.DailyUsageQty)
	}
	if row.TargetStockQty != 135 {
		t.Fatalf("target stock = %v, want 135", row.TargetStockQty)
	}
	if row.RecommendedOrderQty != 200 {
		t.Fatalf("recommended = %v, want 200", row.RecommendedOrderQty)
	}
	if row.Urgency != "긴급" {
		t.Fatalf("urgency = %q, want 긴급", row.Urgency)
	}
	if row.StockSource != "NIMS계산" {
		t.Fatalf("source = %q, want NIMS계산", row.StockSource)
	}
}

func TestBuildOrderPlanSufficientStock(t *testing.T) {
	usage := []drug.UsageRow{
		{Code: "626900860", Name: "누비질정250mg", Component: "아르모다피닐", Category: "일반약", UsageQty: 30, OrderCount: 1},
	}
	stocks := map[string]drug.StockBalance{
		"626900860": {Code: "626900860", CurrentStockQty: 100, Source: "DB계산"},
	}

	plan := BuildOrderPlan("20260501", "20260530", 45, 30, usage, stocks)
	row := plan.Rows[0]
	if row.RecommendedOrderQty != 0 {
		t.Fatalf("recommended = %v, want 0", row.RecommendedOrderQty)
	}
	if row.Urgency != "충분" {
		t.Fatalf("urgency = %q, want 충분", row.Urgency)
	}
}

func TestBuildOrderPlanKeepsSameIngredientProductsSeparateByDefault(t *testing.T) {
	usage := []drug.UsageRow{
		{Code: "RIVOTRIL", Name: "리보트릴정 0.5mg", Component: "클로나제팜", Category: "향정/마약류", UsageQty: 30},
		{Code: "GENERIC", Name: "클로나제팜정 0.5mg", Component: "클로나제팜", Category: "향정/마약류", UsageQty: 30},
	}
	stocks := map[string]drug.StockBalance{
		"RIVOTRIL": {Code: "RIVOTRIL", CurrentStockQty: 0, Source: "NIMS계산"},
		"GENERIC":  {Code: "GENERIC", CurrentStockQty: 1000, Source: "NIMS계산"},
	}

	plan := BuildOrderPlan("20260501", "20260530", 45, 30, usage, stocks)
	if len(plan.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(plan.Rows))
	}
	found := false
	for _, row := range plan.Rows {
		if row.MedfeeCode != "RIVOTRIL" {
			continue
		}
		found = true
		if row.RecommendedOrderQty != 100 {
			t.Fatalf("Rivotril recommended order = %d, want 100", row.RecommendedOrderQty)
		}
	}
	if !found {
		t.Fatal("Rivotril row was not found")
	}
}

func TestBuildOrderPlanRoundsOrderQuantityUpTo100(t *testing.T) {
	usage := []drug.UsageRow{{Code: "A", Name: "약품A 10mg", Component: "성분", Category: "일반약", UsageQty: 30}}
	stocks := map[string]drug.StockBalance{"A": {Code: "A", CurrentStockQty: 1}}

	plan := BuildOrderPlan("20260501", "20260530", 45, 30, usage, stocks)
	if plan.Rows[0].RecommendedOrderQty != 100 {
		t.Fatalf("recommended order = %d, want 100", plan.Rows[0].RecommendedOrderQty)
	}
}

func TestBuildOrderPlanOptions(t *testing.T) {
	usage := []drug.UsageRow{
		{Code: "A", Name: "약품A 10mg", Component: "성분", Category: "일반약", UsageQty: 90, OrderCount: 1},
		{Code: "B", Name: "약품B 10mg", Component: "성분", Category: "일반약", UsageQty: 30, OrderCount: 1},
	}
	stocks := map[string]drug.StockBalance{
		"A": {Code: "A", CurrentStockQty: 0, Source: "DB계산"},
		"B": {Code: "B", CurrentStockQty: 0, Source: "DB계산"},
	}

	grouped := BuildOrderPlanWithOptions("20260501", "20260530", 45, 30, usage, stocks, PlanOptions{GroupSameIngredientDose: true})
	if len(grouped.Rows) != 1 {
		t.Fatalf("grouped rows = %d, want 1", len(grouped.Rows))
	}
	if grouped.Rows[0].RepresentativeName != "약품A 10mg | 약품B 10mg" {
		t.Fatalf("representative name = %q, want joined product names", grouped.Rows[0].RepresentativeName)
	}
	if len(grouped.Rows[0].ProductNames) != 2 {
		t.Fatalf("product names = %v, want 2 names", grouped.Rows[0].ProductNames)
	}

	ungrouped := BuildOrderPlanWithOptions("20260501", "20260530", 45, 30, usage, stocks, PlanOptions{GroupSameIngredientDose: false, RoundOrderQtyUpTo100: true})
	if len(ungrouped.Rows) != 2 {
		t.Fatalf("ungrouped rows = %d, want 2", len(ungrouped.Rows))
	}
	for _, row := range ungrouped.Rows {
		if row.RecommendedOrderQty%100 != 0 {
			t.Fatalf("recommended order %d is not rounded to 100 units", row.RecommendedOrderQty)
		}
	}
}
