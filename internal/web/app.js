"use strict";

const $ = (id) => document.getElementById(id);
const tokenKey = "us_token";
const emailKey = "us_email";
const themeKey = "us_theme";
const pageSize = 20;
const { validURL, validAlias, expiryISO, localDateTime, linkStatus, formatDate, mergePage, responseMessage } = UILogic;
const state = {
  mode: "login",
  links: [],
  total: 0,
  selectedCode: "",
  listRequest: 0,
  statsRequest: 0,
  listBusy: false,
};

function notice(id, message, kind = "error") {
  const el = $(id);
  el.textContent = message;
  el.hidden = !message;
  el.classList.toggle("notice-success", kind === "success");
  el.classList.toggle("notice-error", kind !== "success");
}

function fieldError(id, message) {
  const input = $(id);
  $(id + "-error").textContent = message;
  input.setAttribute("aria-invalid", message ? "true" : "false");
  return !message;
}

function clearFields(ids) {
  ids.forEach((id) => fieldError(id, ""));
}

function safeURL(value) {
  return validURL(value) ? value : "";
}

function setBusy(button, busy, label) {
  if (busy) {
    button.dataset.idleLabel = button.textContent;
    button.dataset.busyLabel = label;
    button.textContent = label;
  } else {
    if (button.textContent === button.dataset.busyLabel) {
      button.textContent = button.dataset.idleLabel || button.textContent;
    }
    delete button.dataset.busyLabel;
    delete button.dataset.idleLabel;
  }
  button.disabled = busy;
}

function currentToken() {
  return localStorage.getItem(tokenKey) || "";
}

async function request(path, options = {}) {
  const headers = { "Content-Type": "application/json", ...options.headers };
  if (currentToken()) headers.Authorization = "Bearer " + currentToken();
  let response;
  try {
    response = await fetch(path, { ...options, headers });
  } catch {
    throw new Error("Sem conexão com o serviço. Confira a rede e tente novamente.");
  }
  if (response.status === 401 && !path.startsWith("/auth/")) {
    endSession("Sua sessão expirou. Entre novamente para continuar.");
    throw new Error("Sua sessão expirou.");
  }
  if (response.status === 204) return {};
  let data = {};
  try { data = await response.json(); } catch { /* Resposta vazia ou inválida. */ }
  if (!response.ok) {
    if (response.status === 429) throw new Error("Muitas tentativas. Aguarde um pouco e tente novamente.");
    if (response.status >= 500) throw new Error("O serviço está indisponível no momento. Tente novamente.");
    const err = new Error(responseMessage(response.status, data.error));
    err.status = response.status;
    err.code = data.error;
    throw err;
  }
  return data;
}

function themeIsDark() {
  const stored = localStorage.getItem(themeKey);
  return stored ? stored === "dark" : window.matchMedia("(prefers-color-scheme: dark)").matches;
}

function updateThemeButton() {
  const dark = themeIsDark();
  $("theme-toggle").textContent = dark ? "☀ Tema claro" : "◐ Tema escuro";
  $("theme-toggle").setAttribute("aria-label", dark ? "Ativar tema claro" : "Ativar tema escuro");
  document.querySelector('meta[name="theme-color"]').content = dark ? "#131b22" : "#f4f1eb";
}

function initTheme() {
  const stored = localStorage.getItem(themeKey);
  if (stored === "light" || stored === "dark") document.documentElement.dataset.theme = stored;
  updateThemeButton();
  $("theme-toggle").addEventListener("click", () => {
    const next = themeIsDark() ? "light" : "dark";
    localStorage.setItem(themeKey, next);
    document.documentElement.dataset.theme = next;
    updateThemeButton();
  });
  window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => {
    if (!localStorage.getItem(themeKey)) updateThemeButton();
  });
}

function setMode(mode) {
  state.mode = mode;
  $("tab-login").setAttribute("aria-pressed", String(mode === "login"));
  $("tab-register").setAttribute("aria-pressed", String(mode === "register"));
  $("password").autocomplete = mode === "register" ? "new-password" : "current-password";
  $("auth-go").textContent = mode === "register" ? "Criar conta" : "Entrar";
  clearFields(["email", "password"]);
  notice("auth-err", "");
}

