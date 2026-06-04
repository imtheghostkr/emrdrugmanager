package inventory

import (
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/imtheghostkr/emrdrugmanager/internal/drug"
)

var dosageRe = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(mg|㎎|g|mcg|μg|ug|ml|%)`)

type PlanOptions struct {
	GroupSameIngredientDose bool
	TruncateOrderQtyTo10    bool
}

func BuildOrderPlan(from, to string, targetDays, usageDays int, usage []drug.UsageRow, stocks map[string]drug.StockBalance) drug.OrderPlan {
	return BuildOrderPlanWithOptions(from, to, targetDays, usageDays, usage, stocks, PlanOptions{GroupSameIngredientDose: true})
}

func BuildOrderPlanWithOptions(from, to string, targetDays, usageDays int, usage []drug.UsageRow, stocks map[string]drug.StockBalance, opts PlanOptions) drug.OrderPlan {
	if targetDays <= 0 {
		targetDays = 45
	}
	if usageDays <= 0 {
		usageDays = 1
	}
	grouped := map[string]*drug.OrderPlanRow{}
	for _, row := range usage {
		ingredient := strings.TrimSpace(row.Component)
		if ingredient == "" {
			ingredient = extractIngredient(row.Name)
		}
		dosage := extractDosage(row.Name)
		key := row.Category + "|" + ingredient + "|" + dosage
		if !opts.GroupSameIngredientDose {
			key = row.Code
		}
		item := grouped[key]
		if item == nil {
			item = &drug.OrderPlanRow{
				Category:           row.Category,
				Ingredient:         ingredient,
				Dosage:             dosage,
				RepresentativeName: row.Name,
				ProductNames:       nonEmptyList(row.Name),
				MedfeeCode:         row.Code,
				StockSource:        "DB계산",
			}
			grouped[key] = item
		} else {
			item.ProductNames = appendUnique(item.ProductNames, row.Name)
			item.MedfeeCode = appendUniqueCSV(item.MedfeeCode, row.Code)
		}
		item.UsageQty += row.UsageQty
		item.OrderCount += row.OrderCount
	}

	rows := make([]drug.OrderPlanRow, 0, len(grouped))
	for _, item := range grouped {
		if len(item.ProductNames) > 0 {
			item.RepresentativeName = strings.Join(item.ProductNames, " | ")
		}
		for _, code := range strings.Split(item.MedfeeCode, ",") {
			code = strings.TrimSpace(code)
			if stock, ok := stocks[code]; ok {
				item.CurrentStockQty += stock.CurrentStockQty
				if stock.Source == "NIMS계산" {
					item.StockSource = "NIMS계산"
				}
			}
		}
		item.DailyUsageQty = round1(item.UsageQty / float64(usageDays))
		item.Avg30dUsage = round1(item.UsageQty / (float64(usageDays) / 30.0))
		item.TargetStockQty = round1(item.DailyUsageQty * float64(targetDays))
		item.ShortageQty = round1(math.Max(item.TargetStockQty-item.CurrentStockQty, 0))
		item.RecommendedOrderQty = int(math.Ceil(item.ShortageQty))
		if opts.TruncateOrderQtyTo10 {
			item.RecommendedOrderQty = (item.RecommendedOrderQty / 10) * 10
		}
		if item.DailyUsageQty > 0 {
			item.CoverageDays = round1(item.CurrentStockQty / item.DailyUsageQty)
		} else {
			item.CoverageDays = 9999
		}
		switch {
		case item.CoverageDays <= 7:
			item.Urgency = "긴급"
		case item.RecommendedOrderQty > 0:
			item.Urgency = "주문필요"
		default:
			item.Urgency = "충분"
		}
		rows = append(rows, *item)
	}
	sort.Slice(rows, func(i, j int) bool {
		ri, rj := urgencyRank(rows[i].Urgency), urgencyRank(rows[j].Urgency)
		if ri != rj {
			return ri < rj
		}
		if rows[i].CoverageDays != rows[j].CoverageDays {
			return rows[i].CoverageDays < rows[j].CoverageDays
		}
		return rows[i].RecommendedOrderQty > rows[j].RecommendedOrderQty
	})
	summary := drug.OrderPlanSummary{}
	for _, row := range rows {
		if row.RecommendedOrderQty > 0 {
			summary.NeedCount++
			summary.RecommendedTotal += row.RecommendedOrderQty
		}
		if row.Urgency == "긴급" {
			summary.UrgentCount++
		}
	}
	return drug.OrderPlan{From: from, To: to, TargetDays: targetDays, Summary: summary, Rows: rows}
}

func extractDosage(name string) string {
	match := dosageRe.FindStringSubmatch(name)
	if len(match) < 3 {
		return ""
	}
	unit := strings.ToLower(match[2])
	if unit == "㎎" {
		unit = "mg"
	}
	return match[1] + unit
}

func extractIngredient(name string) string {
	if idx := strings.Index(name, "("); idx >= 0 {
		end := strings.Index(name[idx+1:], ")")
		if end >= 0 {
			return strings.TrimSpace(name[idx+1 : idx+1+end])
		}
	}
	return strings.TrimSpace(name)
}

func urgencyRank(value string) int {
	switch value {
	case "긴급":
		return 0
	case "주문필요":
		return 1
	default:
		return 2
	}
}

func round1(value float64) float64 {
	return math.Round(value*10) / 10
}

func nonEmptyList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return []string{value}
}

func appendUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func appendUniqueCSV(csv, value string) string {
	return strings.Join(appendUnique(splitCSV(csv), value), ", ")
}

func splitCSV(csv string) []string {
	parts := strings.Split(csv, ",")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return cleaned
}
