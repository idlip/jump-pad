// Bare JS, no build step, no framework. One page, sections shown/hidden by
// the current path -- server sends this same file for every page route.

route();
wireRedirectForm();
wirePasteForm();
wirePastePreviewToggle();
wirePasteFileUpload();
wireExpiryCustomDate("redirect-expiry", "redirect-expiry-date-row");
wireExpiryCustomDate("paste-expiry", "paste-expiry-date-row");
hideTokenFieldsIfUnneeded();
showAdminLink();
initAppearance();

const maxPasteBytes = 500 * 1024; // must match the backend's cap

// hideTokenFieldsIfUnneeded hides the auth-token inputs when the server has no token configured.
function hideTokenFieldsIfUnneeded() {
  if (window.JUMPPAD_CONFIG.authRequired) return;
  document.getElementById("redirect-token-row").hidden = true;
  document.getElementById("paste-token-row").hidden = true;
}

// route shows the section matching location.pathname, and loads a paste if viewing one.
function route() {
  const sections = { "/": "landing", "/new-redirect": "new-redirect", "/new-paste": "new-paste", "/admin": "admin" };
  const path = location.pathname;
  const id = path.startsWith("/view/") ? path.slice("/view/".length) : null;
  const activeId = id ? "view-paste" : (sections[path] || "landing");

  for (const section of document.querySelectorAll("main > section")) {
    section.hidden = section.id !== activeId;
  }
  if (id) loadPaste(id);
  if (activeId === "admin") initAdmin();
}

// wireExpiryCustomDate shows the date input only when its select is set to "custom".
function wireExpiryCustomDate(selectId, rowId) {
  const select = document.getElementById(selectId);
  const row = document.getElementById(rowId);
  select.addEventListener("change", () => { row.hidden = select.value !== "custom"; });
}

// wireRedirectForm submits the redirect-creation form and shows the resulting short link.
function wireRedirectForm() {
  const form = document.getElementById("redirect-form");
  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    const out = document.getElementById("redirect-result");
    out.textContent = "Creating...";
    try {
      const slug = await submitForm(form, "/redirects", "redirect-token");
      out.textContent = "";
      renderLinkRow(out, location.origin + window.JUMPPAD_CONFIG.redirectPrefix + slug, "Your short link");
    } catch (err) {
      out.textContent = "Error: " + err.message;
    }
  });
}

// wirePasteForm submits the paste-creation form and shows both the highlighted and raw links.
function wirePasteForm() {
  const form = document.getElementById("paste-form");
  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    const out = document.getElementById("paste-result");
    out.textContent = "Creating...";
    try {
      const id = await submitForm(form, "/pastes", "paste-token");
      out.textContent = "";
      renderLinkRow(out, location.origin + "/view/" + id, "Highlighted view");
      renderLinkRow(out, location.origin + window.JUMPPAD_CONFIG.pastePrefix + id, "Raw text",
                    "works with curl too: curl " + location.origin + window.JUMPPAD_CONFIG.pastePrefix + id);
    } catch (err) {
      out.textContent = "Error: " + err.message;
    }
  });
}

// wirePastePreviewToggle swaps the paste textarea for a highlighted read-only preview, and back.
function wirePastePreviewToggle() {
  const toggle = document.getElementById("paste-preview-toggle");
  const textarea = document.getElementById("paste-content");
  const preview = document.getElementById("paste-preview");
  const language = document.getElementById("paste-language");

  toggle.addEventListener("click", async () => {
    const editing = preview.hidden;
    if (editing) {
      await highlightInto(document.getElementById("paste-highlight"), textarea.value, language.value);
    }
    preview.hidden = !editing;
    textarea.hidden = editing;
    toggle.textContent = editing ? "Edit" : "Preview";
    toggle.setAttribute("aria-pressed", editing ? "true" : "false");
  });
}