function showAuth() {
  $("auth").hidden = false;
  $("app").hidden = true;
  $("session").hidden = true;
  $("who").textContent = "";
}

function showApp() {
  $("auth").hidden = true;
  $("app").hidden = false;
  $("session").hidden = false;
  $("who").textContent = localStorage.getItem(emailKey) || "";
  $("who").title = $("who").textContent;
  notice("session-notice", "");
  loadLinks(false);
}

function endSession(message = "") {
  localStorage.removeItem(tokenKey);
  localStorage.removeItem(emailKey);
  state.links = [];
  state.total = 0;
  state.selectedCode = "";
  state.listRequest++;
  state.statsRequest++;
  state.listBusy = false;
  $("links").replaceChildren();
  $("details").hidden = true;
  $("result").hidden = true;
  $("password").value = "";
  showAuth();
  notice("session-notice", message);
  if (message) $("email").focus();
}

async function submitAuth(event) {
  event.preventDefault();
  clearFields(["email", "password"]);
  notice("auth-err", "");
  const email = $("email").value.trim();
  const password = $("password").value;
  let valid = true;
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) valid = fieldError("email", "Informe um e-mail válido.") && valid;
  if (password.length < 8) valid = fieldError("password", "Use pelo menos 8 caracteres.") && valid;
  if (!valid) {
    document.querySelector('#auth-form [aria-invalid="true"]').focus();
    return;
  }
  const button = $("auth-go");
  setBusy(button, true, state.mode === "register" ? "Criando conta…" : "Entrando…");
  try {
    const path = state.mode === "register" ? "/auth/register" : "/auth/login";
    const data = await request(path, { method: "POST", body: JSON.stringify({ email, password }) });
    localStorage.setItem(tokenKey, data.access_token);
    localStorage.setItem(emailKey, data.email);
    $("password").value = "";
    showApp();
    $("workspace-title").focus();
  } catch (error) {
    if (error.status === 401) notice("auth-err", "E-mail ou senha incorretos.");
    else if (state.mode === "register" && error.status === 400) notice("auth-err", "Não foi possível criar a conta. Confira os dados ou tente outro e-mail.");
    else notice("auth-err", error.message);
  } finally {
    setBusy(button, false);
  }
}

async function logout() {
  const button = $("logout");
  setBusy(button, true, "Saindo…");
  try { await request("/auth/logout", { method: "POST" }); } catch { /* Logout é local. */ }
  endSession();
  setBusy(button, false);
  $("email").focus();
}

async function copyText(value, targetId) {
  try {
    await navigator.clipboard.writeText(value);
    $(targetId).textContent = "Link copiado.";
  } catch {
    $(targetId).textContent = "Não foi possível copiar. Selecione o link exibido.";
  }
}

function showResult(shortURL) {
  const safe = safeURL(shortURL);
  if (!safe) throw new Error("O serviço devolveu um link inválido.");
  $("created").href = safe;
  $("created").textContent = safe;
  $("copy-msg").textContent = "";
  $("curl").textContent = 'curl -sS -D - -o NUL "' + safe + '"';
  try {
    drawQR($("qr"), safe);
    $("qr").hidden = false;
  } catch {
    $("qr").hidden = true;
    $("qr").removeAttribute("src");
  }
  $("result").hidden = false;
}

async function submitCreate(event) {
  event.preventDefault();
  clearFields(["url", "alias", "expires"]);
  notice("create-err", "");
  const url = $("url").value.trim();
  const alias = $("alias").value.trim();
  const expires = expiryISO($("expires").value);
  let valid = true;
  if (!validURL(url)) valid = fieldError("url", "Informe uma URL http ou https válida.") && valid;
  if (alias && !validAlias(alias)) valid = fieldError("alias", "Use 3 a 20 letras, números ou hífens entre caracteres.") && valid;
  if (expires === null) valid = fieldError("expires", "Escolha uma data futura válida.") && valid;
  if (!valid) {
    document.querySelector('#create-form [aria-invalid="true"]').focus();
    return;
  }
  const body = { url };
  if (alias) body.alias = alias;
  if (expires) body.expires_at = expires;
  const button = $("create");
  setBusy(button, true, "Criando…");
  try {
    const data = await request("/links", { method: "POST", body: JSON.stringify(body) });
    showResult(data.short_url);
    $("create-form").reset();
    await loadLinks(false);
    $("created").focus();
  } catch (error) {
    if (error.code === "invalid url") fieldError("url", error.message);
    else if (error.code === "invalid alias") fieldError("alias", error.message);
    else if (error.code === "invalid expiration") fieldError("expires", error.message);
    else if (error.status === 409 && alias) fieldError("alias", "Este alias já está em uso.");
    else notice("create-err", error.message);
  } finally {
    setBusy(button, false);
  }
}

