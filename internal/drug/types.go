package drug

type Drug struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Component string `json:"component"`
	DrugType  string `json:"drug_type"`
}

type StockBalance struct {
	Code              string  `json:"code"`
	Name              string  `json:"name"`
	Component         string  `json:"component"`
	DrugType          string  `json:"drug_type"`
	ReceivedQty       float64 `json:"received_qty"`
	ReturnDisposalQty float64 `json:"return_disposal_qty"`
	InternalUsageQty  float64 `json:"internal_usage_qty"`
	CurrentStockQty   float64 `json:"current_stock_qty"`
	Source            string  `json:"source"`
}

type UsageRow struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Component  string  `json:"component"`
	DrugType   string  `json:"drug_type"`
	Category   string  `json:"category"`
	Qty        float64 `json:"qty"`
	Days       float64 `json:"days"`
	UsageQty   float64 `json:"usage_qty"`
	OrderCount int     `json:"order_count"`
}

type OrderPlanRow struct {
	Category            string   `json:"category"`
	Ingredient          string   `json:"ingredient"`
	Dosage              string   `json:"dosage"`
	RepresentativeName  string   `json:"representative_name"`
	ProductNames        []string `json:"product_names"`
	MedfeeCode          string   `json:"medfee_code"`
	UsageQty            float64  `json:"usage_qty"`
	DailyUsageQty       float64  `json:"daily_usage_qty"`
	Avg30dUsage         float64  `json:"avg_30d_usage"`
	CurrentStockQty     float64  `json:"current_stock_qty"`
	TargetStockQty      float64  `json:"target_stock_qty"`
	ShortageQty         float64  `json:"shortage_qty"`
	RecommendedOrderQty int      `json:"recommended_order_qty"`
	CoverageDays        float64  `json:"coverage_days"`
	Urgency             string   `json:"urgency"`
	StockSource         string   `json:"stock_source"`
	OrderCount          int      `json:"order_count"`
}

type OrderPlanSummary struct {
	NeedCount        int `json:"need_count"`
	UrgentCount      int `json:"urgent_count"`
	RecommendedTotal int `json:"recommended_total"`
}

type OrderPlan struct {
	From       string           `json:"from"`
	To         string           `json:"to"`
	TargetDays int              `json:"target_days"`
	Summary    OrderPlanSummary `json:"summary"`
	Rows       []OrderPlanRow   `json:"rows"`
}
