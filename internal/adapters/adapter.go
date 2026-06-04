package adapters

import (
	"context"

	"github.com/imtheghostkr/emrdrugmanager/internal/drug"
)

type ConnectionReport struct {
	OK             bool     `json:"ok"`
	ServerVersion  string   `json:"server_version"`
	ServerVersionN int      `json:"server_version_num"`
	CurrentUser    string   `json:"current_user"`
	IsSuperuser    bool     `json:"is_superuser"`
	IsReadonly     bool     `json:"is_readonly"`
	Warnings       []string `json:"warnings"`
	MissingTables  []string `json:"missing_tables"`
}

type DrugAdapter interface {
	Name() string
	TestConnection(ctx context.Context) (ConnectionReport, error)
	SearchDrugs(ctx context.Context, query string) ([]drug.Drug, error)
	GetDrug(ctx context.Context, code string) (drug.Drug, error)
	GetStock(ctx context.Context, code string) (drug.StockBalance, error)
	GetUsage(ctx context.Context, from, to string) ([]drug.UsageRow, error)
	GetUsageByCode(ctx context.Context, code, from, to string) (drug.UsageRow, error)
	GetStocks(ctx context.Context, codes []string) (map[string]drug.StockBalance, error)
	GetAllStocks(ctx context.Context) ([]drug.StockBalance, error)
}
