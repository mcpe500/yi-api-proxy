(function () {
    const API_BASE = "/api";
    let token = localStorage.getItem("gorouter_token");

    const $ = (sel) => document.querySelector(sel);
    const $$ = (sel) => document.querySelectorAll(sel);

    async function api(path, opts = {}) {
        const headers = { "Content-Type": "application/json", ...(opts.headers || {}) };
        if (token) headers["Authorization"] = "Bearer " + token;
        const res = await fetch(API_BASE + path, { ...opts, headers });
        if (res.status === 401) { logout(); throw new Error("Unauthorized"); }
        if (!res.ok) {
            const body = await res.json().catch(() => ({}));
            throw new Error(body.error || body.message || "Request failed");
        }
        return res.json();
    }

    function show(id) {
        $$(".view").forEach((v) => v.classList.add("hidden"));
        const el = $("#" + id + "-view");
        if (el) el.classList.remove("hidden");
    }

    function showApp() {
        $("#login-view").classList.add("hidden");
        $("#sidebar").style.display = "flex";
    }

    function showLogin() {
        $$(".view").forEach((v) => v.classList.add("hidden"));
        $("#login-view").classList.remove("hidden");
        $("#sidebar").style.display = "none";
    }

    function logout() {
        token = null;
        localStorage.removeItem("gorouter_token");
        showLogin();
    }

    async function checkAuth() {
        if (!token) { showLogin(); return; }
        try {
            const me = await api("/auth/me");
            $("#user-email").textContent = me.email || me.data?.email || "";
            showApp();
            navigate("dashboard");
        } catch {
            logout();
        }
    }

    function navigate(section) {
        $$(".nav-item").forEach((n) => n.classList.remove("active"));
        const nav = $(`.nav-item[data-section="${section}"]`);
        if (nav) nav.classList.add("active");
        show(section);
        loadSection(section);
    }

    async function loadSection(section) {
        switch (section) {
            case "dashboard": await loadDashboard(); break;
            case "users": await loadUsers(); break;
            case "apikeys": await loadApiKeys(); break;
            case "providers": await loadProviders(); break;
            case "models": await loadModels(); break;
            case "usage": await loadUsage(); break;
            case "settings": await loadSettings(); break;
        }
    }

    async function loadDashboard() {
        try {
            const stats = await api("/admin/usage/summary");
            const d = stats.data || stats;
            const cards = [
                { label: "Total Users", value: d.total_users ?? "-" },
                { label: "Active Keys", value: d.active_keys ?? "-" },
                { label: "Providers", value: d.providers ?? "-" },
                { label: "Requests (24h)", value: d.requests_24h ?? "-" },
                { label: "Tokens Used", value: d.tokens_used ?? "-" },
                { label: "Error Rate", value: d.error_rate != null ? d.error_rate + "%" : "-" },
            ];
            $("#stats-cards").innerHTML = cards
                .map(
                    (c) =>
                        `<div class="stat-card"><div class="stat-label">${c.label}</div><div class="stat-value">${c.value}</div></div>`
                )
                .join("");
        } catch {
            $("#stats-cards").innerHTML = '<div class="stat-card"><div class="stat-label">Error</div><div class="stat-value">-</div></div>';
        }
    }

    async function loadUsers() {
        try {
            const res = await api("/admin/users");
            const users = res.data || res || [];
            const tbody = $("#users-table tbody");
            tbody.innerHTML = users
                .map(
                    (u) => `<tr>
                    <td>${esc(u.email)}</td>
                    <td>${esc(u.role)}</td>
                    <td><span class="badge badge-${u.status || "active"}">${esc(u.status || "active")}</span></td>
                    <td>${esc(u.created_at || "-")}</td>
                    <td>
                        <button class="btn btn-sm btn-secondary" onclick="editUser('${u.id}')">Edit</button>
                        ${u.status !== "suspended" ? `<button class="btn btn-sm btn-danger" onclick="suspendUser('${u.id}')">Suspend</button>` : `<button class="btn btn-sm btn-primary" onclick="activateUser('${u.id}')">Activate</button>`}
                    </td>
                </tr>`
                )
                .join("");
        } catch {
            $("#users-table tbody").innerHTML = '<tr><td colspan="5">Failed to load users</td></tr>';
        }
    }

    async function loadApiKeys() {
        try {
            const res = await api("/admin/api-keys");
            const keys = res.data || res || [];
            const tbody = $("#apikeys-table tbody");
            tbody.innerHTML = keys
                .map(
                    (k) => `<tr>
                    <td>${esc(k.name || k.label)}</td>
                    <td>${esc(k.user_email || k.user_id)}</td>
                    <td>${esc(k.key_prefix || "-")}</td>
                    <td>${k.quota_limit ?? "unlimited"}</td>
                    <td>${k.quota_used ?? 0}</td>
                    <td><span class="badge badge-${k.status === "revoked" ? "suspended" : "active"}">${esc(k.status || "active")}</span></td>
                    <td>${esc(k.created_at || "-")}</td>
                    <td>
                        ${k.status !== "revoked" ? `<button class="btn btn-sm btn-danger" onclick="revokeKey('${k.id}')">Revoke</button>` : ""}
                        <button class="btn btn-sm btn-secondary" onclick="rotateKey('${k.id}')">Rotate</button>
                    </td>
                </tr>`
                )
                .join("");
        } catch {
            $("#apikeys-table tbody").innerHTML = '<tr><td colspan="8">Failed to load API keys</td></tr>';
        }
    }

    async function loadProviders() {
        try {
            const res = await api("/admin/providers");
            const providers = res.data || res || [];
            const tbody = $("#providers-table tbody");
            tbody.innerHTML = providers
                .map(
                    (p) => `<tr>
                    <td>${esc(p.name)}</td>
                    <td>${esc(p.type)}</td>
                    <td>${esc(p.base_url || "-")}</td>
                    <td><span class="badge badge-${p.enabled ? "enabled" : "disabled"}">${p.enabled ? "Enabled" : "Disabled"}</span></td>
                    <td>
                        <button class="btn btn-sm btn-secondary" onclick="editProvider('${p.id}')">Edit</button>
                        <button class="btn btn-sm btn-secondary" onclick="testProvider('${p.id}')">Test</button>
                        <button class="btn btn-sm btn-${p.enabled ? "danger" : "primary"}" onclick="toggleProvider('${p.id}', ${!p.enabled})">${p.enabled ? "Disable" : "Enable"}</button>
                    </td>
                </tr>`
                )
                .join("");
        } catch {
            $("#providers-table tbody").innerHTML = '<tr><td colspan="5">Failed to load providers</td></tr>';
        }
    }

    async function loadModels() {
        try {
            const res = await api("/admin/models");
            const models = res.data || res || [];
            const tbody = $("#models-table tbody");
            tbody.innerHTML = models
                .map(
                    (m) => `<tr>
                    <td>${esc(m.model_id || m.id)}</td>
                    <td>${esc(m.provider_name || m.provider_id)}</td>
                    <td><span class="badge badge-${m.enabled ? "enabled" : "disabled"}">${m.enabled ? "Enabled" : "Disabled"}</span></td>
                    <td><button class="btn btn-sm btn-secondary" onclick="toggleModel('${m.id}', ${!m.enabled})">${m.enabled ? "Disable" : "Enable"}</button></td>
                </tr>`
                )
                .join("");
        } catch {
            $("#models-table tbody").innerHTML = '<tr><td colspan="4">Failed to load models</td></tr>';
        }
    }

    async function loadUsage() {
        try {
            const res = await api("/admin/usage/timeseries");
            const points = res.data || res || [];
            drawBarChart("usage-chart", points);
        } catch {
            drawBarChart("usage-chart", []);
        }
        try {
            const summary = await api("/admin/usage/summary");
            const d = summary.data || summary;
            $("#usage-summary").innerHTML = `
                <div class="stats-grid">
                    <div class="stat-card"><div class="stat-label">Total Requests</div><div class="stat-value">${d.total_requests ?? "-"}</div></div>
                    <div class="stat-card"><div class="stat-label">Total Tokens</div><div class="stat-value">${d.total_tokens ?? "-"}</div></div>
                    <div class="stat-card"><div class="stat-label">Avg Latency</div><div class="stat-value">${d.avg_latency_ms != null ? d.avg_latency_ms + "ms" : "-"}</div></div>
                </div>`;
        } catch {}
    }

    async function loadSettings() {
        try {
            const res = await api("/admin/settings");
            const s = res.data || res || {};
            $("#setting-default-quota").value = s.default_quota ?? "";
            $("#setting-rate-limit").value = s.rate_limit ?? "";
        } catch {}
    }

    function drawBarChart(canvasId, data) {
        const canvas = document.getElementById(canvasId);
        if (!canvas) return;
        const ctx = canvas.getContext("2d");
        const w = canvas.parentElement.clientWidth - 48;
        const h = 300;
        canvas.width = w;
        canvas.height = h;

        ctx.fillStyle = "#1a1d27";
        ctx.fillRect(0, 0, w, h);

        if (!data.length) {
            ctx.fillStyle = "#8b8fa8";
            ctx.font = "14px sans-serif";
            ctx.textAlign = "center";
            ctx.fillText("No usage data available", w / 2, h / 2);
            return;
        }

        const values = data.map((d) => d.count || d.requests || d.value || 0);
        const labels = data.map((d) => d.date || d.label || d.hour || "");
        const max = Math.max(...values, 1);
        const barW = Math.max(4, (w - 40) / data.length - 2);
        const chartH = h - 50;

        ctx.fillStyle = "#8b8fa8";
        ctx.font = "11px sans-serif";
        ctx.textAlign = "center";
        data.forEach((_, i) => {
            const x = 30 + i * (barW + 2);
            if (i % Math.ceil(data.length / 10) === 0) {
                ctx.fillText(labels[i], x + barW / 2, h - 5);
            }
        });

        values.forEach((v, i) => {
            const barH = (v / max) * (chartH - 20);
            const x = 30 + i * (barW + 2);
            const y = chartH - barH;
            ctx.fillStyle = "#6c5ce7";
            ctx.fillRect(x, y, barW, barH);
        });

        ctx.fillStyle = "#8b8fa8";
        ctx.font = "11px sans-serif";
        ctx.textAlign = "right";
        for (let i = 0; i <= 4; i++) {
            const y = chartH - (i / 4) * (chartH - 20);
            ctx.fillText(Math.round((max * i) / 4), 25, y + 4);
        }
    }

    function esc(s) {
        if (s == null) return "";
        const d = document.createElement("div");
        d.textContent = String(s);
        return d.innerHTML;
    }

    function showModal(id) {
        $(id).classList.remove("hidden");
    }

    function hideModal(id) {
        $(id).classList.add("hidden");
    }

    document.addEventListener("DOMContentLoaded", () => {
        $("#sidebar").style.display = "none";

        $("#login-form").addEventListener("submit", async (e) => {
            e.preventDefault();
            const email = $("#login-email").value;
            const password = $("#login-password").value;
            $("#login-error").textContent = "";
            try {
                const res = await api("/auth/login", {
                    method: "POST",
                    body: JSON.stringify({ email, password }),
                });
                token = res.token || res.data?.token;
                if (token) {
                    localStorage.setItem("gorouter_token", token);
                    await checkAuth();
                }
            } catch (err) {
                $("#login-error").textContent = err.message;
            }
        });

        $$(".nav-item").forEach((item) => {
            item.addEventListener("click", (e) => {
                e.preventDefault();
                navigate(item.dataset.section);
            });
        });

        $("#logout-btn").addEventListener("click", logout);

        $("#add-user-btn").addEventListener("click", () => {
            $("#user-id").value = "";
            $("#user-email-input").value = "";
            $("#user-password").value = "";
            $("#user-role").value = "user";
            $("#user-modal-title").textContent = "Add User";
            showModal("#user-modal");
        });

        $("#user-form").addEventListener("submit", async (e) => {
            e.preventDefault();
            const id = $("#user-id").value;
            const body = {
                email: $("#user-email-input").value,
                password: $("#user-password").value || undefined,
                role: $("#user-role").value,
            };
            try {
                if (id) {
                    await api("/admin/users/" + id, { method: "PATCH", body: JSON.stringify(body) });
                } else {
                    await api("/admin/users", { method: "POST", body: JSON.stringify(body) });
                }
                hideModal("#user-modal");
                loadUsers();
            } catch (err) {
                alert(err.message);
            }
        });

        $("#add-provider-btn").addEventListener("click", () => {
            $("#provider-id").value = "";
            $("#provider-name").value = "";
            $("#provider-url").value = "";
            $("#provider-apikey").value = "";
            $("#provider-type").value = "openai";
            $("#provider-modal-title").textContent = "Add Provider";
            showModal("#provider-modal");
        });

        $("#provider-form").addEventListener("submit", async (e) => {
            e.preventDefault();
            const id = $("#provider-id").value;
            const body = {
                name: $("#provider-name").value,
                type: $("#provider-type").value,
                base_url: $("#provider-url").value,
                api_key: $("#provider-apikey").value || undefined,
            };
            try {
                if (id) {
                    await api("/admin/providers/" + id, { method: "PATCH", body: JSON.stringify(body) });
                } else {
                    await api("/admin/providers", { method: "POST", body: JSON.stringify(body) });
                }
                hideModal("#provider-modal");
                loadProviders();
            } catch (err) {
                alert(err.message);
            }
        });

        $$(".close").forEach((btn) => {
            btn.addEventListener("click", () => {
                btn.closest(".modal").classList.add("hidden");
            });
        });

        window.addEventListener("click", (e) => {
            if (e.target.classList.contains("modal")) e.target.classList.add("hidden");
        });

        $("#settings-form").addEventListener("submit", async (e) => {
            e.preventDefault();
            try {
                await api("/admin/settings", {
                    method: "PATCH",
                    body: JSON.stringify({
                        default_quota: parseInt($("#setting-default-quota").value) || 0,
                        rate_limit: parseInt($("#setting-rate-limit").value) || 0,
                    }),
                });
                alert("Settings saved");
            } catch (err) {
                alert(err.message);
            }
        });

        checkAuth();
    });

    window.editUser = async (id) => {
        try {
            const res = await api("/admin/users/" + id);
            const u = res.data || res;
            $("#user-id").value = u.id;
            $("#user-email-input").value = u.email;
            $("#user-role").value = u.role;
            $("#user-password").value = "";
            $("#user-modal-title").textContent = "Edit User";
            showModal("#user-modal");
        } catch (err) {
            alert(err.message);
        }
    };

    window.suspendUser = async (id) => {
        if (!confirm("Suspend this user?")) return;
        try {
            await api("/admin/users/" + id + "/suspend", { method: "POST" });
            loadUsers();
        } catch (err) { alert(err.message); }
    };

    window.activateUser = async (id) => {
        try {
            await api("/admin/users/" + id + "/activate", { method: "POST" });
            loadUsers();
        } catch (err) { alert(err.message); }
    };

    window.revokeKey = async (id) => {
        if (!confirm("Revoke this key?")) return;
        try {
            await api("/admin/api-keys/" + id + "/revoke", { method: "POST" });
            loadApiKeys();
        } catch (err) { alert(err.message); }
    };

    window.rotateKey = async (id) => {
        if (!confirm("Rotate this key? Old key will stop working.")) return;
        try {
            const res = await api("/admin/api-keys/" + id + "/rotate", { method: "POST" });
            const d = res.data || res;
            if (d.key) alert("New key: " + d.key);
            loadApiKeys();
        } catch (err) { alert(err.message); }
    };

    window.editProvider = async (id) => {
        try {
            const res = await api("/admin/providers/" + id);
            const p = res.data || res;
            $("#provider-id").value = p.id;
            $("#provider-name").value = p.name;
            $("#provider-type").value = p.type;
            $("#provider-url").value = p.base_url || "";
            $("#provider-apikey").value = "";
            $("#provider-modal-title").textContent = "Edit Provider";
            showModal("#provider-modal");
        } catch (err) { alert(err.message); }
    };

    window.testProvider = async (id) => {
        try {
            const res = await api("/admin/providers/" + id + "/test", { method: "POST" });
            const d = res.data || res;
            alert(d.success ? "Connection OK" : "Failed: " + (d.error || "unknown"));
        } catch (err) { alert("Test failed: " + err.message); }
    };

    window.toggleProvider = async (id, enable) => {
        try {
            await api("/admin/providers/" + id + "/" + (enable ? "enable" : "disable"), { method: "POST" });
            loadProviders();
        } catch (err) { alert(err.message); }
    };

    window.toggleModel = async (id, enable) => {
        try {
            await api("/admin/models/" + id, {
                method: "PATCH",
                body: JSON.stringify({ enabled: enable }),
            });
            loadModels();
        } catch (err) { alert(err.message); }
    };
})();
