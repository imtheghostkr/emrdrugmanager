package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/imtheghostkr/emrdrugmanager/internal/adapters"
	"github.com/imtheghostkr/emrdrugmanager/internal/adapters/eghis"
	"github.com/imtheghostkr/emrdrugmanager/internal/config"
	"github.com/imtheghostkr/emrdrugmanager/internal/credential"
	"github.com/imtheghostkr/emrdrugmanager/internal/drug"
	"github.com/imtheghostkr/emrdrugmanager/internal/export"
	"github.com/imtheghostkr/emrdrugmanager/internal/inventory"
)

type Options struct {
	Version  string
	Config   config.Config
	Paths    config.Paths
	StaticFS fs.FS
	Logger   *slog.Logger
}

type App struct {
	version    string
	cfg        config.Config
	paths      config.Paths
	staticFS   fs.FS
	logger     *slog.Logger
	setupToken string
	credStore  credential.Store
	cidrs      []*net.IPNet
}

type setupRequest struct {
	Adapter  string `json:"adapter"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
	Password string `json:"password"`
	SSLMode  string `json:"sslmode"`
}

type serverSettingsRequest struct {
	Host                string   `json:"host"`
	Port                int      `json:"port"`
	AccessTokenRequired bool     `json:"access_token_required"`
	AccessToken         string   `json:"access_token"`
	AllowedCIDRs        []string `json:"allowed_cidrs"`
}

func New(opts Options) (*App, error) {
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	app := &App{
		version:    opts.Version,
		cfg:        opts.Config,
		paths:      opts.Paths,
		staticFS:   opts.StaticFS,
		logger:     opts.Logger,
		setupToken: token,
		credStore:  credential.Store{Dir: opts.Paths.CredentialDir},
	}
	for _, cidr := range opts.Config.Server.AllowedCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, err
		}
		app.cidrs = append(app.cidrs, network)
	}
	return app, nil
}

func (a *App) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(a.securityMiddleware)

	r.Get("/health", a.handleHealth)
	r.Get("/version", a.handleVersion)
	r.Get("/ui", a.handleUI)
	r.Handle("/assets/*", http.StripPrefix("/", http.FileServer(http.FS(a.staticFS))))

	r.Route("/api", func(api chi.Router) {
		api.Get("/setup/status", a.handleSetupStatus)
		api.Post("/setup/test-connection", a.handleSetupTest)
		api.Post("/setup/save", a.requireSetupToken(a.handleSetupSave))
		api.Post("/setup/reset", a.requireSetupToken(a.handleSetupReset))
		api.Get("/settings/server", a.handleServerSettings)
		api.Post("/settings/server", a.requireSetupToken(a.handleServerSettingsSave))

		api.Get("/drugs/search", a.withAdapter(a.handleDrugSearch))
		api.Get("/drugs/{code}", a.withAdapter(a.handleDrugDetail))
		api.Get("/drugs/{code}/stock", a.withAdapter(a.handleDrugStock))
		api.Get("/stocks", a.withAdapter(a.handleStocks))
		api.Get("/usage", a.withAdapter(a.handleUsage))
		api.Get("/user-codes/{code}/stock", a.withAdapter(a.handleUserCodeStock))
		api.Get("/user-codes/{code}/usage", a.withAdapter(a.handleUserCodeUsage))
		api.Get("/inventory/order-plan", a.withAdapter(a.handleOrderPlan))
		api.Get("/inventory/order-plan.xlsx", a.withAdapter(a.handleOrderPlanXLSX))
	})
	return r
}

func (a *App) securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.hostAllowed(r.Host) {
			writeError(w, http.StatusForbidden, "host is not allowed")
			return
		}
		if !a.remoteAllowed(r) {
			writeError(w, http.StatusForbidden, "remote address is not allowed")
			return
		}
		if a.cfg.Server.Host == "0.0.0.0" && !a.isLoopbackRequest(r) && a.cfg.Server.AccessTokenRequired && strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api/setup/status" {
			if !a.accessTokenOK(r) {
				writeError(w, http.StatusUnauthorized, "access token required")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) hostAllowed(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	host = strings.ToLower(host)
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	return a.cfg.Server.Host == "0.0.0.0"
}

func (a *App) remoteAllowed(r *http.Request) bool {
	if len(a.cidrs) == 0 {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, cidr := range a.cidrs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func (a *App) accessTokenOK(r *http.Request) bool {
	if a.cfg.Server.AccessTokenHash == "" {
		return false
	}
	token := r.Header.Get("X-Access-Token")
	if token == "" {
		token = r.URL.Query().Get("access_token")
	}
	if token == "" {
		return false
	}
	sum := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(a.cfg.Server.AccessTokenHash)) == 1
}

func (a *App) isLoopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (a *App) requireSetupToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Setup-Token")), []byte(a.setupToken)) != 1 {
			writeError(w, http.StatusUnauthorized, "setup token required")
			return
		}
		next(w, r)
	}
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "version": a.version})
}

func (a *App) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"version": a.version})
}

func (a *App) handleUI(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, a.staticFS, "index.html")
}

func (a *App) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	configured := a.cfg.Database.PasswordRef != "" && a.cfg.Database.User != "" && a.cfg.Database.Name != ""
	setupToken := ""
	if a.isLoopbackRequest(r) {
		setupToken = a.setupToken
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"configured":  configured,
		"setup_token": setupToken,
		"adapter":     a.cfg.EMR.Adapter,
		"database":    a.cfg.Database.RedactedSummary(),
		"message":     statusMessage(configured),
		"security": map[string]string{
			"label": securityLabel(a.cfg.Server),
			"role":  "읽기 전용 권장",
		},
	})
}

func statusMessage(configured bool) string {
	if configured {
		return "DB 설정이 저장되어 있습니다."
	}
	return "최초 설정이 필요합니다."
}

func securityLabel(server config.ServerConfig) string {
	if server.Host == "0.0.0.0" {
		if server.AccessTokenRequired && len(server.AllowedCIDRs) > 0 {
			return "LAN/VPN 공개"
		}
		if server.AccessTokenRequired {
			return "LAN 공개 주의"
		}
		return "위험"
	}
	return "로컬 전용"
}

func (a *App) handleSetupTest(w http.ResponseWriter, r *http.Request) {
	var req setupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	report, err := a.testRequest(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (a *App) handleSetupSave(w http.ResponseWriter, r *http.Request) {
	var req setupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	report, err := a.testRequest(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if report.IsSuperuser {
		writeError(w, http.StatusForbidden, "PostgreSQL 관리자 계정은 저장할 수 없습니다.")
		return
	}
	if !report.IsReadonly {
		writeError(w, http.StatusForbidden, "읽기 전용 계정만 저장할 수 있습니다. INSERT/UPDATE/DELETE 권한을 제거한 계정을 사용하세요.")
		return
	}
	if missing := missingCoreTables(report.MissingTables); len(missing) > 0 {
		writeError(w, http.StatusBadRequest, "Eghis 핵심 약품 테이블 권한이 없습니다: "+strings.Join(missing, ", "))
		return
	}
	db := requestDB(req)
	ref, err := a.credStore.Save("eghis_"+db.User, req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	db.PasswordRef = ref
	a.cfg.EMR.Adapter = "eghis"
	a.cfg.Database = db
	if err := config.Save(a.paths.ConfigPath, a.cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "report": report})
}

func missingCoreTables(missing []string) []string {
	core := map[string]bool{
		"h0_mst_drug":    true,
		"h0drug_stock":   true,
		"h1opdin":        true,
		"h2opd_doct_ord": true,
	}
	out := make([]string, 0)
	for _, table := range missing {
		if core[table] {
			out = append(out, table)
		}
	}
	return out
}

func (a *App) handleSetupReset(w http.ResponseWriter, r *http.Request) {
	if a.cfg.Database.PasswordRef != "" {
		_ = a.credStore.Delete(a.cfg.Database.PasswordRef)
	}
	a.cfg.Database.PasswordRef = ""
	if err := config.Save(a.paths.ConfigPath, a.cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handleServerSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"host":                    a.cfg.Server.Host,
		"port":                    a.cfg.Server.Port,
		"access_token_required":   a.cfg.Server.AccessTokenRequired,
		"access_token_configured": a.cfg.Server.AccessTokenHash != "",
		"allowed_cidrs":           a.cfg.Server.AllowedCIDRs,
		"restart_required":        false,
	})
}

func (a *App) handleServerSettingsSave(w http.ResponseWriter, r *http.Request) {
	var req serverSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	host := strings.TrimSpace(req.Host)
	if host == "" {
		host = "127.0.0.1"
	}
	if host != "127.0.0.1" && host != "0.0.0.0" {
		writeError(w, http.StatusBadRequest, "host must be 127.0.0.1 or 0.0.0.0")
		return
	}
	if req.Port <= 0 || req.Port > 65535 {
		writeError(w, http.StatusBadRequest, "port must be 1-65535")
		return
	}
	cidrs, networks, err := parseCIDRs(req.AllowedCIDRs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.AccessTokenRequired && strings.TrimSpace(req.AccessToken) == "" && a.cfg.Server.AccessTokenHash == "" {
		writeError(w, http.StatusBadRequest, "access token is required when token protection is enabled")
		return
	}

	next := a.cfg
	next.Server.Host = host
	next.Server.Port = req.Port
	next.Server.AccessTokenRequired = req.AccessTokenRequired
	next.Server.AllowedCIDRs = cidrs
	if token := strings.TrimSpace(req.AccessToken); token != "" {
		sum := sha256.Sum256([]byte(token))
		next.Server.AccessTokenHash = hex.EncodeToString(sum[:])
	}
	if !req.AccessTokenRequired {
		next.Server.AccessTokenHash = ""
	}
	if err := config.Save(a.paths.ConfigPath, next); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.cfg = next
	a.cidrs = networks
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                    true,
		"restart_required":      true,
		"access_token_required": a.cfg.Server.AccessTokenRequired,
		"allowed_cidrs":         a.cfg.Server.AllowedCIDRs,
		"host":                  a.cfg.Server.Host,
		"port":                  a.cfg.Server.Port,
	})
}

func parseCIDRs(values []string) ([]string, []*net.IPNet, error) {
	cleaned := make([]string, 0, len(values))
	networks := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			return nil, nil, errors.New("invalid CIDR: " + value)
		}
		cleaned = append(cleaned, value)
		networks = append(networks, network)
	}
	return cleaned, networks, nil
}

func (a *App) testRequest(ctx context.Context, req setupRequest) (adapters.ConnectionReport, error) {
	if strings.TrimSpace(req.User) == "" {
		return adapters.ConnectionReport{}, errors.New("user is required")
	}
	if strings.TrimSpace(req.Password) == "" {
		return adapters.ConnectionReport{}, errors.New("password is required")
	}
	adapter, err := eghis.New(ctx, requestDB(req), req.Password)
	if err != nil {
		return adapters.ConnectionReport{}, err
	}
	defer adapter.Close()
	return adapter.TestConnection(ctx)
}

func requestDB(req setupRequest) config.DatabaseConfig {
	sslmode := req.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}
	host := strings.TrimSpace(req.Host)
	if host == "" {
		host = "127.0.0.1"
	}
	port := req.Port
	if port == 0 {
		port = 5432
	}
	name := strings.TrimSpace(req.Database)
	if name == "" {
		name = "postgres"
	}
	return config.DatabaseConfig{
		Host:    host,
		Port:    port,
		Name:    name,
		User:    strings.TrimSpace(req.User),
		SSLMode: sslmode,
	}
}

type adapterHandler func(http.ResponseWriter, *http.Request, adapters.DrugAdapter)

func (a *App) withAdapter(handler adapterHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		password, err := a.credStore.Load(a.cfg.Database.PasswordRef)
		if err != nil {
			writeError(w, http.StatusPreconditionRequired, "DB 설정이 필요합니다.")
			return
		}
		adapter, err := eghis.New(r.Context(), a.cfg.Database, password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer adapter.Close()
		handler(w, r, adapter)
	}
}

func (a *App) handleDrugSearch(w http.ResponseWriter, r *http.Request, adapter adapters.DrugAdapter) {
	rows, err := adapter.SearchDrugs(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (a *App) handleDrugDetail(w http.ResponseWriter, r *http.Request, adapter adapters.DrugAdapter) {
	item, err := adapter.GetDrug(r.Context(), chi.URLParam(r, "code"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *App) handleDrugStock(w http.ResponseWriter, r *http.Request, adapter adapters.DrugAdapter) {
	item, err := adapter.GetStock(r.Context(), chi.URLParam(r, "code"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *App) handleStocks(w http.ResponseWriter, r *http.Request, adapter adapters.DrugAdapter) {
	rows, err := adapter.GetAllStocks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (a *App) handleUsage(w http.ResponseWriter, r *http.Request, adapter adapters.DrugAdapter) {
	from, to := dateRange(r)
	rows, err := adapter.GetUsage(r.Context(), from, to, usageQueryOptions(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if queryBool(r, "group_same", false) {
		rows = inventory.GroupUsageRowsByIngredientDose(rows)
	}
	writeJSON(w, http.StatusOK, rows)
}

func (a *App) handleUserCodeStock(w http.ResponseWriter, r *http.Request, adapter adapters.DrugAdapter) {
	item, err := adapter.GetStock(r.Context(), chi.URLParam(r, "code"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *App) handleUserCodeUsage(w http.ResponseWriter, r *http.Request, adapter adapters.DrugAdapter) {
	from, to := dateRange(r)
	item, err := adapter.GetUsageByCode(r.Context(), chi.URLParam(r, "code"), from, to, usageQueryOptions(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *App) handleOrderPlan(w http.ResponseWriter, r *http.Request, adapter adapters.DrugAdapter) {
	plan, err := a.orderPlan(r.Context(), r, adapter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (a *App) handleOrderPlanXLSX(w http.ResponseWriter, r *http.Request, adapter adapters.DrugAdapter) {
	plan, err := a.orderPlan(r.Context(), r, adapter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	data, err := export.OrderPlanXLSX(plan)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Content-Disposition", `attachment; filename="`+export.FileName(plan.From, plan.To)+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}

func (a *App) orderPlan(ctx context.Context, r *http.Request, adapter adapters.DrugAdapter) (drug.OrderPlan, error) {
	from, to := dateRange(r)
	targetDays, _ := strconv.Atoi(r.URL.Query().Get("target_days"))
	if targetDays <= 0 {
		targetDays = 45
	}
	groupSame := queryBool(r, "group_same", true)
	truncateOrderQty := queryBool(r, "truncate_order_qty", false)
	usage, err := adapter.GetUsage(ctx, from, to, usageQueryOptions(r))
	if err != nil {
		return drug.OrderPlan{}, err
	}
	codes := make([]string, 0, len(usage))
	for _, row := range usage {
		codes = append(codes, row.Code)
	}
	stocks, err := adapter.GetStocks(ctx, codes)
	if err != nil {
		return drug.OrderPlan{}, err
	}
	usageDays := daysBetween(from, to)
	plan := inventory.BuildOrderPlanWithOptions(from, to, targetDays, usageDays, usage, stocks, inventory.PlanOptions{
		GroupSameIngredientDose: groupSame,
		TruncateOrderQtyTo10:    truncateOrderQty,
	})
	return plan, nil
}

func usageQueryOptions(r *http.Request) adapters.QueryOptions {
	return adapters.QueryOptions{
		ExcludeOutside:   queryBool(r, "exclude_outside", false),
		ExcludeInjection: queryBool(r, "exclude_injection", false),
	}
}

func queryBool(r *http.Request, key string, defaultValue bool) bool {
	value := strings.ToLower(strings.TrimSpace(r.URL.Query().Get(key)))
	if value == "" {
		return defaultValue
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func dateRange(r *http.Request) (string, string) {
	now := time.Now()
	to := r.URL.Query().Get("to")
	from := r.URL.Query().Get("from")
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days > 0 {
		if to == "" {
			to = now.Format("20060102")
		}
		end, err := time.Parse("20060102", to)
		if err != nil {
			end = now
		}
		to = end.Format("20060102")
		from = end.AddDate(0, 0, -(days - 1)).Format("20060102")
		return from, to
	}
	if to == "" {
		to = now.Format("20060102")
	}
	if from == "" {
		from = now.AddDate(0, -3, 0).Format("20060102")
	}
	return from, to
}

func daysBetween(from, to string) int {
	start, err1 := time.Parse("20060102", from)
	end, err2 := time.Parse("20060102", to)
	if err1 != nil || err2 != nil || end.Before(start) {
		return 1
	}
	return int(end.Sub(start).Hours()/24) + 1
}

func randomToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