function linkByCode(code) {
  return state.links.find((link) => link.short_code === code);
}

function linkItem(link) {
  const li = document.createElement("li");
  li.className = "link-item";
  const main = document.createElement("div");
  main.className = "link-main";
  const a = document.createElement("a");
  a.className = "link-url";
  a.textContent = link.short_url;
  const safe = safeURL(link.short_url);
  if (safe) {
    a.href = safe;
    a.target = "_blank";
    a.rel = "noopener noreferrer";
  }
  const meta = document.createElement("p");
  meta.className = "link-meta";
  meta.textContent = link.original_url;
  const sub = document.createElement("div");
  sub.className = "link-sub";
  const status = document.createElement("span");
  status.className = "status-pill";
  status.classList.add("is-" + linkStatus(link).toLowerCase());
  status.textContent = linkStatus(link);
  const date = document.createElement("span");
  date.className = "field-hint";
  date.textContent = link.expires_at ? "Até " + formatDate(link.expires_at) : "Sem validade";
  sub.append(status, date);
  main.append(a, meta, sub);
  const actions = document.createElement("div");
  actions.className = "link-actions";
  const details = document.createElement("button");
  details.className = "button button-secondary";
  details.type = "button";
  details.textContent = "Detalhes";
  details.dataset.code = link.short_code;
  details.addEventListener("click", () => selectLink(link.short_code));
  const copy = document.createElement("button");
  copy.className = "button button-secondary";
  copy.type = "button";
  copy.textContent = "Copiar";
  copy.addEventListener("click", () => copyText(link.short_url, "list-copy-msg"));
  actions.append(details, copy);
  li.append(main, actions);
  return li;
}

function renderLinks() {
  $("links").replaceChildren(...state.links.map(linkItem));
  $("link-count").textContent = state.total === 1 ? "1 link" : state.total + " links";
  $("empty-links").hidden = state.links.length > 0;
  $("more").hidden = state.links.length >= state.total;
  if (state.selectedCode) {
    const selected = linkByCode(state.selectedCode);
    if (selected) renderDetails(selected);
    else closeDetails();
  }
}

async function loadLinks(append) {
  if (state.listBusy && append) return;
  const id = ++state.listRequest;
  state.listBusy = true;
  $("list-loading").hidden = false;
  $("more").disabled = true;
  $("retry-list").hidden = true;
  notice("list-err", "");
  const offset = append ? state.links.length : 0;
  try {
    const data = await request(`/links?limit=${pageSize}&offset=${offset}`);
    if (id !== state.listRequest) return;
    const incoming = Array.isArray(data.links) ? data.links : [];
    state.links = append ? mergePage(state.links, incoming) : incoming;
    state.total = Number(data.total) || 0;
    renderLinks();
  } catch (error) {
    if (id === state.listRequest) {
      notice("list-err", error.message);
      $("retry-list").hidden = false;
    }
  } finally {
    if (id === state.listRequest) {
      state.listBusy = false;
      $("list-loading").hidden = true;
      $("more").disabled = false;
    }
  }
}

function renderDetails(link) {
  $("details-title").textContent = link.short_code;
  $("details-short-url").textContent = link.short_url;
  const safe = safeURL(link.short_url);
  if (safe) $("details-short-url").href = safe;
  else $("details-short-url").removeAttribute("href");
  $("details-destination").textContent = "Destino: " + link.original_url + " · " + (link.expires_at ? "Válido até " + formatDate(link.expires_at) : "Sem validade");
  const status = linkStatus(link);
  $("details-status").textContent = status;
  $("details-status").className = "status-pill is-" + status.toLowerCase();
  $("toggle-active").textContent = link.is_active ? "Desativar link" : linkStatus(link) === "Expirado" ? "Ajustar validade" : "Reativar link";
  $("details-copy-msg").textContent = "";
}