// wirePasteFileUpload loads a chosen text/code file into the content textarea, rejecting binaries and oversized files.
function wirePasteFileUpload() {
  const input = document.getElementById("paste-file");
  const status = document.getElementById("paste-file-status");
  const textarea = document.getElementById("paste-content");

  input.addEventListener("change", async () => {
    const file = input.files[0];
    status.textContent = "";
    if (!file) return;

    if (file.size > maxPasteBytes) {
      status.textContent = "Error: file is over the 500KB paste limit.";
      input.value = "";
      return;
    }

    const text = await file.text();
    if (looksBinary(text)) {
      status.textContent = "Error: that looks like a binary file -- only text/code files are supported.";
      input.value = "";
      return;
    }

    textarea.value = text;
    status.textContent = "Loaded " + file.name + " (" + text.length + " chars).";
  });
}

// looksBinary flags a NUL byte or a high ratio of non-printable control characters in a sample of text.
function looksBinary(text) {
  if (text.includes(String.fromCharCode(0))) return true;
  const sampleLen = Math.min(text.length, 2000);
  let nonPrintable = 0;
  for (let i = 0; i < sampleLen; i++) {
    const code = text.charCodeAt(i);
    if (code < 32 && code !== 9 && code !== 10 && code !== 13) nonPrintable++;
  }
  return sampleLen > 0 && nonPrintable / sampleLen > 0.01;
}

// loadPaste fetches a raw paste by id, renders it highlighted, and wires the copy/raw controls.
async function loadPaste(id) {
  const status = document.getElementById("view-status");
  const code = document.getElementById("view-code");
  try {
    const res = await fetch(window.JUMPPAD_CONFIG.pastePrefix + id);
    if (!res.ok) {
      status.textContent = res.status === 410 ? "This paste has expired." : "Paste not found.";
      return;
    }
    const text = await res.text();
    await highlightInto(code, text, res.headers.get("X-Paste-Language") || "auto");
    status.textContent = "";
    wireViewActions(id, text);
  } catch (err) {
    status.textContent = "Error loading paste: " + err.message;
  }
}

// wireViewActions shows and wires the Copy/Raw controls under a successfully loaded paste.
function wireViewActions(id, text) {
  const actions = document.getElementById("view-actions");
  const rawLink = document.getElementById("view-raw-link");
  const copyButton = document.getElementById("view-copy");

  rawLink.href = window.JUMPPAD_CONFIG.pastePrefix + id;
  copyButton.onclick = () => navigator.clipboard.writeText(text);
  actions.hidden = false;
}

// laod highlight.js only when necessary and required
function ensureHighlighter() {
  if (!window.jumppadHighlighter) {
    window.jumppadHighlighter = new Promise((resolve, reject) => {
      const style = document.createElement("link");
      style.rel = "stylesheet";
      style.href = "/static/vendor/highlight.min.css";

      const script = document.createElement("script");
      script.src = "/static/vendor/highlight.min.js";
      script.onload = resolve;
      script.onerror = () => reject(new Error("cannot load highlight.js"));

      document.head.append(style, script);
    });
  }
  return window.jumppadHighlighter;
}

// highlightInto renders text into code, using hljs's language grammar if
// known, else auto-detect.
async function highlightInto(code, text, language) {
  try {
    await ensureHighlighter();
  } catch {
    code.className = "";
    code.textContent = text;
    return;
  }

  code.removeAttribute("data-highlighted");
  if (language && language !== "auto") {
    code.textContent = text;
    code.className = "language-" + language;
    hljs.highlightElement(code);
  } else {
    code.className = "";
    code.innerHTML = hljs.highlightAuto(text).value;
  }
}

// resolveExpiry reads a form's expiry select, substituting the paired date input's value when set to "custom".
function resolveExpiry(form) {
  const select = form.querySelector('select[name="expiry"]');
  if (!select) return null;
  if (select.value === "custom") {
    return document.getElementById(select.id + "-date").value;
  }
  return select.value;
}

// submitForm posts form as x-www-form-urlencoded, with the token input as X-Auth-Token, and returns the body text.
async function submitForm(form, path, tokenInputId) {
  const body = new URLSearchParams(new FormData(form));
  const expiry = resolveExpiry(form);
  if (expiry !== null) body.set("expiry", expiry);

  const token = document.getElementById(tokenInputId).value;
  const headers = { "Content-Type": "application/x-www-form-urlencoded" };
  if (token) headers["X-Auth-Token"] = token;

  const res = await fetch(path, { method: "POST", headers, body });
  const text = await res.text();
  if (!res.ok) throw new Error(text);
  return text;
}

