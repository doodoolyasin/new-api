package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"ai-gateway/models"
)

func (h *APIHandler) RegisterDashboardRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", h.handleDashboard)
	mux.HandleFunc("/dashboard", h.handleDashboard)
	mux.HandleFunc("/admin", h.handleDashboard)

	// Web API endpoints for Dashboard
	mux.HandleFunc("/api/v1/admin/overview", h.handleAdminOverview)
	mux.HandleFunc("/api/v1/admin/keys/all", h.handleAdminAllKeys)
	mux.HandleFunc("/api/v1/admin/pricing", h.handleAdminPricing)
	mux.HandleFunc("/api/v1/admin/coupons", h.handleAdminCoupons)
}

func (h *APIHandler) handleAdminOverview(w http.ResponseWriter, r *http.Request) {
	if !h.verifyAdmin(r) {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Admin key required")
		return
	}
	stats, err := h.db.GetSystemStats()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

func (h *APIHandler) handleAdminAllKeys(w http.ResponseWriter, r *http.Request) {
	if !h.verifyAdmin(r) {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Admin key required")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	keys, err := h.db.GetAllVirtualKeys(limit, offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(keys)
}

func (h *APIHandler) handleAdminPricing(w http.ResponseWriter, r *http.Request) {
	if !h.verifyAdmin(r) {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Admin key required")
		return
	}
	if r.Method == http.MethodGet {
		list, err := h.db.GetAllModelPricing()
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)
		return
	}
	if r.Method == http.MethodPut || r.Method == http.MethodPost {
		var p models.ModelPricing
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			h.writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if err := h.db.UpdateModelPricing(&p); err != nil {
			h.writeError(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		return
	}
}

func (h *APIHandler) handleAdminCoupons(w http.ResponseWriter, r *http.Request) {
	if !h.verifyAdmin(r) {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Admin key required")
		return
	}
	if r.Method == http.MethodGet {
		list, err := h.db.GetAllCoupons()
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)
		return
	}
}

func (h *APIHandler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/dashboard" && r.URL.Path != "/admin" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(dashboardHTML))
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="fa" dir="rtl">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>داشبورد مدیریت AI Key Gateway</title>
  <link href="https://fonts.googleapis.com/css2?family=Vazirmatn:wght@300;400;500;700;900&display=swap" rel="stylesheet">
  <script src="https://cdn.tailwindcss.com"></script>
  <style>
    body { font-family: 'Vazirmatn', sans-serif; }
    .glass { background: rgba(30, 41, 59, 0.7); backdrop-filter: blur(12px); border: 1px solid rgba(255, 255, 255, 0.08); }
    .glass-card { background: rgba(15, 23, 42, 0.6); backdrop-filter: blur(8px); border: 1px solid rgba(255, 255, 255, 0.05); }
  </style>
</head>
<body class="bg-slate-950 text-slate-100 min-h-screen flex flex-col">

  <!-- Top Navbar -->
  <header class="glass sticky top-0 z-50 px-6 py-4 flex items-center justify-between border-b border-slate-800">
    <div class="flex items-center gap-3">
      <div class="w-10 h-10 rounded-xl bg-gradient-to-tr from-cyan-500 to-blue-600 flex items-center justify-center font-bold text-xl shadow-lg shadow-cyan-500/20">
        AI
      </div>
      <div>
        <h1 class="font-black text-lg text-white">AI Key Linker Gateway</h1>
        <div class="flex items-center gap-2 text-xs text-slate-400">
          <span class="inline-block w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
          <span>وضعیت سرور: آنلاین (Golang Core)</span>
        </div>
      </div>
    </div>

    <!-- Admin Key Input -->
    <div class="flex items-center gap-3">
      <div class="flex items-center bg-slate-900 border border-slate-700/80 rounded-xl px-3 py-1.5 focus-within:border-cyan-500 transition">
        <span class="text-xs text-slate-400 ml-2">رمز ادمین:</span>
        <input type="password" id="adminKeyInput" placeholder="ادمین کی..." class="bg-transparent text-xs text-cyan-300 focus:outline-none w-36" />
        <button onclick="saveAdminKey()" class="text-xs bg-cyan-600 hover:bg-cyan-500 text-white px-2 py-1 rounded-lg transition mr-2">ذخیره</button>
      </div>
    </div>
  </header>

  <div class="flex flex-1 overflow-hidden">
    <!-- Sidebar Navigation -->
    <aside class="w-64 glass border-l border-slate-800 flex flex-col p-4 gap-2">
      <button onclick="switchTab('overview')" id="btn-overview" class="nav-btn flex items-center gap-3 px-4 py-3 rounded-xl text-sm font-semibold transition bg-cyan-600/20 text-cyan-400 border border-cyan-500/30">
        📊 داشبورد و آمار کل
      </button>
      <button onclick="switchTab('keys')" id="btn-keys" class="nav-btn flex items-center gap-3 px-4 py-3 rounded-xl text-sm font-semibold text-slate-400 hover:text-white hover:bg-slate-800/50 transition">
        🔑 کلیدهای مجازی (Virtual Keys)
      </button>
      <button onclick="switchTab('providers')" id="btn-providers" class="nav-btn flex items-center gap-3 px-4 py-3 rounded-xl text-sm font-semibold text-slate-400 hover:text-white hover:bg-slate-800/50 transition">
        📡 استخر ارائه‌دهندگان (Providers)
      </button>
      <button onclick="switchTab('pricing')" id="btn-pricing" class="nav-btn flex items-center gap-3 px-4 py-3 rounded-xl text-sm font-semibold text-slate-400 hover:text-white hover:bg-slate-800/50 transition">
        💰 ضرایب قیمت مدل‌ها
      </button>
      <button onclick="switchTab('coupons')" id="btn-coupons" class="nav-btn flex items-center gap-3 px-4 py-3 rounded-xl text-sm font-semibold text-slate-400 hover:text-white hover:bg-slate-800/50 transition">
        🎟 کدهای هدیه (Coupons)
      </button>
    </aside>

    <!-- Main Content Area -->
    <main class="flex-1 overflow-y-auto p-8 space-y-6">

      <!-- TAB 1: OVERVIEW -->
      <section id="tab-overview" class="space-y-6">
        <h2 class="text-xl font-bold text-white">نمای کلی سیستم</h2>
        <div class="grid grid-cols-1 md:grid-cols-4 gap-5">
          <div class="glass-card p-5 rounded-2xl border-r-4 border-cyan-500">
            <span class="text-xs text-slate-400 font-medium">کلیدهای فعال / کل</span>
            <div class="text-2xl font-black text-white mt-2" id="stat-keys">...</div>
          </div>
          <div class="glass-card p-5 rounded-2xl border-r-4 border-indigo-500">
            <span class="text-xs text-slate-400 font-medium">سهمیه کل توزیع‌شده</span>
            <div class="text-2xl font-black text-white mt-2" id="stat-quota-total">...</div>
          </div>
          <div class="glass-card p-5 rounded-2xl border-r-4 border-pink-500">
            <span class="text-xs text-slate-400 font-medium">توکن‌های مصرف‌شده</span>
            <div class="text-2xl font-black text-white mt-2" id="stat-quota-consumed">...</div>
          </div>
          <div class="glass-card p-5 rounded-2xl border-r-4 border-emerald-500">
            <span class="text-xs text-slate-400 font-medium">ارائه‌دهندگان سالم</span>
            <div class="text-2xl font-black text-white mt-2" id="stat-providers">...</div>
          </div>
        </div>

        <div class="glass-card p-6 rounded-2xl space-y-4">
          <h3 class="font-bold text-white text-base">اطلاعات اتصال و اندپوینت کلاینت‌ها</h3>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs">
            <div class="bg-slate-900/80 p-4 rounded-xl border border-slate-800">
              <span class="text-slate-400 block mb-1">OpenAI Compatible Base URL:</span>
              <code class="text-cyan-400 font-mono select-all">http://95.182.80.85:8085/v1</code>
            </div>
            <div class="bg-slate-900/80 p-4 rounded-xl border border-slate-800">
              <span class="text-slate-400 block mb-1">Health Check URL:</span>
              <code class="text-emerald-400 font-mono select-all">http://95.182.80.85:8085/healthz</code>
            </div>
          </div>
        </div>
      </section>

      <!-- TAB 2: VIRTUAL KEYS -->
      <section id="tab-keys" class="hidden space-y-6">
        <div class="flex items-center justify-between">
          <h2 class="text-xl font-bold text-white">مدیریت کلیدهای مجازی</h2>
          <button onclick="openCreateKeyModal()" class="bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-bold px-4 py-2.5 rounded-xl transition flex items-center gap-2 shadow-lg shadow-cyan-600/20">
            ➕ ساخت کلید جدید
          </button>
        </div>

        <div class="glass-card rounded-2xl overflow-hidden border border-slate-800">
          <div class="overflow-x-auto">
            <table class="w-full text-right text-xs text-slate-300">
              <thead class="bg-slate-900/90 text-slate-400 font-bold border-b border-slate-800">
                <tr>
                  <th class="p-3.5">شناسه کلید</th>
                  <th class="p-3.5">کاربر تلگرام</th>
                  <th class="p-3.5">پلن</th>
                  <th class="p-3.5">سهمیه باقیمانده</th>
                  <th class="p-3.5">سرعت (RPM)</th>
                  <th class="p-3.5">همزمانی</th>
                  <th class="p-3.5">آی‌پی‌ها</th>
                  <th class="p-3.5">وضعیت</th>
                  <th class="p-3.5 text-center">عملیات</th>
                </tr>
              </thead>
              <tbody id="keysTableBody" class="divide-y divide-slate-800/60">
                <tr><td colspan="9" class="p-4 text-center text-slate-500">در حال دریافت اطلاعات...</td></tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>

      <!-- TAB 3: PROVIDERS -->
      <section id="tab-providers" class="hidden space-y-6">
        <div class="flex items-center justify-between">
          <h2 class="text-xl font-bold text-white">استخر ارائه‌دهندگان بالادست (Upstream Accounts)</h2>
          <button onclick="openAddProviderModal()" class="bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-bold px-4 py-2.5 rounded-xl transition flex items-center gap-2">
            ➕ افزودن ارائه‌دهنده
          </button>
        </div>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4" id="providersContainer">
          <div class="p-4 text-slate-500">در حال بارگذاری...</div>
        </div>
      </section>

      <!-- TAB 4: PRICING -->
      <section id="tab-pricing" class="hidden space-y-6">
        <h2 class="text-xl font-bold text-white">ضرایب محاسبه مصرف مدل‌ها (Pricing Multipliers)</h2>
        <div class="glass-card rounded-2xl overflow-hidden border border-slate-800 p-6">
          <table class="w-full text-right text-xs text-slate-300">
            <thead class="bg-slate-900/90 text-slate-400 font-bold border-b border-slate-800">
              <tr>
                <th class="p-3.5">نام مدل</th>
                <th class="p-3.5">ضریب ورودی</th>
                <th class="p-3.5">ضریب خروجی</th>
                <th class="p-3.5">حداقل کسر</th>
                <th class="p-3.5">وضعیت</th>
                <th class="p-3.5 text-center">عملیات</th>
              </tr>
            </thead>
            <tbody id="pricingTableBody" class="divide-y divide-slate-800/60"></tbody>
          </table>
        </div>
      </section>

      <!-- TAB 5: COUPONS -->
      <section id="tab-coupons" class="hidden space-y-6">
        <div class="flex items-center justify-between">
          <h2 class="text-xl font-bold text-white">کدهای هدیه و شارژ (Coupons)</h2>
          <button onclick="openCreateCouponModal()" class="bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-bold px-4 py-2.5 rounded-xl transition">
            ➕ ساخت کد جدید
          </button>
        </div>
        <div class="glass-card rounded-2xl overflow-hidden border border-slate-800">
          <table class="w-full text-right text-xs text-slate-300">
            <thead class="bg-slate-900/90 text-slate-400 font-bold border-b border-slate-800">
              <tr>
                <th class="p-3.5">کد</th>
                <th class="p-3.5">مقدار توکن</th>
                <th class="p-3.5">روزهای اعتبار</th>
                <th class="p-3.5">تعداد مصرف‌شده / سقف</th>
                <th class="p-3.5">وضعیت</th>
              </tr>
            </thead>
            <tbody id="couponsTableBody" class="divide-y divide-slate-800/60"></tbody>
          </table>
        </div>
      </section>

    </main>
  </div>

  <!-- Create Key Modal -->
  <div id="modalCreateKey" class="fixed inset-0 bg-black/80 backdrop-blur-sm hidden items-center justify-center p-4 z-50">
    <div class="glass-card bg-slate-900 p-6 rounded-2xl max-w-md w-full space-y-4 border border-slate-700">
      <h3 class="text-base font-bold text-white">صدور کلید مجازی جدید</h3>
      <div class="space-y-3 text-xs">
        <div>
          <label class="block text-slate-400 mb-1">شناسه کاربر تلگرام:</label>
          <input type="text" id="newKeyOwner" class="w-full bg-slate-950 border border-slate-700 rounded-lg p-2.5 text-white" placeholder="مثال: 1099934856">
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="block text-slate-400 mb-1">سهمیه توکن:</label>
            <input type="number" id="newKeyQuota" class="w-full bg-slate-950 border border-slate-700 rounded-lg p-2.5 text-white" value="50000">
          </div>
          <div>
            <label class="block text-slate-400 mb-1">اعتبار (روز):</label>
            <input type="number" id="newKeyDays" class="w-full bg-slate-950 border border-slate-700 rounded-lg p-2.5 text-white" value="30">
          </div>
        </div>
        <div class="grid grid-cols-3 gap-3">
          <div>
            <label class="block text-slate-400 mb-1">سرعت (RPM):</label>
            <input type="number" id="newKeyRPM" class="w-full bg-slate-950 border border-slate-700 rounded-lg p-2.5 text-white" value="10">
          </div>
          <div>
            <label class="block text-slate-400 mb-1">همزمانی:</label>
            <input type="number" id="newKeyConc" class="w-full bg-slate-950 border border-slate-700 rounded-lg p-2.5 text-white" value="2">
          </div>
          <div>
            <label class="block text-slate-400 mb-1">سقف IP:</label>
            <input type="number" id="newKeyIPs" class="w-full bg-slate-950 border border-slate-700 rounded-lg p-2.5 text-white" value="2">
          </div>
        </div>
      </div>
      <div class="flex justify-end gap-2 pt-2">
        <button onclick="closeModal('modalCreateKey')" class="px-4 py-2 bg-slate-800 text-slate-300 text-xs rounded-lg">انصراف</button>
        <button onclick="submitCreateKey()" class="px-4 py-2 bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-bold rounded-lg">صدور کلید</button>
      </div>
    </div>
  </div>

  <script>
    let currentAdminKey = localStorage.getItem('gateway_admin_key') || 'adm_secret_key_linker_2026_x';
    document.getElementById('adminKeyInput').value = currentAdminKey;

    function saveAdminKey() {
      currentAdminKey = document.getElementById('adminKeyInput').value.trim();
      localStorage.setItem('gateway_admin_key', currentAdminKey);
      alert('کلید ادمین ذخیره شد.');
      loadOverview();
    }

    function authHeaders() {
      return {
        'Authorization': 'Bearer ' + currentAdminKey,
        'Content-Type': 'application/json'
      };
    }

    function switchTab(tab) {
      ['overview', 'keys', 'providers', 'pricing', 'coupons'].forEach(t => {
        document.getElementById('tab-' + t).classList.add('hidden');
        document.getElementById('btn-' + t).className = 'nav-btn flex items-center gap-3 px-4 py-3 rounded-xl text-sm font-semibold text-slate-400 hover:text-white hover:bg-slate-800/50 transition';
      });
      document.getElementById('tab-' + tab).classList.remove('hidden');
      document.getElementById('btn-' + tab).className = 'nav-btn flex items-center gap-3 px-4 py-3 rounded-xl text-sm font-semibold transition bg-cyan-600/20 text-cyan-400 border border-cyan-500/30';

      if (tab === 'overview') loadOverview();
      if (tab === 'keys') loadKeys();
      if (tab === 'providers') loadProviders();
      if (tab === 'pricing') loadPricing();
      if (tab === 'coupons') loadCoupons();
    }

    async function loadOverview() {
      try {
        const res = await fetch('/api/v1/admin/overview', { headers: authHeaders() });
        if (res.ok) {
          const d = await res.json();
          document.getElementById('stat-keys').textContent = (d.active_keys || 0) + ' / ' + (d.total_keys || 0);
          document.getElementById('stat-quota-total').textContent = Number(d.total_quota_issued || 0).toLocaleString();
          document.getElementById('stat-quota-consumed').textContent = Number(d.total_quota_consumed || 0).toLocaleString();
          document.getElementById('stat-providers').textContent = (d.active_providers || 0) + ' / ' + (d.total_providers || 0);
        }
      } catch (e) { console.error(e); }
    }

    async function loadKeys() {
      const tbody = document.getElementById('keysTableBody');
      tbody.innerHTML = '<tr><td colspan="9" class="p-4 text-center text-slate-500">در حال دریافت...</td></tr>';
      try {
        const res = await fetch('/api/v1/admin/keys/all?limit=50', { headers: authHeaders() });
        if (res.ok) {
          const keys = await res.json();
          if (!keys || keys.length === 0) {
            tbody.innerHTML = '<tr><td colspan="9" class="p-4 text-center text-slate-500">هیچ کلیدی یافت نشد.</td></tr>';
            return;
          }
          tbody.innerHTML = keys.map(k => {
            const stColor = k.status === 'ACTIVE' ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' : 'bg-red-500/10 text-red-400 border-red-500/20';
            return '<tr class="hover:bg-slate-900/50 transition">' +
              '<td class="p-3 font-mono text-cyan-300 font-bold">' + k.key_id + '</td>' +
              '<td class="p-3 font-mono">' + k.owner_id + '</td>' +
              '<td class="p-3"><span class="px-2 py-0.5 rounded-full text-[10px] bg-slate-800">' + k.plan_id + '</span></td>' +
              '<td class="p-3 font-mono font-bold">' + Number(k.remain_quota).toLocaleString() + ' <span class="text-slate-500">/ ' + Number(k.total_quota).toLocaleString() + '</span></td>' +
              '<td class="p-3 font-mono">' + k.rpm_limit + '</td>' +
              '<td class="p-3 font-mono">' + k.max_concurrency + '</td>' +
              '<td class="p-3 font-mono">' + k.current_ip_count + ' / ' + k.max_allowed_ips + '</td>' +
              '<td class="p-3"><span class="px-2 py-0.5 rounded-full text-[10px] border ' + stColor + '">' + k.status + '</span></td>' +
              '<td class="p-3 text-center flex items-center justify-center gap-1.5">' +
                '<button onclick="regenKey(\'' + k.key_id + '\')" class="p-1 px-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded text-[11px]">تعویض</button>' +
                '<button onclick="addQuotaPrompt(\'' + k.key_id + '\')" class="p-1 px-2 bg-cyan-600/30 hover:bg-cyan-600 text-cyan-300 rounded text-[11px]">شارژ</button>' +
                '<button onclick="revokeKey(\'' + k.key_id + '\')" class="p-1 px-2 bg-red-600/20 hover:bg-red-600 text-red-300 rounded text-[11px]">ابطال</button>' +
              '</td>' +
            '</tr>';
          }).join('');
        }
      } catch (e) { console.error(e); }
    }

    async function loadProviders() {
      const container = document.getElementById('providersContainer');
      container.innerHTML = '<div class="p-4 text-slate-500">در حال دریافت...</div>';
      try {
        const res = await fetch('/api/v1/providers', { headers: authHeaders() });
        if (res.ok) {
          const list = await res.json();
          container.innerHTML = list.map(p => {
            const stColor = p.status === 'ACTIVE' ? 'text-emerald-400' : 'text-amber-400';
            return '<div class="glass-card p-5 rounded-2xl border border-slate-800 space-y-2">' +
              '<div class="flex items-center justify-between">' +
                '<span class="font-bold text-white text-sm">' + p.name + '</span>' +
                '<span class="text-xs font-bold ' + stColor + '">● ' + p.status + '</span>' +
              '</div>' +
              '<div class="text-xs text-slate-400">آدرس: <code class="text-slate-300">' + p.base_url + '</code></div>' +
              '<div class="text-xs text-slate-400">مدل‌های مجاز: <code class="text-cyan-400">' + p.supported_models + '</code></div>' +
              '<div class="flex items-center gap-4 text-xs pt-2 text-slate-400">' +
                '<span>اولویت: <b class="text-white">' + p.priority + '</b></span>' +
                '<span>خطاهای پیاپی: <b class="text-rose-400">' + p.consecutive_failures + '</b></span>' +
                '<span>تأخیر میانگین: <b class="text-emerald-400">' + (p.avg_latency_ms || 0) + 'ms</b></span>' +
              '</div>' +
            '</div>';
          }).join('');
        }
      } catch (e) { console.error(e); }
    }

    async function loadPricing() {
      const tbody = document.getElementById('pricingTableBody');
      try {
        const res = await fetch('/api/v1/admin/pricing', { headers: authHeaders() });
        if (res.ok) {
          const list = await res.json();
          tbody.innerHTML = list.map(p => {
            return '<tr>' +
              '<td class="p-3 font-bold text-white">' + p.model_name + '</td>' +
              '<td class="p-3 font-mono"><input type="number" step="0.5" id="in_mult_' + p.id + '" value="' + p.input_multiplier + '" class="w-20 bg-slate-900 border border-slate-700 p-1 rounded text-white"></td>' +
              '<td class="p-3 font-mono"><input type="number" step="0.5" id="out_mult_' + p.id + '" value="' + p.output_multiplier + '" class="w-20 bg-slate-900 border border-slate-700 p-1 rounded text-white"></td>' +
              '<td class="p-3 font-mono">' + p.min_charge + '</td>' +
              '<td class="p-3">' + (p.enabled ? '🟢 فعال' : '🔴 غیرفعال') + '</td>' +
              '<td class="p-3 text-center"><button onclick="savePricing(' + p.id + ', \'' + p.model_name + '\')" class="bg-cyan-600 hover:bg-cyan-500 text-white px-3 py-1 rounded text-xs">ذخیره</button></td>' +
            '</tr>';
          }).join('');
        }
      } catch (e) { console.error(e); }
    }

    async function savePricing(id, model_name) {
      const inMult = parseFloat(document.getElementById('in_mult_' + id).value);
      const outMult = parseFloat(document.getElementById('out_mult_' + id).value);
      const res = await fetch('/api/v1/admin/pricing', {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({ id: id, model_name: model_name, input_multiplier: inMult, output_multiplier: outMult, min_charge: 1, enabled: true })
      });
      if (res.ok) alert('ضریب مدل به‌روزرسانی شد.');
    }

    async function loadCoupons() {
      const tbody = document.getElementById('couponsTableBody');
      try {
        const res = await fetch('/api/v1/admin/coupons', { headers: authHeaders() });
        if (res.ok) {
          const list = await res.json();
          tbody.innerHTML = list.map(c => {
            return '<tr>' +
              '<td class="p-3 font-mono font-bold text-cyan-400">' + c.code + '</td>' +
              '<td class="p-3 font-mono">' + Number(c.quota).toLocaleString() + '</td>' +
              '<td class="p-3 font-mono">' + c.days + ' روز</td>' +
              '<td class="p-3 font-mono">' + c.used_count + ' / ' + c.max_uses + '</td>' +
              '<td class="p-3">' + (c.is_active ? '🟢 فعال' : '🔴 غیرفعال') + '</td>' +
            '</tr>';
          }).join('');
        }
      } catch (e) { console.error(e); }
    }

    function openCreateKeyModal() { document.getElementById('modalCreateKey').classList.remove('hidden'); document.getElementById('modalCreateKey').classList.add('flex'); }
    function closeModal(id) { document.getElementById(id).classList.add('hidden'); document.getElementById(id).classList.remove('flex'); }

    async function submitCreateKey() {
      const owner = document.getElementById('newKeyOwner').value.trim();
      const quota = parseInt(document.getElementById('newKeyQuota').value);
      const days = parseInt(document.getElementById('newKeyDays').value);
      const rpm = parseInt(document.getElementById('newKeyRPM').value);
      const conc = parseInt(document.getElementById('newKeyConc').value);
      const ips = parseInt(document.getElementById('newKeyIPs').value);

      if (!owner) return alert('شناسه کاربر تلگرام الزامی است.');

      const res = await fetch('/api/v1/keys', {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({ owner_id: owner, plan_id: 'CUSTOM', quota: quota, days: days, rpm_limit: rpm, max_concurrency: conc, max_allowed_ips: ips, allowed_models: '*' })
      });

      if (res.ok) {
        const k = await res.json();
        alert('کلید صادر شد:\n' + k.key_secret);
        closeModal('modalCreateKey');
        loadKeys();
      } else {
        alert('خطا در صدور کلید');
      }
    }

    async function regenKey(keyId) {
      if (!confirm('آیا از ابطال و تعویض این کلید اطمینان دارید؟')) return;
      const res = await fetch('/api/v1/keys/regenerate', {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({ old_key_id: keyId, actor: 'dashboard_admin' })
      });
      if (res.ok) {
        const k = await res.json();
        alert('کلید جدید صادر شد:\n' + k.key_secret);
        loadKeys();
      }
    }

    async function addQuotaPrompt(keyId) {
      const amount = prompt('مقدار توکن جهت افزودن به این کلید:');
      if (!amount) return;
      const res = await fetch('/api/v1/keys/quota', {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({ key_id: keyId, amount: parseInt(amount), type: 'ADMIN_ADJUSTMENT', details: 'Added via Dashboard', actor: 'dashboard' })
      });
      if (res.ok) {
        alert('سهمیه با موفقیت افزوده شد.');
        loadKeys();
      }
    }

    async function revokeKey(keyId) {
      if (!confirm('آیا مطمئن هستید که می‌خواهید کلید را باطل کنید؟')) return;
      const res = await fetch('/api/v1/keys/revoke', {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({ key_id: keyId, actor: 'dashboard_admin' })
      });
      if (res.ok) {
        alert('کلید باطل شد.');
        loadKeys();
      }
    }

    // Initialize
    loadOverview();
  </script>
</body>
</html>`
