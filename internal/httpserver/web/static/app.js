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

  toggle.addEventListener("click", () => {
    const editing = preview.hidden;
    if (editing) {
      highlightInto(document.getElementById("paste-highlight"), textarea.value, language.value);
    }
    preview.hidden = !editing;
    textarea.hidden = editing;
    toggle.textContent = editing ? "Edit" : "Preview";
    toggle.setAttribute("aria-pressed", editing ? "true" : "false");
  });
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
    highlightInto(code, text, res.headers.get("X-Paste-Language") || "auto");
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

// highlightInto renders text into code, using hljs's language grammar if known, else auto-detect.
function highlightInto(code, text, language) {
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
