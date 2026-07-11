const state = { setupToken: "", configured: false, stockRows: [] };

const $ = (id) => document.getElementById(id);

async function api(path, options = {}) {
  const headers = options.headers || {};
  if (state.setupToken) headers["X-Setup-Token"] = state.setupToken;
  if (options.body && !headers["Content-Type"]) headers["Content-Type"] = "application/json";
  const res = await fetch(path, { ...options, headers });
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { data = text; }
  if (!res.ok) throw new Error(typeof data === "string" ? data : (data?.error || res.statusText));
  return data;
}

function filenameFromDisposition(header, fallback) {
  if (!header) return fallback;
  const utf8 = header.match(/filename\*=UTF-8''([^;]+)/i);
  if (utf8) return decodeURIComponent(utf8[1].replaceAll("+", "%20"));
  const quoted = header.match(/filename="([^"]+)"/i);
  if (quoted) return quoted[1];
  const plain = header.match(/filename=([^;]+)/i);
  return plain ? plain[1].trim() : fallback;
}

async function downloadFile(path, fallbackName) {
  const headers = {};
  if (state.setupToken) headers["X-Setup-Token"] = state.setupToken;
  const res = await fetch(path, { headers });
  if (!res.ok) {
    const text = await res.text();
    let message = text || res.statusText;
    try { message = JSON.parse(text)?.error || message; } catch {}
    throw new Error(message);
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filenameFromDisposition(res.headers.get("Content-Disposition"), fallbackName);
  document.body.appendChild(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

function formConfig() {
  return {
    adapter: "eghis",
    host: $("dbHost").value.trim(),
    port: Number($("dbPort").value),
    database: $("dbName").value.trim(),
    user: $("dbUser").value.trim(),
    password: $("dbPassword").value,
    sslmode: $("sslMode").value
  };
}

function showSetup(show) {
  $("setupPanel").classList.toggle("hidden", !show);
  $("appPanel").classList.toggle("hidden", show);
}

function setStatus(status) {
  $("statusLine").textContent = status.message || "-";
  $("securityBadge").textContent = status.security?.label || "확인됨";
  $("adapterName").textContent = status.adapter || "-";
  $("dbSummary").textContent = status.database || "-";
  $("roleSummary").textContent = status.security?.role || "-";
}

async function init() {
  const status = await api("/api/setup/status");
  state.setupToken = status.setup_token || "";
  state.configured = status.configured;
  setStatus(status);
  showSetup(!status.configured);
  await loadServerSettings();
}

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

function setText(id, value) {
  $(id).textContent = value;
}

function renderTable(target, rows, columns) {
  if (!rows || rows.length === 0) {
    target.innerHTML = "<p>결과가 없습니다.</p>";
    return;
  }
  const header = columns.map(c => `<th>${escapeHtml(c.label)}</th>`).join("");
  const body = rows.map(row => `<tr>${columns.map(c => `<td>${escapeHtml(row[c.key])}</td>`).join("")}</tr>`).join("");
  target.innerHTML = `<table><thead><tr>${header}</tr></thead><tbody>${body}</tbody></table>`;
}

function dateQuery(daysId, fromId, toId) {
  const from = $(fromId).value.trim();
  const to = $(toId).value.trim();
  if (from || to) {
    return `from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`;
  }
  const days = Number($(daysId).value || 365);
  return `days=${encodeURIComponent(days)}`;
}

function appendOption(parts, id, key) {
  parts.push(`${key}=${$(id).checked ? "true" : "false"}`);
}

function usageQuery() {
  const parts = [dateQuery("usageDays", "usageFrom", "usageTo")];
  appendOption(parts, "usageGroupSameDose", "group_same");
  appendOption(parts, "usageExcludeOutside", "exclude_outside");
  appendOption(parts, "usageExcludeInjection", "exclude_injection");
  return parts.join("&");
}

function planQuery() {
  const preset = $("planPreset").value;
  const parts = [];
  if (preset === "custom") {
    parts.push(`from=${encodeURIComponent($("fromDate").value.trim())}`);
    parts.push(`to=${encodeURIComponent($("toDate").value.trim())}`);
  } else {
    parts.push(`days=${encodeURIComponent(preset)}`);
  }
  parts.push(`target_days=${encodeURIComponent(Number($("targetDays").value || 45))}`);
  parts.push(`group_same=${$("groupSameDose").checked ? "true" : "false"}`);
  appendOption(parts, "planExcludeOutside", "exclude_outside");
  appendOption(parts, "planExcludeInjection", "exclude_injection");
  parts.push(`round_order_qty_to_100=${$("roundOrderQtyTo100").checked ? "true" : "false"}`);
  return parts.join("&");
}

async function loadServerSettings() {
  const settings = await api("/api/settings/server");
  $("serverHost").value = settings.host || "127.0.0.1";
  $("serverPort").value = settings.port || 3987;
  $("accessTokenRequired").checked = Boolean(settings.access_token_required);
  $("allowedCidrs").value = (settings.allowed_cidrs || []).join("\n");
  $("accessToken").placeholder = settings.access_token_configured ? "기존 토큰 유지" : "새 토큰 입력";
}

function serverSettingsPayload() {
  const cidrs = $("allowedCidrs").value
    .split(/[\n,]+/)
    .map(value => value.trim())
    .filter(Boolean);
  return {
    host: $("serverHost").value,
    port: Number($("serverPort").value || 3987),
    access_token_required: $("accessTokenRequired").checked,
    access_token: $("accessToken").value,
    allowed_cidrs: cidrs,
  };
}

const stockColumns = [
  { key: "code", label: "코드" },
  { key: "name", label: "약품명" },
  { key: "component", label: "성분" },
  { key: "drug_type", label: "구분" },
  { key: "current_stock_qty", label: "현재재고" },
  { key: "received_qty", label: "입고" },
  { key: "return_disposal_qty", label: "반품/폐기" },
  { key: "internal_usage_qty", label: "사용량" },
  { key: "source", label: "출처" },
];

const usageColumns = [
  { key: "code", label: "코드" },
  { key: "insurance_code", label: "보험코드" },
  { key: "name", label: "약품명" },
  { key: "component", label: "성분" },
  { key: "category", label: "구분" },
  { key: "usage_qty", label: "처방량" },
  { key: "order_count", label: "처방건수" },
];

function renderStocks() {
  const query = $("stockQuery").value.trim().toLocaleLowerCase("ko-KR");
  const rows = query
    ? state.stockRows.filter(row => [row.code, row.name, row.component]
      .some(value => String(value ?? "").toLocaleLowerCase("ko-KR").includes(query)))
    : state.stockRows;
  const status = query ? `${rows.length}개 표시 / 전체 ${state.stockRows.length}개` : `${rows.length}개 코드 조회됨`;
  setText("stocksStatus", status);
  renderTable($("stocksResults"), rows, stockColumns);
}

document.addEventListener("input", event => {
  if (event.target.id === "stockQuery" && state.stockRows.length > 0) renderStocks();
});

document.addEventListener("click", async (event) => {
  const tab = event.target.closest("[data-tab]");
  if (tab) {
    document.querySelectorAll("[data-tab]").forEach(b => b.classList.remove("active"));
    document.querySelectorAll(".tabPanel").forEach(p => p.classList.add("hidden"));
    tab.classList.add("active");
    $(tab.dataset.tab).classList.remove("hidden");
  }

  if (event.target.id === "testConnectionBtn") {
    $("setupResult").textContent = "연결 테스트 중...";
    try {
      const result = await api("/api/setup/test-connection", { method: "POST", body: JSON.stringify(formConfig()) });
      $("setupResult").textContent = JSON.stringify(result, null, 2);
    } catch (err) {
      $("setupResult").textContent = err.message;
    }
  }

  if (event.target.id === "saveSetupBtn") {
    $("setupResult").textContent = "저장 중...";
    try {
      const result = await api("/api/setup/save", { method: "POST", body: JSON.stringify(formConfig()) });
      $("setupResult").textContent = JSON.stringify(result, null, 2);
      await init();
    } catch (err) {
      $("setupResult").textContent = err.message;
    }
  }

  if (event.target.id === "resetSetupBtn") {
    if (!confirm("저장된 DB 설정을 초기화할까요?")) return;
    await api("/api/setup/reset", { method: "POST" });
    await init();
  }

  if (event.target.id === "saveServerSettingsBtn") {
    setText("serverSettingsStatus", "서버 설정 저장 중...");
    try {
      const result = await api("/api/settings/server", { method: "POST", body: JSON.stringify(serverSettingsPayload()) });
      $("accessToken").value = "";
      setText("serverSettingsStatus", result.restart_required ? "저장 완료. 앱을 재시작하면 접속 범위와 포트가 적용됩니다." : "저장 완료");
      await loadServerSettings();
    } catch (err) {
      setText("serverSettingsStatus", err.message);
    }
  }

  if (event.target.id === "drugSearchBtn") {
    const q = encodeURIComponent($("drugQuery").value.trim());
    const rows = await api(`/api/drugs/search?q=${q}`);
    renderTable($("drugResults"), rows, [
      { key: "code", label: "코드" },
      { key: "name", label: "약품명" },
      { key: "component", label: "성분" },
      { key: "drug_type", label: "구분" },
    ]);
  }

  if (event.target.id === "loadStocksBtn") {
    setText("stocksStatus", "전체 재고 조회 중...");
    $("stocksResults").innerHTML = "";
    try {
      const rows = await api("/api/stocks");
      state.stockRows = rows;
      $("stockQuery").disabled = false;
      $("stockQuery").placeholder = "약품명, 성분명, 코드 검색";
      renderStocks();
    } catch (err) {
      setText("stocksStatus", err.message);
    }
  }

  if (event.target.id === "downloadStocksBtn") {
    setText("stocksStatus", "XLSX 파일 생성 중...");
    try {
      await downloadFile("/api/stocks.xlsx", "drug_stocks.xlsx");
      setText("stocksStatus", "XLSX 다운로드를 시작했습니다.");
    } catch (err) {
      setText("stocksStatus", err.message);
    }
  }

  if (event.target.id === "loadUsageBtn") {
    const query = usageQuery();
    setText("usageStatus", "처방량 조회 중...");
    $("usageResults").innerHTML = "";
    try {
      const rows = await api(`/api/usage?${query}`);
      setText("usageStatus", rows.length === 0 ? "결과가 없습니다. 기간을 넓혀서 다시 조회하세요." : `${rows.length}개 코드 조회됨`);
      renderTable($("usageResults"), rows, usageColumns);
    } catch (err) {
      setText("usageStatus", err.message);
    }
  }

  if (event.target.id === "downloadUsageBtn") {
    const query = usageQuery();
    setText("usageStatus", "XLSX 파일 생성 중...");
    try {
      await downloadFile(`/api/usage.xlsx?${query}`, "drug_usage.xlsx");
      setText("usageStatus", "XLSX 다운로드를 시작했습니다.");
    } catch (err) {
      setText("usageStatus", err.message);
    }
  }

  if (event.target.id === "loadCodeStockBtn") {
    const code = $("userCode").value.trim();
    if (!code) {
      setText("codeStatus", "사용자 코드를 입력하세요.");
      return;
    }
    setText("codeStatus", "코드별 재고 조회 중...");
    $("codeStockResults").innerHTML = "";
    try {
      const row = await api(`/api/user-codes/${encodeURIComponent(code)}/stock`);
      setText("codeStatus", "재고 조회 완료");
      renderTable($("codeStockResults"), [row], stockColumns);
    } catch (err) {
      setText("codeStatus", err.message);
    }
  }

  if (event.target.id === "loadCodeUsageBtn") {
    const code = $("userCode").value.trim();
    if (!code) {
      setText("codeStatus", "사용자 코드를 입력하세요.");
      return;
    }
    const query = dateQuery("codeUsageDays", "codeUsageFrom", "codeUsageTo");
    setText("codeStatus", "코드별 처방량 조회 중...");
    $("codeUsageResults").innerHTML = "";
    try {
      const row = await api(`/api/user-codes/${encodeURIComponent(code)}/usage?${query}`);
      setText("codeStatus", row.order_count === 0 ? "결과가 없습니다. 기간을 넓혀서 다시 조회하세요." : "처방량 조회 완료");
      renderTable($("codeUsageResults"), [row], usageColumns);
    } catch (err) {
      setText("codeStatus", err.message);
    }
  }

  if (event.target.id === "loadPlanBtn") {
    const result = await api(`/api/inventory/order-plan?${planQuery()}`);
    const rows = (result.rows || []).filter(row => Number(row.recommended_order_qty) > 0);
    $("planSummary").innerHTML = `
      <div class="card"><span>주문 필요</span><strong>${result.summary?.need_count ?? 0}</strong></div>
      <div class="card"><span>긴급</span><strong>${result.summary?.urgent_count ?? 0}</strong></div>
      <div class="card"><span>권장주문 합계</span><strong>${result.summary?.recommended_total ?? 0}</strong></div>
    `;
    renderTable($("planResults"), rows, [
      { key: "medfee_code", label: "약품코드" },
      { key: "insurance_code", label: "보험코드" },
      { key: "representative_name", label: "약품명" },
      { key: "usage_qty", label: "기간처방량" },
      { key: "target_stock_qty", label: "비축필요량" },
      { key: "current_stock_qty", label: "현재재고" },
      { key: "recommended_order_qty", label: "주문필요수량" },
    ]);
  }

  if (event.target.id === "downloadPlanBtn") {
    setText("planStatus", "XLSX 파일 생성 중...");
    try {
      await downloadFile(`/api/inventory/order-plan.xlsx?${planQuery()}`, "drug_order_plan.xlsx");
      setText("planStatus", "XLSX 다운로드를 시작했습니다.");
    } catch (err) {
      setText("planStatus", err.message);
    }
  }
});

init().catch(err => {
  $("statusLine").textContent = err.message;
});