// renderLinkRow appends "label: [url] [Copy]" (plus an optional hint line) to out.
function renderLinkRow(out, url, label, hint) {
  const row = document.createElement("div");

  const text = document.createElement("input");
  text.type = "text";
  text.readOnly = true;
  text.value = url;
  text.setAttribute("aria-label", label);

  const button = document.createElement("button");
  button.type = "button";
  button.textContent = "Copy";

  const status = document.createElement("span");
  status.setAttribute("role", "status");
  button.addEventListener("click", async () => {
    await navigator.clipboard.writeText(url);
    status.textContent = "Copied -- you can share or open this now.";
    setTimeout(() => { status.textContent = ""; }, 4000);
  });

  row.append(document.createTextNode(label + ": "), text, button, status);
  out.append(row);

  if (hint) {
    const hintRow = document.createElement("p");
    hintRow.textContent = hint;
    out.append(hintRow);
  }
}

// ---- Admin page ----------------------------------------------------------
// One table per item type, one dialog for both add and edit. The token
// lives in sessionStorage, so a reload keeps it and closing the tab drops
// it. No function below holds module level state: initAdmin builds one
// state object and passes it in, which keeps the page reloadable.

// showAdminLink reveals the Admin link when the server has an admin token.
function showAdminLink() {
  if (window.JUMPPAD_CONFIG.adminEnabled) {
    document.getElementById("nav-admin").hidden = false;
  }
}

// adminToken reads the token that this tab holds.
function adminToken() {
  return sessionStorage.getItem("jumppad-admin-token") || "";
}

// initAdmin wires the token form, the two tables, and the dialog.
function initAdmin() {
  const status = document.getElementById("admin-status");
  if (!window.JUMPPAD_CONFIG.adminEnabled) {
    status.textContent = "The admin page is off. Set admin_token in the server configuration to switch it on.";
    document.getElementById("admin-auth").hidden = true;
    return;
  }

  const state = { redirects: [], pastes: [], sort: { redirects: null, pastes: null } };

  document.getElementById("admin-auth").addEventListener("submit", (event) => {
    event.preventDefault();
    sessionStorage.setItem("jumppad-admin-token", document.getElementById("admin-token").value);
    loadAdminItems(state);
  });
  document.getElementById("admin-reload").addEventListener("click", () => loadAdminItems(state));
  document.getElementById("admin-forget").addEventListener("click", () => {
    sessionStorage.removeItem("jumppad-admin-token");
    location.reload();
  });
  document.getElementById("admin-show-expired").addEventListener("change", () => renderAdminTables(state));
  document.getElementById("admin-add-redirect").addEventListener("click", () => openAdminDialog("redirect", null));
  document.getElementById("admin-add-paste").addEventListener("click", () => openAdminDialog("paste", null));
  document.getElementById("admin-dialog-cancel").addEventListener("click", () => document.getElementById("admin-dialog").close());
  document.getElementById("admin-form").addEventListener("submit", (event) => {
    event.preventDefault();
    saveAdminItem(state);
  });

  wireAdminSorting(state, "redirects");
  wireAdminSorting(state, "pastes");

  if (adminToken()) {
    document.getElementById("admin-token").value = adminToken();
    loadAdminItems(state);
  }
}

// adminRequest sends one admin call with the token in the header. It
// returns the parsed body, or throws with the message from the server.
async function adminRequest(method, path, body) {
  const options = { method, headers: { "X-Auth-Token": adminToken() } };
  if (body) {
    options.headers["Content-Type"] = "application/json";
    options.body = JSON.stringify(body);
  }

  const res = await fetch(path, options);
  if (res.status === 204) return null;

  const text = await res.text();
  const parsed = text ? JSON.parse(text) : null;
  if (!res.ok) throw new Error(parsed && parsed.error ? parsed.error : res.status + " " + res.statusText);
  return parsed;
}

