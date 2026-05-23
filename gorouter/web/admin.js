(function () {
    const API_BASE = "";
    let token = localStorage.getItem("gorouter_token");

    const $ = (s) => document.querySelector(s);
    const $$ = (s) => document.querySelectorAll(s);

    function toast(msg, type = "error") {
        const el = document.createElement("div");
        el.className = "toast toast-" + type;
        el.textContent = msg;
        $("#toast-container").appendChild(el);
        setTimeout(() => el.remove(), 3000);
    }

    async function api(path, opts = {}) {
        const headers = { "Content-Type": "application/json", ...(opts.headers || {}) };
        if (token) headers["Authorization"] = "Bearer " + token;
        const res = await fetch(API_BASE + path, { ...opts, headers });
        if (res.status === 401) { logout(); throw new Error("Unauthorized"); }
        const body = await res.json().catch(() => ({}));
        if (!res.ok) {
            throw new Error(body.error || body.message || "Request failed (" + res.status + ")");
        }
        return body;
    }

    function esc(s) {
        if (s == null) return "";
        const d = document.createElement("div");
        d.textContent = String(s);
        return d.innerHTML;
    }

    function showPage(id) {
        $$(".page").forEach((p) => p.classList.add("hidden"));
        $("#" + id).classList.remove("hidden");
    }

    function showSection(route) {
        $$(".section").forEach((s) => s.classList.add("hidden"));
        const sec = $("#section-" + route);
        if (sec) sec.classList.remove("hidden");
        $$(".nav-link").forEach((l) => l.classList.remove("active"));
        const link = $(`.nav-link[data-route="${route}"]`);
        if (link) link.classList.add("active");
    }

    function logout() {
        token = null;
        localStorage.removeItem("gorouter_token");
        showPage("login-page");
    }

    function router() {
        const hash = location.hash.replace("#", "") || "/dashboard";
        const route = hash.replace("/", "").split("/")[0] || "dashboard";
        if (!token) { showPage("login-page"); return; }
        showPage("admin-page");
        showSection(route);
        loadSection(route);
    }

    function openModal(title, bodyHtml) {
        $("#modal-title").textContent = title;
        $("#modal-body").innerHTML = bodyHtml;
        $("#modal-overlay").classList.remove("hidden");
    }

    function closeModal() {
        $("#modal-overlay").classList.add("hidden");
        $("#modal-body").innerHTML = "";
    }

    function loadSection(route) {
        switch (route) {
            case "dashboard": loadDashboard(); break;
            case "users": loadUsers(); break;
            case "apikeys": loadApiKeys(); break;
            case "providers": loadProviders(); break;
            case "models": loadModels(); break;
            case "aliases": loadAliases(); break;
            case "combos": loadCombos(); break;
            case "quotas": loadQuotas(); break;
            case "usage": loadUsage(); break;
        }
    }

    async function loadDashboard() {
        try {
            const stats = await api("/admin/dashboard/stats");
            const cards = [
                { label: "Users", value: stats.total_users ?? 0 },
                { label: "API Keys", value: stats.total_api_keys ?? 0 },
                { label: "Providers", value: stats.total_providers ?? 0 },
                { label: "Models", value: stats.total_models ?? 0 },
                { label: "Requests Today", value: stats.usage_today?.total_requests ?? 0 },
                { label: "Tokens Today", value: stats.usage_today?.total_tokens ?? 0 },
                { label: "Cost Today", value: "$" + (stats.usage_today?.estimated_cost ?? 0).toFixed(4) },
                { label: "Avg Latency", value: (stats.usage_today?.avg_latency_ms ?? 0).toFixed(0) + "ms" },
            ];
            $("#stats-cards").innerHTML = cards.map((c) =>
                `<div class="stat-card"><div class="stat-label">${c.label}</div><div class="stat-value">${c.value}</div></div>`
            ).join("");
        } catch (e) {
            toast("Failed to load stats: " + e.message);
        }

        try {
            const data = await api("/admin/dashboard/recent-activity");
            const items = data.items || [];
            const tbody = $("#activity-table tbody");
            tbody.innerHTML = items.map((a) => `<tr>
                <td>${esc(new Date(a.created_at).toLocaleString())}</td>
                <td>${esc(a.user_id)}</td>
                <td>${esc(a.model)}</td>
                <td>${esc(a.provider)}</td>
                <td><span class="badge badge-${a.status_code < 400 ? 'active' : 'error'}">${a.status_code}</span></td>
                <td>${a.tokens}</td>
                <td>$${a.estimated_cost.toFixed(4)}</td>
                <td>${a.latency_ms}ms</td>
            </tr>`).join("");
        } catch {}
    }

    async function loadUsers() {
        try {
            const res = await api("/admin/users");
            const users = Array.isArray(res.data) ? res.data :
                Array.isArray(res) ? res : [];
            const tbody = $("#users-table tbody");
            tbody.innerHTML = users.map((u) => `<tr>
                <td>${esc(u.email)}</td>
                <td>${esc(u.name || "-")}</td>
                <td><span class="badge badge-${u.role === 'admin' ? 'enabled' : 'disabled'}">${esc(u.role)}</span></td>
                <td><span class="badge badge-${u.status || 'active'}">${esc(u.status || "active")}</span></td>
                <td>${esc(u.created_at ? new Date(u.created_at).toLocaleDateString() : "-")}</td>
                <td class="actions-cell">
                    <button class="btn btn-sm btn-secondary" onclick="window._editUser('${u.id}')">Edit</button>
                    ${u.status !== "suspended"
                        ? `<button class="btn btn-sm btn-danger" onclick="window._suspendUser('${u.id}')">Suspend</button>`
                        : `<button class="btn btn-sm btn-primary" onclick="window._activateUser('${u.id}')">Activate</button>`}
                </td>
            </tr>`).join("");
        } catch (e) {
            toast("Failed to load users: " + e.message);
        }
    }

    async function loadApiKeys() {
        try {
            const res = await api("/admin/api-keys");
            const keys = Array.isArray(res.keys) ? res.keys :
                Array.isArray(res.data) ? res.data :
                Array.isArray(res) ? res : [];
            const tbody = $("#apikeys-table tbody");
            tbody.innerHTML = keys.map((k) => `<tr>
                <td>${esc(k.name)}</td>
                <td>${esc(k.user_id)}</td>
                <td><code>${esc(k.key_prefix || "-")}</code></td>
                <td>${(k.scopes || []).map((s) => `<span class="badge badge-disabled">${esc(s)}</span>`).join(" ")}</td>
                <td><span class="badge badge-${k.status === 'revoked' ? 'revoked' : 'active'}">${esc(k.status || "active")}</span></td>
                <td>${esc(k.created_at ? new Date(k.created_at).toLocaleDateString() : "-")}</td>
                <td class="actions-cell">
                    ${k.status !== "revoked" ? `<button class="btn btn-sm btn-danger" onclick="window._revokeKey('${k.id}')">Revoke</button>` : ""}
                    ${k.status !== "revoked" ? `<button class="btn btn-sm btn-secondary" onclick="window._rotateKey('${k.id}')">Rotate</button>` : ""}
                </td>
            </tr>`).join("");
        } catch (e) {
            toast("Failed to load API keys: " + e.message);
        }
    }

    async function loadProviders() {
        try {
            const res = await api("/admin/providers");
            const providers = Array.isArray(res.data) ? res.data :
                Array.isArray(res) ? res : [];
            const tbody = $("#providers-table tbody");
            tbody.innerHTML = providers.map((p) => `<tr>
                <td>${esc(p.name)}</td>
                <td>${esc(p.provider || p.auth_type)}</td>
                <td>${esc(p.base_url || "-")}</td>
                <td>${p.priority}</td>
                <td><span class="badge badge-${p.status === 'active' ? 'active' : p.status === 'cooldown' ? 'cooldown' : 'error'}">${esc(p.status || "active")}</span></td>
                <td class="actions-cell">
                    <button class="btn btn-sm btn-secondary" onclick="window._editProvider('${p.id}')">Edit</button>
                    <button class="btn btn-sm btn-danger" onclick="window._deleteProvider('${p.id}')">Delete</button>
                </td>
            </tr>`).join("");
        } catch (e) {
            toast("Failed to load providers: " + e.message);
        }
    }

    async function loadModels() {
        try {
            const res = await api("/admin/models");
            const models = Array.isArray(res.data) ? res.data :
                Array.isArray(res) ? res : [];
            const tbody = $("#models-table tbody");
            tbody.innerHTML = models.map((m) => `<tr>
                <td>${esc(m.display_name || m.model_id)}</td>
                <td><code>${esc(m.model_id || m.id)}</code></td>
                <td>${esc(m.provider || m.provider_id)}</td>
                <td>${m.context_window ? m.context_window.toLocaleString() : "-"}</td>
                <td><span class="badge badge-${m.enabled || m.is_active ? 'enabled' : 'disabled'}">${m.enabled || m.is_active ? "Active" : "Disabled"}</span></td>
                <td>${m.supports_stream ? "Yes" : "No"}</td>
                <td class="actions-cell">
                    <button class="btn btn-sm btn-secondary" onclick="window._editModel('${m.id}')">Edit</button>
                    <button class="btn btn-sm btn-${m.enabled ? 'danger' : 'primary'}" onclick="window._toggleModel('${m.id}', ${!m.enabled})">${m.enabled ? "Disable" : "Enable"}</button>
                    <button class="btn btn-sm btn-danger" onclick="window._deleteModel('${m.id}')">Delete</button>
                </td>
            </tr>`).join("");
        } catch (e) {
            toast("Failed to load models: " + e.message);
        }
    }

    async function loadAliases() {
        try {
            const res = await api("/admin/aliases");
            const aliases = Array.isArray(res.data) ? res.data :
                Array.isArray(res) ? res : [];
            const tbody = $("#aliases-table tbody");
            tbody.innerHTML = aliases.map((a) => `<tr>
                <td>${esc(a.name)}</td>
                <td><code>${esc(a.target_id)}</code></td>
                <td>${esc(a.provider || "-")}</td>
                <td>${esc(a.description || "-")}</td>
                <td class="actions-cell">
                    <button class="btn btn-sm btn-secondary" onclick="window._editAlias('${a.id}')">Edit</button>
                    <button class="btn btn-sm btn-danger" onclick="window._deleteAlias('${a.id}')">Delete</button>
                </td>
            </tr>`).join("");
        } catch (e) {
            toast("Failed to load aliases: " + e.message);
        }
    }

    async function loadCombos() {
        try {
            const res = await api("/admin/combos");
            const combos = Array.isArray(res.data) ? res.data :
                Array.isArray(res) ? res : [];
            const tbody = $("#combos-table tbody");
            tbody.innerHTML = combos.map((c) => `<tr>
                <td>${esc(c.name)}</td>
                <td>${esc(c.description || "-")}</td>
                <td>${esc(c.user_id)}</td>
                <td>${c.items ? c.items.length : 0}</td>
                <td><span class="badge badge-${c.is_active ? 'active' : 'disabled'}">${c.is_active ? "Active" : "Inactive"}</span></td>
                <td class="actions-cell">
                    <button class="btn btn-sm btn-secondary" onclick="window._viewCombo('${c.id}')">View</button>
                </td>
            </tr>`).join("");
        } catch (e) {
            toast("Failed to load combos: " + e.message);
        }
    }

    async function loadQuotas() {
        try {
            const res = await api("/admin/quotas");
            const quotas = Array.isArray(res.data) ? res.data :
                Array.isArray(res) ? res : [];
            const tbody = $("#quotas-table tbody");
            tbody.innerHTML = quotas.map((q) => `<tr>
                <td>${esc(q.user_id)}</td>
                <td>${q.monthly_token_cap ? q.monthly_token_cap.toLocaleString() : "-"}</td>
                <td>${q.monthly_cost_cap ? "$" + q.monthly_cost_cap.toFixed(4) : "-"}</td>
                <td>${q.used_tokens ? q.used_tokens.toLocaleString() : 0}</td>
                <td>${q.used_cost ? "$" + q.used_cost.toFixed(4) : "$0.0000"}</td>
                <td>${q.reset_at ? esc(new Date(q.reset_at).toLocaleDateString()) : "-"}</td>
                <td class="actions-cell">
                    <button class="btn btn-sm btn-secondary" onclick="window._editQuota('${q.id}')">Edit</button>
                    <button class="btn btn-sm btn-primary" onclick="window._resetQuota('${q.id}')">Reset</button>
                    <button class="btn btn-sm btn-danger" onclick="window._deleteQuota('${q.id}')">Delete</button>
                </td>
            </tr>`).join("");
        } catch (e) {
            toast("Failed to load quotas: " + e.message);
        }
    }

    async function loadUsage() {
        try {
            const stats = await api("/admin/dashboard/stats");
            const u = stats.usage_today || {};
            const cards = [
                { label: "Total Requests", value: u.total_requests ?? 0 },
                { label: "Total Tokens", value: u.total_tokens ?? 0 },
                { label: "Estimated Cost", value: "$" + (u.estimated_cost ?? 0).toFixed(4) },
                { label: "Avg Latency", value: (u.avg_latency_ms ?? 0).toFixed(0) + "ms" },
            ];
            $("#usage-stats").innerHTML = cards.map((c) =>
                `<div class="stat-card"><div class="stat-label">${c.label}</div><div class="stat-value">${c.value}</div></div>`
            ).join("");
        } catch {}

        drawBarChart("usage-chart", []);
    }

    function drawBarChart(canvasId, data) {
        const canvas = document.getElementById(canvasId);
        if (!canvas) return;
        const ctx = canvas.getContext("2d");
        const container = canvas.parentElement;
        const w = container.clientWidth;
        const h = 300;
        canvas.width = w;
        canvas.height = h;

        ctx.fillStyle = "#16213e";
        ctx.fillRect(0, 0, w, h);

        if (!data.length) {
            ctx.fillStyle = "#8892a8";
            ctx.font = "14px sans-serif";
            ctx.textAlign = "center";
            ctx.fillText("No usage data available", w / 2, h / 2);
            return;
        }

        const values = data.map((d) => d.count || d.requests || d.value || 0);
        const labels = data.map((d) => d.date || d.label || "");
        const max = Math.max(...values, 1);
        const barW = Math.max(4, (w - 60) / data.length - 2);
        const chartH = h - 50;

        ctx.fillStyle = "#8892a8";
        ctx.font = "11px sans-serif";
        ctx.textAlign = "right";
        for (let i = 0; i <= 4; i++) {
            const y = chartH - (i / 4) * (chartH - 20);
            ctx.fillText(Math.round((max * i) / 4), 50, y + 4);
            ctx.strokeStyle = "#253a5e";
            ctx.beginPath();
            ctx.moveTo(55, y);
            ctx.lineTo(w, y);
            ctx.stroke();
        }

        values.forEach((v, i) => {
            const barH = (v / max) * (chartH - 20);
            const x = 60 + i * (barW + 2);
            const y = chartH - barH;
            ctx.fillStyle = "#4a9eff";
            ctx.fillRect(x, y, barW, barH);
        });

        ctx.fillStyle = "#8892a8";
        ctx.font = "11px sans-serif";
        ctx.textAlign = "center";
        data.forEach((_, i) => {
            if (i % Math.ceil(data.length / 8) === 0) {
                const x = 60 + i * (barW + 2) + barW / 2;
                ctx.fillText(labels[i], x, h - 5);
            }
        });
    }

    async function checkAuth() {
        if (!token) { showPage("login-page"); return; }
        try {
            showPage("admin-page");
            router();
        } catch {
            logout();
        }
    }

    window._editUser = async (id) => {
        try {
            const res = await api("/admin/users/" + id);
            const u = res.data || res;
            openModal("Edit User", `
                <form id="edit-user-form">
                    <input type="hidden" id="eu-id" value="${esc(u.id)}">
                    <div class="form-group"><label>Email</label><input type="email" id="eu-email" value="${esc(u.email)}" required></div>
                    <div class="form-group"><label>Name</label><input type="text" id="eu-name" value="${esc(u.name || "")}"></div>
                    <div class="form-group"><label>New Password (leave blank to keep)</label><input type="password" id="eu-password"></div>
                    <div class="form-group"><label>Role</label><select id="eu-role"><option value="user" ${u.role === 'user' ? 'selected' : ''}>User</option><option value="admin" ${u.role === 'admin' ? 'selected' : ''}>Admin</option></select></div>
                    <button type="submit" class="btn btn-primary">Save</button>
                </form>
            `);
            $("#edit-user-form").addEventListener("submit", async (e) => {
                e.preventDefault();
                try {
                    const body = { email: $("#eu-email").value, name: $("#eu-name").value, role: $("#eu-role").value };
                    const pw = $("#eu-password").value;
                    if (pw) body.password = pw;
                    await api("/admin/users/" + $("#eu-id").value, { method: "PATCH", body: JSON.stringify(body) });
                    closeModal();
                    toast("User updated", "success");
                    loadUsers();
                } catch (err) { toast(err.message); }
            });
        } catch (err) { toast(err.message); }
    };

    window._suspendUser = async (id) => {
        if (!confirm("Suspend this user?")) return;
        try {
            await api("/admin/users/" + id + "/suspend", { method: "POST" });
            toast("User suspended", "success");
            loadUsers();
        } catch (err) { toast(err.message); }
    };

    window._activateUser = async (id) => {
        try {
            await api("/admin/users/" + id + "/activate", { method: "POST" });
            toast("User activated", "success");
            loadUsers();
        } catch (err) { toast(err.message); }
    };

    window._revokeKey = async (id) => {
        if (!confirm("Revoke this API key? This cannot be undone.")) return;
        try {
            await api("/admin/api-keys/" + id + "/revoke", { method: "POST" });
            toast("Key revoked", "success");
            loadApiKeys();
        } catch (err) { toast(err.message); }
    };

    window._rotateKey = async (id) => {
        if (!confirm("Rotate this key? The old key will stop working.")) return;
        try {
            const res = await api("/admin/api-keys/" + id + "/rotate", { method: "POST" });
            const d = res.data || res;
            if (d.key) {
                openModal("Key Rotated", `
                    <div class="new-key-banner">
                        <p>Save this new key now - it will not be shown again:</p>
                        <code>${esc(d.key)}</code>
                    </div>
                    <button class="btn btn-primary" onclick="navigator.clipboard.writeText('${esc(d.key)}');this.textContent='Copied!'">Copy Key</button>
                `);
            }
            loadApiKeys();
        } catch (err) { toast(err.message); }
    };

    window._editProvider = async (id) => {
        try {
            const res = await api("/admin/providers/" + id);
            const p = res.data || res;
            openModal("Edit Provider", `
                <form id="edit-provider-form">
                    <input type="hidden" id="ep-id" value="${esc(p.id)}">
                    <div class="form-group"><label>Name</label><input type="text" id="ep-name" value="${esc(p.name)}" required></div>
                    <div class="form-group"><label>Provider Type</label><select id="ep-type">
                        <option value="openai" ${p.provider==='openai'?'selected':''}>OpenAI</option>
                        <option value="anthropic" ${p.provider==='anthropic'?'selected':''}>Anthropic</option>
                        <option value="openrouter" ${p.provider==='openrouter'?'selected':''}>OpenRouter</option>
                        <option value="custom" ${p.provider==='custom'?'selected':''}>Custom</option>
                    </select></div>
                    <div class="form-group"><label>Base URL</label><input type="url" id="ep-url" value="${esc(p.base_url || "")}"></div>
                    <div class="form-group"><label>New API Key (leave blank to keep)</label><input type="password" id="ep-apikey"></div>
                    <div class="form-group"><label>Priority</label><input type="number" id="ep-priority" value="${p.priority || 0}"></div>
                    <button type="submit" class="btn btn-primary">Save</button>
                </form>
            `);
            $("#edit-provider-form").addEventListener("submit", async (e) => {
                e.preventDefault();
                try {
                    const body = {
                        name: $("#ep-name").value,
                        provider: $("#ep-type").value,
                        base_url: $("#ep-url").value,
                        priority: parseInt($("#ep-priority").value) || 0,
                    };
                    if ($("#ep-apikey").value) body.api_key = $("#ep-apikey").value;
                    await api("/admin/providers/" + $("#ep-id").value, { method: "PATCH", body: JSON.stringify(body) });
                    closeModal();
                    toast("Provider updated", "success");
                    loadProviders();
                } catch (err) { toast(err.message); }
            });
        } catch (err) { toast(err.message); }
    };

    window._deleteProvider = async (id) => {
        if (!confirm("Delete this provider?")) return;
        try {
            await api("/admin/providers/" + id, { method: "DELETE" });
            toast("Provider deleted", "success");
            loadProviders();
        } catch (err) { toast(err.message); }
    };

    window._editModel = async (id) => {
        try {
            const res = await api("/admin/models/" + id);
            const m = res.data || res;
            openModal("Edit Model", `
                <form id="edit-model-form">
                    <input type="hidden" id="em-id" value="${esc(m.id)}">
                    <div class="form-group"><label>Provider ID</label><input type="text" id="em-provider-id" value="${esc(m.provider_id)}" required></div>
                    <div class="form-group"><label>Model ID</label><input type="text" id="em-model-id" value="${esc(m.model_id)}" required></div>
                    <div class="form-group"><label>Display Name</label><input type="text" id="em-display-name" value="${esc(m.display_name || "")}"></div>
                    <div class="form-group"><label>Mode</label><input type="text" id="em-mode" value="${esc(m.mode || "chat")}"></div>
                    <div class="form-group"><label>Context Window</label><input type="number" id="em-context" value="${m.context_window || 0}"></div>
                    <div class="form-group"><label>Max Output Tokens</label><input type="number" id="em-max-output" value="${m.max_output_tokens || 0}"></div>
                    <div class="form-group"><label>Input Cost per 1K</label><input type="number" step="0.0001" id="em-input-cost" value="${m.input_cost_per_1k || 0}"></div>
                    <div class="form-group"><label>Output Cost per 1K</label><input type="number" step="0.0001" id="em-output-cost" value="${m.output_cost_per_1k || 0}"></div>
                    <div class="form-group"><label>Default Timeout (ms)</label><input type="number" id="em-timeout" value="${m.default_timeout_ms || 30000}"></div>
                    <div class="form-group"><label>Tags (comma separated)</label><input type="text" id="em-tags" value="${(m.tags || []).join(", ")}"></div>
                    <div class="form-group"><label><input type="checkbox" id="em-active" ${m.is_active || m.enabled ? "checked" : ""}> Active</label></div>
                    <div class="form-group"><label><input type="checkbox" id="em-stream" ${m.supports_stream ? "checked" : ""}> Supports Streaming</label></div>
                    <button type="submit" class="btn btn-primary">Save</button>
                </form>
            `);
            $("#edit-model-form").addEventListener("submit", async (e) => {
                e.preventDefault();
                try {
                    const tags = $("#em-tags").value.split(",").map((t) => t.trim()).filter(Boolean);
                    const body = {
                        provider_id: $("#em-provider-id").value,
                        model_id: $("#em-model-id").value,
                        display_name: $("#em-display-name").value,
                        mode: $("#em-mode").value,
                        context_window: parseInt($("#em-context").value) || 0,
                        max_output_tokens: parseInt($("#em-max-output").value) || 0,
                        input_cost_per_1k: parseFloat($("#em-input-cost").value) || 0,
                        output_cost_per_1k: parseFloat($("#em-output-cost").value) || 0,
                        default_timeout_ms: parseInt($("#em-timeout").value) || 30000,
                        tags: tags,
                        is_active: $("#em-active").checked,
                        supports_stream: $("#em-stream").checked,
                    };
                    await api("/admin/models/" + $("#em-id").value, { method: "PUT", body: JSON.stringify(body) });
                    closeModal();
                    toast("Model updated", "success");
                    loadModels();
                } catch (err) { toast(err.message); }
            });
        } catch (err) { toast(err.message); }
    };

    window._toggleModel = async (id, enable) => {
        try {
            await api("/admin/models/" + id, {
                method: "PATCH",
                body: JSON.stringify({ enabled: enable }),
            });
            toast(enable ? "Model enabled" : "Model disabled", "success");
            loadModels();
        } catch (err) { toast(err.message); }
    };

    window._deleteModel = async (id) => {
        if (!confirm("Delete this model?")) return;
        try {
            await api("/admin/models/" + id, { method: "DELETE" });
            toast("Model deleted", "success");
            loadModels();
        } catch (err) { toast(err.message); }
    };

    window._editAlias = async (id) => {
        try {
            const res = await api("/admin/aliases/" + id);
            const a = res.data || res;
            openModal("Edit Alias", `
                <form id="edit-alias-form">
                    <input type="hidden" id="ea-id" value="${esc(a.id)}">
                    <div class="form-group"><label>Name</label><input type="text" id="ea-name" value="${esc(a.name)}" required></div>
                    <div class="form-group"><label>Target ID</label><input type="text" id="ea-target" value="${esc(a.target_id)}" required></div>
                    <div class="form-group"><label>Provider</label><input type="text" id="ea-provider" value="${esc(a.provider || "")}"></div>
                    <div class="form-group"><label>Description</label><input type="text" id="ea-desc" value="${esc(a.description || "")}"></div>
                    <button type="submit" class="btn btn-primary">Save</button>
                </form>
            `);
            $("#edit-alias-form").addEventListener("submit", async (e) => {
                e.preventDefault();
                try {
                    const body = {
                        name: $("#ea-name").value,
                        target_id: $("#ea-target").value,
                        provider: $("#ea-provider").value,
                        description: $("#ea-desc").value,
                    };
                    await api("/admin/aliases/" + $("#ea-id").value, { method: "PUT", body: JSON.stringify(body) });
                    closeModal();
                    toast("Alias updated", "success");
                    loadAliases();
                } catch (err) { toast(err.message); }
            });
        } catch (err) { toast(err.message); }
    };

    window._deleteAlias = async (id) => {
        if (!confirm("Delete this alias?")) return;
        try {
            await api("/admin/aliases/" + id, { method: "DELETE" });
            toast("Alias deleted", "success");
            loadAliases();
        } catch (err) { toast(err.message); }
    };

    window._viewCombo = async (id) => {
        try {
            const res = await api("/admin/combos/" + id);
            const c = res.data || res;
            openModal("Combo Details", `
                <div class="combo-detail">
                    <p><strong>Name:</strong> ${esc(c.name)}</p>
                    <p><strong>Description:</strong> ${esc(c.description || "-")}</p>
                    <p><strong>User ID:</strong> ${esc(c.user_id)}</p>
                    <p><strong>Status:</strong> ${c.is_active ? "Active" : "Inactive"}</p>
                    <p><strong>Items:</strong> ${c.items ? c.items.length : 0}</p>
                </div>
            `);
        } catch (err) { toast(err.message); }
    };

    window._editQuota = async (id) => {
        try {
            const res = await api("/admin/quotas/" + id);
            const q = res.data || res;
            openModal("Edit Quota", `
                <form id="edit-quota-form">
                    <input type="hidden" id="eq-id" value="${esc(q.id)}">
                    <div class="form-group"><label>Monthly Token Cap</label><input type="number" id="eq-token-cap" value="${q.monthly_token_cap || 0}"></div>
                    <div class="form-group"><label>Monthly Cost Cap ($)</label><input type="number" step="0.0001" id="eq-cost-cap" value="${q.monthly_cost_cap || 0}"></div>
                    <button type="submit" class="btn btn-primary">Save</button>
                </form>
            `);
            $("#edit-quota-form").addEventListener("submit", async (e) => {
                e.preventDefault();
                try {
                    const body = {
                        monthly_token_cap: parseInt($("#eq-token-cap").value) || 0,
                        monthly_cost_cap: parseFloat($("#eq-cost-cap").value) || 0,
                    };
                    await api("/admin/quotas/" + $("#eq-id").value, { method: "PUT", body: JSON.stringify(body) });
                    closeModal();
                    toast("Quota updated", "success");
                    loadQuotas();
                } catch (err) { toast(err.message); }
            });
        } catch (err) { toast(err.message); }
    };

    window._resetQuota = async (id) => {
        if (!confirm("Reset this quota's usage?")) return;
        try {
            await api("/admin/quotas/" + id + "/reset-usage", { method: "POST" });
            toast("Quota reset", "success");
            loadQuotas();
        } catch (err) { toast(err.message); }
    };

    window._deleteQuota = async (id) => {
        if (!confirm("Delete this quota?")) return;
        try {
            await api("/admin/quotas/" + id, { method: "DELETE" });
            toast("Quota deleted", "success");
            loadQuotas();
        } catch (err) { toast(err.message); }
    };

    document.addEventListener("DOMContentLoaded", () => {
        showPage("login-page");

        $("#login-form").addEventListener("submit", async (e) => {
            e.preventDefault();
            const email = $("#login-email").value;
            const password = $("#login-password").value;
            const errEl = $("#login-error");
            errEl.classList.add("hidden");
            try {
                const res = await api("/auth/login", {
                    method: "POST",
                    body: JSON.stringify({ email, password }),
                });
                token = res.token || res.data?.token;
                if (token) {
                    localStorage.setItem("gorouter_token", token);
                    document.cookie = "auth_token=" + token + "; path=/; SameSite=Strict";
                    checkAuth();
                }
            } catch (err) {
                errEl.textContent = err.message;
                errEl.classList.remove("hidden");
            }
        });

        $$(".nav-link").forEach((link) => {
            link.addEventListener("click", (e) => {
                e.preventDefault();
                location.hash = link.getAttribute("href");
            });
        });

        $("#logout-btn").addEventListener("click", () => {
            document.cookie = "auth_token=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT";
            logout();
        });

        $("#modal-close-btn").addEventListener("click", closeModal);
        $("#modal-overlay").addEventListener("click", (e) => {
            if (e.target === $("#modal-overlay")) closeModal();
        });

        $("#btn-add-user").addEventListener("click", () => {
            openModal("Add User", `
                <form id="add-user-form">
                    <div class="form-group"><label>Email</label><input type="email" id="au-email" required></div>
                    <div class="form-group"><label>Name</label><input type="text" id="au-name"></div>
                    <div class="form-group"><label>Password</label><input type="password" id="au-password" required></div>
                    <div class="form-group"><label>Role</label><select id="au-role"><option value="user">User</option><option value="admin">Admin</option></select></div>
                    <button type="submit" class="btn btn-primary">Create User</button>
                </form>
            `);
            $("#add-user-form").addEventListener("submit", async (e) => {
                e.preventDefault();
                try {
                    await api("/admin/users", {
                        method: "POST",
                        body: JSON.stringify({
                            email: $("#au-email").value,
                            name: $("#au-name").value,
                            password: $("#au-password").value,
                            role: $("#au-role").value,
                        }),
                    });
                    closeModal();
                    toast("User created", "success");
                    loadUsers();
                } catch (err) { toast(err.message); }
            });
        });

        $("#btn-add-apikey").addEventListener("click", () => {
            openModal("Create API Key", `
                <form id="add-apikey-form">
                    <div class="form-group"><label>User ID</label><input type="text" id="ak-user-id" required></div>
                    <div class="form-group"><label>Name</label><input type="text" id="ak-name" value="api-key"></div>
                    <div class="form-group"><label>Scopes (comma separated)</label><input type="text" id="ak-scopes" value="chat, models:read"></div>
                    <div class="form-group"><label>Expires At (RFC3339, optional)</label><input type="text" id="ak-expires" placeholder="2026-12-31T00:00:00Z"></div>
                    <button type="submit" class="btn btn-primary">Create Key</button>
                </form>
            `);
            $("#add-apikey-form").addEventListener("submit", async (e) => {
                e.preventDefault();
                try {
                    const scopes = $("#ak-scopes").value.split(",").map((s) => s.trim()).filter(Boolean);
                    const body = {
                        user_id: $("#ak-user-id").value,
                        name: $("#ak-name").value,
                        scopes: scopes,
                    };
                    if ($("#ak-expires").value) body.expires_at = $("#ak-expires").value;
                    const res = await api("/admin/api-keys/create", {
                        method: "POST",
                        body: JSON.stringify(body),
                    });
                    const d = res.data || res;
                    closeModal();
                    if (d.key) {
                        openModal("API Key Created", `
                            <div class="new-key-banner">
                                <p>Save this key now - it will not be shown again:</p>
                                <code>${esc(d.key)}</code>
                            </div>
                            <button class="btn btn-primary" onclick="navigator.clipboard.writeText('${esc(d.key)}');this.textContent='Copied!'">Copy Key</button>
                        `);
                    }
                    loadApiKeys();
                } catch (err) { toast(err.message); }
            });
        });

        $("#btn-add-provider").addEventListener("click", () => {
            openModal("Add Provider", `
                <form id="add-provider-form">
                    <div class="form-group"><label>Name</label><input type="text" id="ap-name" required></div>
                    <div class="form-group"><label>Provider Type</label><select id="ap-type">
                        <option value="openai">OpenAI</option>
                        <option value="anthropic">Anthropic</option>
                        <option value="openrouter">OpenRouter</option>
                        <option value="custom">Custom</option>
                    </select></div>
                    <div class="form-group"><label>Base URL</label><input type="url" id="ap-url"></div>
                    <div class="form-group"><label>API Key</label><input type="password" id="ap-apikey"></div>
                    <div class="form-group"><label>Priority</label><input type="number" id="ap-priority" value="0"></div>
                    <button type="submit" class="btn btn-primary">Create Provider</button>
                </form>
            `);
            $("#add-provider-form").addEventListener("submit", async (e) => {
                e.preventDefault();
                try {
                    await api("/admin/providers", {
                        method: "POST",
                        body: JSON.stringify({
                            name: $("#ap-name").value,
                            provider: $("#ap-type").value,
                            base_url: $("#ap-url").value,
                            api_key: $("#ap-apikey").value || undefined,
                            priority: parseInt($("#ap-priority").value) || 0,
                        }),
                    });
                    closeModal();
                    toast("Provider created", "success");
                    loadProviders();
                } catch (err) { toast(err.message); }
            });
        });

        $("#btn-add-model").addEventListener("click", () => {
            openModal("Add Model", `
                <form id="add-model-form">
                    <div class="form-group"><label>Provider ID</label><input type="text" id="am-provider-id" required></div>
                    <div class="form-group"><label>Model ID</label><input type="text" id="am-model-id" required></div>
                    <div class="form-group"><label>Display Name</label><input type="text" id="am-display-name"></div>
                    <div class="form-group"><label>Mode</label><input type="text" id="am-mode" value="chat"></div>
                    <div class="form-group"><label>Context Window</label><input type="number" id="am-context" value="8192"></div>
                    <div class="form-group"><label>Max Output Tokens</label><input type="number" id="am-max-output" value="4096"></div>
                    <div class="form-group"><label>Input Cost per 1K</label><input type="number" step="0.0001" id="am-input-cost" value="0"></div>
                    <div class="form-group"><label>Output Cost per 1K</label><input type="number" step="0.0001" id="am-output-cost" value="0"></div>
                    <div class="form-group"><label>Default Timeout (ms)</label><input type="number" id="am-timeout" value="30000"></div>
                    <div class="form-group"><label>Tags (comma separated)</label><input type="text" id="am-tags"></div>
                    <div class="form-group"><label><input type="checkbox" id="am-active" checked> Active</label></div>
                    <div class="form-group"><label><input type="checkbox" id="am-stream" checked> Supports Streaming</label></div>
                    <button type="submit" class="btn btn-primary">Create Model</button>
                </form>
            `);
            $("#add-model-form").addEventListener("submit", async (e) => {
                e.preventDefault();
                try {
                    const tags = $("#am-tags").value.split(",").map((t) => t.trim()).filter(Boolean);
                    await api("/admin/models", {
                        method: "POST",
                        body: JSON.stringify({
                            provider_id: $("#am-provider-id").value,
                            model_id: $("#am-model-id").value,
                            display_name: $("#am-display-name").value,
                            mode: $("#am-mode").value,
                            context_window: parseInt($("#am-context").value) || 0,
                            max_output_tokens: parseInt($("#am-max-output").value) || 0,
                            input_cost_per_1k: parseFloat($("#am-input-cost").value) || 0,
                            output_cost_per_1k: parseFloat($("#am-output-cost").value) || 0,
                            default_timeout_ms: parseInt($("#am-timeout").value) || 30000,
                            tags: tags,
                            is_active: $("#am-active").checked,
                            supports_stream: $("#am-stream").checked,
                        }),
                    });
                    closeModal();
                    toast("Model created", "success");
                    loadModels();
                } catch (err) { toast(err.message); }
            });
        });

        $("#btn-add-alias") && $("#btn-add-alias").addEventListener("click", () => {
            openModal("Add Alias", `
                <form id="add-alias-form">
                    <div class="form-group"><label>Name</label><input type="text" id="aa-name" required></div>
                    <div class="form-group"><label>Target ID</label><input type="text" id="aa-target" required></div>
                    <div class="form-group"><label>Provider</label><input type="text" id="aa-provider"></div>
                    <div class="form-group"><label>Description</label><input type="text" id="aa-desc"></div>
                    <button type="submit" class="btn btn-primary">Create Alias</button>
                </form>
            `);
            $("#add-alias-form").addEventListener("submit", async (e) => {
                e.preventDefault();
                try {
                    await api("/admin/aliases", {
                        method: "POST",
                        body: JSON.stringify({
                            name: $("#aa-name").value,
                            target_id: $("#aa-target").value,
                            provider: $("#aa-provider").value,
                            description: $("#aa-desc").value,
                        }),
                    });
                    closeModal();
                    toast("Alias created", "success");
                    loadAliases();
                } catch (err) { toast(err.message); }
            });
        });

        $("#btn-add-quota") && $("#btn-add-quota").addEventListener("click", () => {
            openModal("Add Quota", `
                <form id="add-quota-form">
                    <div class="form-group"><label>User ID</label><input type="text" id="aq-user-id" required></div>
                    <div class="form-group"><label>Monthly Token Cap</label><input type="number" id="aq-token-cap" value="0"></div>
                    <div class="form-group"><label>Monthly Cost Cap ($)</label><input type="number" step="0.0001" id="aq-cost-cap" value="0"></div>
                    <button type="submit" class="btn btn-primary">Create Quota</button>
                </form>
            `);
            $("#add-quota-form").addEventListener("submit", async (e) => {
                e.preventDefault();
                try {
                    await api("/admin/quotas", {
                        method: "POST",
                        body: JSON.stringify({
                            user_id: $("#aq-user-id").value,
                            monthly_token_cap: parseInt($("#aq-token-cap").value) || 0,
                            monthly_cost_cap: parseFloat($("#aq-cost-cap").value) || 0,
                        }),
                    });
                    closeModal();
                    toast("Quota created", "success");
                    loadQuotas();
                } catch (err) { toast(err.message); }
            });
        });

        window.addEventListener("hashchange", router);
        checkAuth();
    });
})();