function closeDetails() {
  const code = state.selectedCode;
  state.selectedCode = "";
  state.statsRequest++;
  $("details").hidden = true;
  $("edit-form").hidden = true;
  const trigger = [...document.querySelectorAll(".link-item button[data-code]")]
    .find((button) => button.dataset.code === code);
  if (trigger) trigger.focus();
}

function selectLink(code) {
  const link = linkByCode(code);
  if (!link) return;
  if (state.selectedCode !== code) $("stats").replaceChildren();
  state.selectedCode = code;
  renderDetails(link);
  $("edit-form").hidden = true;
  notice("details-err", "");
  $("details").hidden = false;
  $("details").scrollIntoView({ behavior: "smooth", block: "start" });
  $("details-title").setAttribute("tabindex", "-1");
  $("details-title").focus();
  loadStats(code);
}

function barGroup(title, entries, labelKey) {
  const section = document.createElement("section");
  section.className = "analytics-bars";
  const heading = document.createElement("h4");
  heading.textContent = title;
  section.append(heading);
  if (!entries || entries.length === 0) {
    const empty = document.createElement("p");
    empty.className = "field-hint";
    empty.textContent = "Sem dados ainda.";
    section.append(empty);
    return section;
  }
  const max = Math.max(1, ...entries.map((item) => Number(item.count) || 0));
  entries.forEach((item) => {
    const row = document.createElement("div");
    row.className = "bar-row";
    const label = document.createElement("span");
    label.className = "bar-label";
    label.textContent = item[labelKey] || "Não informado";
    const count = document.createElement("span");
    count.textContent = String(item.count || 0);
    const progress = document.createElement("progress");
    progress.className = "bar-track";
    progress.max = max;
    progress.value = Number(item.count) || 0;
    progress.setAttribute("aria-label", label.textContent + ": " + count.textContent);
    row.append(label, progress, count);
    section.append(row);
  });
  return section;
}

function renderStats(data) {
  const root = $("stats");
  const grid = document.createElement("div");
  grid.className = "stat-grid";
  [["Cliques", data.total_clicks || 0], ["Visitantes únicos", data.unique_visitors || 0]].forEach(([label, value]) => {
    const card = document.createElement("div");
    card.className = "stat-card";
    const number = document.createElement("strong");
    number.textContent = Number(value).toLocaleString("pt-BR");
    const title = document.createElement("span");
    title.textContent = label;
    card.append(number, title);
    grid.append(card);
  });
  root.replaceChildren(
    grid,
    barGroup("Acessos por dia", data.clicks_by_day, "day"),
    barGroup("Origens", data.top_referrers, "name"),
    barGroup("Dispositivos", data.device_breakdown, "name"),
    barGroup("Navegadores", data.browser_breakdown, "name"),
  );
}

async function loadStats(code) {
  const id = ++state.statsRequest;
  $("stats-loading").hidden = false;
  $("refresh-stats").disabled = true;
  notice("stats-err", "");
  try {
    const data = await request("/links/" + encodeURIComponent(code) + "/analytics");
    if (id === state.statsRequest && state.selectedCode === code) renderStats(data);
  } catch (error) {
    if (id === state.statsRequest) notice("stats-err", error.message);
  } finally {
    if (id === state.statsRequest) {
      $("stats-loading").hidden = true;
      $("refresh-stats").disabled = false;
    }
  }
}

function openEdit() {
  const link = linkByCode(state.selectedCode);
  if (!link) return;
  $("edit-url").value = link.original_url;
  $("edit-expires").value = localDateTime(link.expires_at);
  clearFields(["edit-url", "edit-expires"]);
  notice("edit-err", "");
  $("edit-form").hidden = false;
  $("edit-url").focus();
}