// loadAdminItems reads both tables and renders them.
async function loadAdminItems(state) {
  const status = document.getElementById("admin-status");
  status.textContent = "Loading...";
  try {
    const items = await adminRequest("GET", "/admin/api/items");
    state.redirects = items.redirects || [];
    state.pastes = items.pastes || [];
    document.getElementById("admin-auth").hidden = true;
    document.getElementById("admin-tables").hidden = false;
    status.textContent = "";
    renderAdminTables(state);
  } catch (err) {
    status.textContent = "Error: " + err.message;
    document.getElementById("admin-auth").hidden = false;
    document.getElementById("admin-tables").hidden = true;
  }
}

// renderAdminTables redraws both tables from the state.
function renderAdminTables(state) {
  renderAdminRows(state, "redirects", ["slug", "target_url", "created_at", "expires_at"]);
  renderAdminRows(state, "pastes", ["id", "language", "created_at", "expires_at"]);
}

// renderAdminRows redraws one table body, hiding expired rows unless the
// checkbox asks for them.
function renderAdminRows(state, collection, columns) {
  const table = document.getElementById("admin-" + collection);
  const body = table.querySelector("tbody");
  const showExpired = document.getElementById("admin-show-expired").checked;
  const order = state.sort[collection];

  let rows = state[collection].filter((row) => showExpired || !isExpired(row));
  if (order) rows = sortAdminRows(rows, order.column, order.ascending);

  body.replaceChildren();
  for (const row of rows) {
    const line = document.createElement("tr");
    if (isExpired(row)) line.setAttribute("data-expired", "true");
    for (const column of columns) line.append(adminCell(collection, row, column));
    line.append(adminActions(state, collection, row));
    body.append(line);
  }

  const hidden = state[collection].length - rows.length;
  table.setAttribute("aria-label", collection + ": " + rows.length + " shown, " + hidden + " expired and hidden");
}

// adminCell builds one table cell. The name cell links to the live item.
function adminCell(collection, row, column) {
  const cell = document.createElement("td");
  const value = row[column];

  if (column === "slug" || column === "id") {
    const link = document.createElement("a");
    link.href = collection === "redirects"
      ? window.JUMPPAD_CONFIG.redirectPrefix + value
      : "/view/" + value;
    link.textContent = value;
    cell.append(link);
    return cell;
  }

  if (column === "created_at") {
    cell.textContent = formatUnix(value);
    return cell;
  }
  if (column === "expires_at") {
    cell.textContent = value === null ? "forever" : formatUnix(value);
    if (isExpired(row)) cell.textContent += " (expired)";
    return cell;
  }

  cell.textContent = value || "";
  cell.title = value || "";
  return cell;
}

// adminActions builds the Edit and Remove cell for one row.
function adminActions(state, collection, row) {
  const cell = document.createElement("td");
  const kind = collection === "redirects" ? "redirect" : "paste";
  const name = row.slug || row.id;

  const edit = document.createElement("button");
  edit.type = "button";
  edit.textContent = "Edit";
  edit.addEventListener("click", () => openAdminDialog(kind, row));

  const remove = document.createElement("button");
  remove.type = "button";
  remove.textContent = "Remove";
  remove.addEventListener("click", () => removeAdminItem(state, kind, name));

  cell.append(edit, remove);
  return cell;
}

// wireAdminSorting makes each marked header sort its own table.
function wireAdminSorting(state, collection) {
  const table = document.getElementById("admin-" + collection);
  for (const header of table.querySelectorAll("th[data-sort]")) {
    header.addEventListener("click", () => {
      const column = header.dataset.sort;
      const order = state.sort[collection];
      const ascending = !(order && order.column === column && order.ascending);
      state.sort[collection] = { column, ascending };

      for (const other of table.querySelectorAll("th[data-sort]")) other.removeAttribute("aria-sort");
      header.setAttribute("aria-sort", ascending ? "ascending" : "descending");
      renderAdminTables(state);
    });
    header.addEventListener("keydown", (event) => {
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        header.click();
      }
    });
  }
}

// sortAdminRows returns a sorted copy, with empty values last.
function sortAdminRows(rows, column, ascending) {
  const direction = ascending ? 1 : -1;
  return [...rows].sort((left, right) => compareAdminValues(left[column], right[column]) * direction);
}

// compareAdminValues orders two cell values, and keeps an empty value last.
function compareAdminValues(left, right) {
  if (left === right) return 0;
  if (left === null || left === undefined || left === "") return 1;
  if (right === null || right === undefined || right === "") return -1;
  return left > right ? 1 : -1;
}

// isExpired says whether a row is past its expiry time.
function isExpired(row) {
  return row.expires_at !== null && row.expires_at !== undefined && row.expires_at * 1000 < Date.now();
}

// formatUnix shows a unix time in the reader's own locale.
function formatUnix(seconds) {
  return new Date(seconds * 1000).toLocaleString();
}

// openAdminDialog fills the one dialog for the four cases: add or edit, a
// redirect or a paste. A null row means add.
function openAdminDialog(kind, row) {
  const dialog = document.getElementById("admin-dialog");
  const isPaste = kind === "paste";
  const name = row ? (row.slug || row.id) : "";

  dialog.dataset.kind = kind;
  dialog.dataset.original = name;
  document.getElementById("admin-dialog-title").textContent = (row ? "Edit " : "Add a ") + kind;
  document.getElementById("admin-dialog-error").textContent = "";

  document.getElementById("admin-row-target").hidden = isPaste;
  document.getElementById("admin-row-language").hidden = !isPaste;
  document.getElementById("admin-row-content").hidden = !isPaste;
  document.getElementById("admin-field-target").required = !isPaste;
  document.getElementById("admin-field-content").required = isPaste;
  document.getElementById("admin-field-slug").required = !isPaste || Boolean(row);

  document.getElementById("admin-field-slug").value = name;
  document.getElementById("admin-field-target").value = row ? (row.target_url || "") : "";
  document.getElementById("admin-field-language").value = row ? (row.language || "") : "";
  document.getElementById("admin-field-content").value = "";
  document.getElementById("admin-field-expiry").value = "";
  document.getElementById("admin-expiry-hint").textContent = adminExpiryHint(row);

  dialog.showModal();
  if (isPaste && row) loadAdminPasteContent(row.id);
}

// adminExpiryHint warns that a save replaces every field, so an empty
// expiry box means forever, and not "keep what it has now".
function adminExpiryHint(row) {
  const rules = "Empty means forever. Also takes 1d, 1w, 1m, a date such as 2027-01-01, or a duration such as 72h.";
  if (!row) return rules;
  const now = row.expires_at === null ? "forever" : formatUnix(row.expires_at);
  return "Now: " + now + ". A save replaces it. " + rules;
}

// loadAdminPasteContent fills the content box, expired paste included.
async function loadAdminPasteContent(id) {
  const field = document.getElementById("admin-field-content");
  field.value = "Loading...";
  try {
    const one = await adminRequest("GET", "/admin/api/pastes/" + encodeURIComponent(id));
    field.value = one.content || "";
  } catch (err) {
    field.value = "";
    document.getElementById("admin-dialog-error").textContent = "Error reading the content: " + err.message;
  }
}

// saveAdminItem sends an add or a full replacement, then reloads the list.
async function saveAdminItem(state) {
  const dialog = document.getElementById("admin-dialog");
  const kind = dialog.dataset.kind;
  const original = dialog.dataset.original;
  const error = document.getElementById("admin-dialog-error");

  const body = {
    slug: document.getElementById("admin-field-slug").value,
    expiry: document.getElementById("admin-field-expiry").value,
  };
  if (kind === "redirect") {
    body.target_url = document.getElementById("admin-field-target").value;
  } else {
    body.content = document.getElementById("admin-field-content").value;
    body.language = document.getElementById("admin-field-language").value;
  }

  const collection = kind === "redirect" ? "redirects" : "pastes";
  const path = original
    ? "/admin/api/" + collection + "/" + encodeURIComponent(original)
    : "/admin/api/" + collection;

  error.textContent = "Saving...";
  try {
    await adminRequest(original ? "PUT" : "POST", path, body);
    dialog.close();
    await loadAdminItems(state);
  } catch (err) {
    error.textContent = "Error: " + err.message;
  }
}