async function submitEdit(event) {
  event.preventDefault();
  const link = linkByCode(state.selectedCode);
  if (!link) return;
  clearFields(["edit-url", "edit-expires"]);
  notice("edit-err", "");
  const url = $("edit-url").value.trim();
  const expiryChanged = $("edit-expires").value !== localDateTime(link.expires_at);
  const expires = expiryChanged ? expiryISO($("edit-expires").value) : "";
  let valid = true;
  if (!validURL(url)) valid = fieldError("edit-url", "Informe uma URL http ou https válida.") && valid;
  if (expiryChanged && expires === null) valid = fieldError("edit-expires", "Escolha uma data futura válida.") && valid;
  if (!valid) {
    document.querySelector('#edit-form [aria-invalid="true"]').focus();
    return;
  }
  const body = { url };
  if (expiryChanged) body.expires_at = expires;
  const button = $("save-edit");
  setBusy(button, true, "Salvando…");
  try {
    const updated = await request("/links/" + encodeURIComponent(link.short_code), { method: "PATCH", body: JSON.stringify(body) });
    state.links = state.links.map((item) => item.short_code === link.short_code ? updated : item);
    renderLinks();
    $("edit-form").hidden = true;
    notice("details-err", "Alterações salvas.", "success");
    $("edit-toggle").focus();
  } catch (error) {
    if (error.code === "invalid url") fieldError("edit-url", error.message);
    else if (error.code === "invalid expiration") fieldError("edit-expires", error.message);
    else notice("edit-err", error.message);
  } finally {
    setBusy(button, false);
  }
}

function confirmDeactivate() {
  const dialog = $("confirm-dialog");
  return new Promise((resolve) => {
    dialog.returnValue = "cancel";
    dialog.addEventListener("close", () => {
      $("toggle-active").focus();
      resolve(dialog.returnValue === "confirm");
    }, { once: true });
    dialog.showModal();
    $("confirm-cancel").focus();
  });
}

async function toggleActive() {
  const link = linkByCode(state.selectedCode);
  if (!link) return;
  if (!link.is_active && linkStatus(link) === "Expirado") {
    openEdit();
    notice("details-err", "Atualize ou remova a validade antes de reativar.");
    $("edit-expires").focus();
    return;
  }
  if (link.is_active && !(await confirmDeactivate())) return;
  const button = $("toggle-active");
  setBusy(button, true, link.is_active ? "Desativando…" : "Reativando…");
  notice("details-err", "");
  try {
    let updated;
    if (link.is_active) {
      await request("/links/" + encodeURIComponent(link.short_code), { method: "DELETE" });
      updated = { ...link, is_active: false };
    } else {
      updated = await request("/links/" + encodeURIComponent(link.short_code), {
        method: "PATCH",
        body: JSON.stringify({ is_active: true }),
      });
    }
    state.links = state.links.map((item) => item.short_code === link.short_code ? updated : item);
    renderLinks();
    notice("details-err", updated.is_active ? "Link reativado." : "Link desativado.", "success");
  } catch (error) {
    notice("details-err", error.message);
  } finally {
    setBusy(button, false);
    if (!$("details").hidden) button.focus();
  }
}

function bindEvents() {
  $("tab-login").addEventListener("click", () => setMode("login"));
  $("tab-register").addEventListener("click", () => setMode("register"));
  $("auth-form").addEventListener("submit", submitAuth);
  $("logout").addEventListener("click", logout);
  $("create-form").addEventListener("submit", submitCreate);
  $("copy-created").addEventListener("click", () => copyText($("created").textContent, "copy-msg"));
  $("more").addEventListener("click", () => loadLinks(true));
  $("retry-list").addEventListener("click", () => loadLinks(false));
  $("close-details").addEventListener("click", closeDetails);
  $("copy-details").addEventListener("click", () => {
    const link = linkByCode(state.selectedCode);
    if (link) copyText(link.short_url, "details-copy-msg");
  });
  $("edit-toggle").addEventListener("click", openEdit);
  $("cancel-edit").addEventListener("click", () => {
    $("edit-form").hidden = true;
    $("edit-toggle").focus();
  });
  $("edit-form").addEventListener("submit", submitEdit);
  $("toggle-active").addEventListener("click", toggleActive);
  $("refresh-stats").addEventListener("click", () => {
    if (state.selectedCode) loadStats(state.selectedCode);
  });
}

initTheme();
bindEvents();
setMode("login");
if (currentToken()) showApp();
else showAuth();