// removeAdminItem asks once, then removes the row. There is no undo.
async function removeAdminItem(state, kind, name) {
  if (!confirm("Remove the " + kind + " " + name + "? This cannot be undone.")) return;
  const collection = kind === "redirect" ? "redirects" : "pastes";
  try {
    await adminRequest("DELETE", "/admin/api/" + collection + "/" + encodeURIComponent(name));
    await loadAdminItems(state);
  } catch (err) {
    document.getElementById("admin-status").textContent = "Error: " + err.message;
  }
}

// ---- Appearance: theme and accent ----------------------------------------
// The palette already uses light-dark(), so a theme change is one
// color-scheme swap on the root element, and no second palette exists. The
// accent is --base09, so the picker overrides one custom property. Both
// choices live in localStorage. The first visit follows the operating
// system, and the first click pins a theme from then on.

// defaultAccent is the base16-default orange that style.css ships with.
function defaultAccent() {
  return "#dc9656";
}

// initAppearance applies the stored choices, then wires the three controls.
function initAppearance() {
  applyStoredTheme();
  applyStoredAccent();

  document.getElementById("theme-toggle").addEventListener("click", () => {
    setTheme(currentTheme() === "dark" ? "light" : "dark");
  });
  document.getElementById("accent-color").addEventListener("input", (event) => {
    setAccent(event.target.value);
  });
  document.getElementById("accent-reset").addEventListener("click", resetAccent);

  watchSystemTheme();
}

// currentTheme returns the pinned theme, or the one the system asks for.
function currentTheme() {
  const pinned = localStorage.getItem("jumppad-theme");
  if (pinned === "dark" || pinned === "light") return pinned;
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

// applyStoredTheme pins the stored theme. With nothing stored it leaves
// the stylesheet to follow the system, and only labels the button.
function applyStoredTheme() {
  const pinned = localStorage.getItem("jumppad-theme");
  if (pinned === "dark" || pinned === "light") {
    applyTheme(pinned);
    return;
  }
  labelThemeToggle(currentTheme());
}

// setTheme pins a theme for every later visit on this browser.
function setTheme(theme) {
  localStorage.setItem("jumppad-theme", theme);
  applyTheme(theme);
}

// applyTheme swaps color-scheme on the root element, which flips every
// light-dark() color at once.
function applyTheme(theme) {
  document.documentElement.style.colorScheme = theme;
  labelThemeToggle(theme);
}

// labelThemeToggle names the theme that a click switches to, so the button
// says what it does and not what the page already is.
function labelThemeToggle(theme) {
  const toggle = document.getElementById("theme-toggle");
  const other = theme === "dark" ? "light" : "dark";
  toggle.textContent = other === "dark" ? "Dark" : "Light";
  toggle.setAttribute("aria-label", "Switch to the " + other + " theme");
  toggle.title = "Switch to the " + other + " theme";
}

// watchSystemTheme keeps the button label right while no theme is pinned
// and the system flips, for example at sunset.
function watchSystemTheme() {
  window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => {
    if (!localStorage.getItem("jumppad-theme")) labelThemeToggle(currentTheme());
  });
}

// applyStoredAccent overrides --base09 with the stored color, and fills
// the picker either way.
function applyStoredAccent() {
  const picker = document.getElementById("accent-color");
  const stored = localStorage.getItem("jumppad-accent");
  picker.value = stored || defaultAccent();
  if (stored) document.documentElement.style.setProperty("--base09", stored);
}

// setAccent stores one color and applies it to links, buttons, and the
// focus ring, which are the three places --base09 reaches.
function setAccent(color) {
  localStorage.setItem("jumppad-accent", color);
  document.documentElement.style.setProperty("--base09", color);
}

// resetAccent drops back to the color that the stylesheet ships with.
function resetAccent() {
  localStorage.removeItem("jumppad-accent");
  document.documentElement.style.removeProperty("--base09");
  document.getElementById("accent-color").value = defaultAccent();
}
