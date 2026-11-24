// rungkit in the browser. All work happens in rungkit.wasm (Go); this file
// only wires the page to window.rungkit, whose functions return JSON strings.
"use strict";

const $ = (id) => document.getElementById(id);
const store = {
  get(k) { try { return localStorage.getItem("rungkit." + k); } catch { return null; } },
  set(k, v) { try { localStorage.setItem("rungkit." + k, v); } catch { /* private mode */ } },
};

const T = {
  ja: {
    tagline: "ラダーをテキストで書き、その場で図と動作を確かめる",
    example: "例を開く", exConveyor: "コンベア（三菱）", exVerify: "三菱の動作確認用", exIec: "IEC の例",
    open: "開く", save: ".ldt を保存", format: "整形", export: "書き出し",
    program: "プログラム（.ldt）", ladder: "ラダー図", clickHint: "要素をクリックして選び、リボンで編集。部品は左からドラッグ（Delete で削除、Esc で解除）",
    parts: "部品", cancel: "やめる", ok: "OK",
    tabLadder: "ラダー", tabSim: "シミュレーション", tabIo: "ファイル・書き出し", browser: "ブラウザ", paneText: "テキスト", paneTree: "構成",
    gCreate: "作成", gModify: "変更", gRemove: "削除・履歴", gText: "テキスト", gScenario: "シナリオ",
    zoomIn: "拡大", zoomOut: "縮小", zoomFit: "幅に合わせる", treeProgram: "プログラム", treeRungs: "ラング", treeSymbols: "デバイス",
    pNo: "a接点（ドラッグして図に置く）", pNc: "b接点", pRise: "立上り", pFall: "立下り", pCmp: "比較",
    pOut: "コイル OUT", pSet: "SET", pRst: "RST", pTon: "タイマー", pCtu: "カウンタ",
    askDevice: "デバイス", askCompare: "比較の式", askTimer: "タイマーとその設定値", askCounter: "カウンタとその設定値",
    hintDevice: "例: X0、M10、Y1", hintCompare: "例: D0 >= K100（= <> < <= > >=）", hintTimer: "例: T0, K50（三菱は100ms単位で K50 = 5秒）", hintCounter: "例: C0, K10",
    askNewRung: "新しいラング", hintRung: "条件 -> 出力 の形。例: X0 & !X1 -> OUT(Y0)",
    dropBefore: "前に入れる", dropAfter: "後に入れる", dropParallel: "並列に足す", dropReplace: "置き換える", dropRung: "新しいラングにする",
    gFile: "ファイル", gEdit: "図で編集", gExport: "書き出し", themeDark: "暗いテーマにする", themeLight: "明るいテーマにする",
    addRung: "ラング追加", before: "前に条件", after: "後に条件", parallel: "並列に追加", replace: "置き換え",
    operand: "デバイス変更", delete: "削除", deleteRung: "ラング削除", undo: "元に戻す",
    selected: (r, e) => e ? `選択中: ${r} / ${e}` : `選択中: ${r}`,
    askRung: "ラングの式（例: (X0 | Y0) & !X1 -> OUT(Y0)）", askCond: "足す条件（例: !X3、rise(X4)、[D0 >= K10]）",
    askParallel: "並列に足すもの（条件なら X5、出力の横なら SET(M1) など）", askReplace: "置き換える要素（例: !X2、OUT(Y1)）",
    askOperand: "新しいデバイス（例: X10）", editFailed: "編集できません: ",
    dropComments: "図で編集すると、テキストは整形され、// で書いたコメントは消えます（ラングのコメントと sym は残ります）。続けますか？",
    scenario: "シナリオ（.ldtest）", trace: "記録を表示", run: "実行",
    foot: "ブラウザの中だけで動きます。入力はこの端末に保存されます。",
    loading: "読み込み中…", rungs: (n) => `${n} ラング`, errors: (e, w) => `エラー ${e} / 警告 ${w}`,
    passed: (p, n) => `${n} 件中 ${p} 件成功`, scans: (n) => `${n} スキャン`, noDrawing: "エラーを直すと図が出ます",
    exportFailed: "書き出せません: ", omitted: (n) => `（途中の ${n} 件は省略）`,
  },
  en: {
    tagline: "Write ladder as text; see the diagram and the behavior as you type",
    example: "Open an example", exConveyor: "Conveyor (MELSEC)", exVerify: "MELSEC behavior check", exIec: "IEC example",
    open: "Open", save: "Save .ldt", format: "Format", export: "Export",
    program: "Program (.ldt)", ladder: "Ladder", clickHint: "Click an element, then edit from the ribbon; drag parts from the left (Delete removes, Esc deselects)",
    parts: "Parts", cancel: "Cancel", ok: "OK",
    tabLadder: "Ladder", tabSim: "Simulation", tabIo: "File & export", browser: "Browser", paneText: "Text", paneTree: "Structure",
    gCreate: "Create", gModify: "Modify", gRemove: "Delete & history", gText: "Text", gScenario: "Scenario",
    zoomIn: "Zoom in", zoomOut: "Zoom out", zoomFit: "Fit width", treeProgram: "Program", treeRungs: "Rungs", treeSymbols: "Devices",
    pNo: "NO contact (drag onto the diagram)", pNc: "NC contact", pRise: "Rising edge", pFall: "Falling edge", pCmp: "Comparison",
    pOut: "Coil OUT", pSet: "SET", pRst: "RST", pTon: "Timer", pCtu: "Counter",
    askDevice: "Device", askCompare: "Comparison", askTimer: "Timer and preset", askCounter: "Counter and preset",
    hintDevice: "e.g. X0, M10, Y1", hintCompare: "e.g. D0 >= K100 (= <> < <= > >=)", hintTimer: "e.g. T0, K50 (MELSEC counts 100 ms, so K50 = 5 s)", hintCounter: "e.g. C0, K10",
    askNewRung: "New rung", hintRung: "condition -> output, e.g. X0 & !X1 -> OUT(Y0)",
    dropBefore: "Insert before", dropAfter: "Insert after", dropParallel: "Add in parallel", dropReplace: "Replace", dropRung: "Make a new rung",
    gFile: "File", gEdit: "Edit on diagram", gExport: "Export", themeDark: "Switch to dark theme", themeLight: "Switch to light theme",
    addRung: "Add rung", before: "Condition before", after: "Condition after", parallel: "Add in parallel", replace: "Replace",
    operand: "Change device", delete: "Delete", deleteRung: "Delete rung", undo: "Undo",
    selected: (r, e) => e ? `Selected: ${r} / ${e}` : `Selected: ${r}`,
    askRung: "Rung (e.g. (X0 | Y0) & !X1 -> OUT(Y0))", askCond: "Condition to add (e.g. !X3, rise(X4), [D0 >= K10])",
    askParallel: "What to add in parallel (a condition such as X5, or next to an output SET(M1))", askReplace: "Replacement (e.g. !X2, OUT(Y1))",
    askOperand: "New device (e.g. X10)", editFailed: "Cannot edit: ",
    dropComments: "Editing on the diagram reformats the text, and // comments are removed (rung comments and sym stay). Continue?",
    scenario: "Scenario (.ldtest)", trace: "Show trace", run: "Run",
    foot: "Runs entirely in your browser. Your input is kept on this device.",
    loading: "Loading…", rungs: (n) => `${n} rungs`, errors: (e, w) => `${e} errors / ${w} warnings`,
    passed: (p, n) => `${p} of ${n} passed`, scans: (n) => `${n} scans`, noDrawing: "Fix the errors to see the diagram",
    exportFailed: "Cannot export: ", omitted: (n) => ` (${n} in the middle omitted)`,
  },
};
let lang = store.get("lang") || (navigator.language.startsWith("ja") ? "ja" : "en");
const t = (k, ...a) => { const v = T[lang][k]; return typeof v === "function" ? v(...a) : v; };

function applyLanguage() {
  document.documentElement.lang = lang;
  document.querySelectorAll("[data-i18n]").forEach((el) => { el.textContent = t(el.dataset.i18n); });
  document.querySelectorAll("[data-i18n-title]").forEach((el) => { el.title = t(el.dataset.i18nTitle); el.setAttribute("aria-label", el.title); });
  $("lang").textContent = lang === "ja" ? "EN" : "日本語";
  applyTheme();
  if (window.rungkit) { rungkit.setLanguage(lang); refresh(); }
}

const EXAMPLES = {
  conveyor: ["examples/conveyor.ldt", "examples/conveyor.ldtest"],
  "melsec-semantics": ["examples/melsec-semantics.ldt", "examples/melsec-semantics.ldtest"],
  iec: ["examples/iec.ldt", "examples/iec.ldtest"],
};

// ---- notices ----

let toastTimer = 0;
function toast(msg) {
  const el = $("toast");
  el.textContent = msg;
  el.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { el.hidden = true; }, 7000);
}

// ---- theme ----

function applyTheme() {
  const dark = document.documentElement.dataset.theme !== "light";
  $("theme").querySelector("use").setAttribute("href", dark ? "#i-sun" : "#i-moon");
  $("theme").title = t(dark ? "themeLight" : "themeDark");
}
function toggleTheme() {
  const next = document.documentElement.dataset.theme === "light" ? "dark" : "light";
  document.documentElement.dataset.theme = next;
  store.set("theme", next);
  applyTheme();
}

// ---- editor helpers ----

function jumpTo(area, line, col) {
  const lines = area.value.split("\n");
  let pos = 0;
  for (let i = 0; i < line - 1 && i < lines.length; i++) pos += lines[i].length + 1;
  pos += Math.max(0, (col || 1) - 1);
  area.focus();
  area.setSelectionRange(pos, pos + 1);
  const lh = parseFloat(getComputedStyle(area).lineHeight) || 20;
  area.scrollTop = Math.max(0, (line - 4) * lh);
}

// utf16le encodes text as UTF-16 LE with a byte order mark.
function utf16le(text) {
  const b = new Uint8Array(2 + text.length * 2);
  b[0] = 0xff; b[1] = 0xfe;
  for (let i = 0; i < text.length; i++) { const c = text.charCodeAt(i); b[2 + 2 * i] = c & 0xff; b[3 + 2 * i] = c >> 8; }
  return b;
}

function download(name, text, type) {
  const url = URL.createObjectURL(new Blob([text], { type: type || "text/plain;charset=utf-8" }));
  const a = Object.assign(document.createElement("a"), { href: url, download: name });
  document.body.append(a); a.click(); a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

// ---- program ----

let fileName = store.get("name") || "program.ldt";
let positions = {};

function refresh() {
  const src = $("src").value;
  store.set("src", src);
  const r = JSON.parse(rungkit.update(fileName, src));
  const status = $("status"), list = $("diags"), svg = $("svg");
  $("docname").textContent = fileName;
  list.replaceChildren();
  if (!r.ok) {
    status.textContent = r.error; status.classList.add("bad");
    $("dot").className = "dot bad";
    svg.innerHTML = `<div class="placeholder">${t("noDrawing")}</div>`;
    const li = document.createElement("li");
    li.className = "error";
    li.innerHTML = `<code>SYNTAX</code>`;
    li.append(r.error);
    if (r.line) li.onclick = () => jumpTo($("src"), r.line, r.col);
    list.append(li);
    positions = {};
    buildTree(null);
    return;
  }
  positions = r.positions || {};
  const errs = r.diagnostics.filter((d) => d.severity === "error").length;
  status.textContent = `${r.profile} · ${t("rungs", r.rungs)} · ${t("errors", errs, r.diagnostics.length - errs)}`;
  status.classList.toggle("bad", errs > 0);
  $("dot").className = "dot " + (errs > 0 ? "bad" : "ok");
  for (const d of r.diagnostics) {
    const li = document.createElement("li");
    li.className = d.severity;
    const code = document.createElement("code"); code.textContent = d.code;
    const where = document.createElement("span"); where.className = "at";
    where.textContent = d.line ? `${d.line}:${d.col}` : [d.rung, d.elem].filter(Boolean).join("/");
    li.append(code, where, d.message);
    if (d.line) li.onclick = () => jumpTo($("src"), d.line, d.col);
    list.append(li);
  }
  svg.innerHTML = r.svg || "";
  applyZoom();
  buildTree(r);
}

let timer = 0;
function scheduleRefresh() { clearTimeout(timer); timer = setTimeout(refresh, 200); }

// ---- editing on the diagram ----

// The diagram edits go through the same edit commands as MCP; the result
// replaces the text, so the text stays the single source of truth.
let sel = { rung: "", elem: "" };
const editHistory = [];

function select(rung, elem) {
  sel = { rung: rung || "", elem: elem || "" };
  document.querySelectorAll("#svg .selected").forEach((x) => x.classList.remove("selected"));
  if (elem) document.querySelector(`#svg g.elem[data-elem="${elem}"]`)?.classList.add("selected");
  else if (rung) document.querySelector(`#svg g.rung[data-rung="${rung}"]`)?.classList.add("selected");
  $("sel").textContent = rung ? t("selected", rung, elem) : "";
  document.querySelectorAll("#tree .row.selected").forEach((x) => x.classList.remove("selected"));
  if (rung) document.querySelector(`#tree .row[data-rung="${rung}"]`)?.classList.add("selected");
  document.querySelectorAll(".edit-bar [data-need]").forEach((b) => {
    b.disabled = b.dataset.need === "elem" ? !sel.elem : !sel.rung;
  });
  $("undo").disabled = editHistory.length === 0;
}

function applyEdit(ops) {
  const before = $("src").value;
  if (/\/\//.test(before) && !editHistory.length && !confirm(t("dropComments"))) return;
  const r = JSON.parse(rungkit.edit(JSON.stringify(ops)));
  if (!r.ok) { toast(t("editFailed") + r.error); refresh(); return; }
  editHistory.push(before);
  $("src").value = r.text;
  refresh();
  select("", "");
}

// askText shows the input dialog and resolves to the trimmed text, or null.
function askText(labelKey, initial = "", hintKey = "") {
  const dlg = $("ask"), input = $("ask-input");
  $("ask-label").textContent = t(labelKey);
  $("ask-hint").textContent = hintKey ? t(hintKey) : "";
  input.value = initial;
  return new Promise((done) => {
    dlg.onclose = () => {
      const v = dlg.returnValue === "ok" ? input.value.trim() : "";
      done(v || null);
    };
    dlg.returnValue = "";
    dlg.showModal();
    input.select();
  });
}

async function editAction(act) {
  let text;
  switch (act) {
    case "addRung":
      if ((text = await askText("askRung", "", "hintRung"))) applyEdit([{ op: "add_rung", text, after: sel.rung }]);
      break;
    case "before": case "after":
      if ((text = await askText("askCond", "", "hintDevice"))) applyEdit([{ op: "insert_series", target: sel.elem, position: act, text }]);
      break;
    case "parallel":
      if ((text = await askText("askParallel"))) applyEdit([{ op: "insert_parallel", target: sel.elem, text }]);
      break;
    case "replace":
      if ((text = await askText("askReplace"))) applyEdit([{ op: "replace_element", target: sel.elem, text }]);
      break;
    case "operand":
      if ((text = await askText("askOperand", "", "hintDevice"))) applyEdit([{ op: "set_operand", target: sel.elem, operand: text }]);
      break;
    case "delete":
      applyEdit([{ op: "delete_element", target: sel.elem }]);
      break;
    case "deleteRung":
      applyEdit([{ op: "delete_rung", rung: sel.rung }]);
      break;
    case "undo":
      if (editHistory.length) { $("src").value = editHistory.pop(); refresh(); select("", ""); }
      break;
  }
}

// ---- parts palette ----

// Each part asks for what it needs and turns it into ladder text.
const PARTS = {
  no:   { out: false, ask: "askDevice",  hint: "hintDevice",  def: "X0",         text: (v) => v },
  nc:   { out: false, ask: "askDevice",  hint: "hintDevice",  def: "X1",         text: (v) => "!" + v },
  rise: { out: false, ask: "askDevice",  hint: "hintDevice",  def: "X0",         text: (v) => `rise(${v})` },
  fall: { out: false, ask: "askDevice",  hint: "hintDevice",  def: "X0",         text: (v) => `fall(${v})` },
  cmp:  { out: false, ask: "askCompare", hint: "hintCompare", def: "D0 >= K10",  text: (v) => `[${v}]` },
  out:  { out: true,  ask: "askDevice",  hint: "hintDevice",  def: "Y0",         text: (v) => `OUT(${v})` },
  set:  { out: true,  ask: "askDevice",  hint: "hintDevice",  def: "M0",         text: (v) => `SET(${v})` },
  rst:  { out: true,  ask: "askDevice",  hint: "hintDevice",  def: "M0",         text: (v) => `RST(${v})` },
  ton:  { out: true,  ask: "askTimer",   hint: "hintTimer",   def: "T0, K50",    text: (v) => `TON(${v})` },
  ctu:  { out: true,  ask: "askCounter", hint: "hintCounter", def: "C0, K10",    text: (v) => `CTU(${v})` },
};

// zoneOf says where a drop on element g at (x, y) goes: before, after,
// parallel (lower part) or replace (middle).
function zoneOf(g, x, y) {
  const r = g.querySelector("rect.hit").getBoundingClientRect();
  const fx = (x - r.left) / r.width, fy = (y - r.top) / r.height;
  if (fy > 0.72) return "parallel";
  if (fx < 0.3) return "before";
  if (fx > 0.7) return "after";
  return "replace";
}

async function placePart(key, target) {
  const part = PARTS[key];
  if (!part) return;
  const v = await askText(part.ask, part.def, part.hint);
  if (!v) return;
  const text = part.text(v);
  if (target.zone === "rung") {
    const rung = part.out ? `X0 -> ${text}` : `${text} -> OUT(Y0)`;
    const full = await askText("askNewRung", rung, "hintRung");
    if (full) applyEdit([{ op: "add_rung", text: full, after: target.rung || "" }]);
    return;
  }
  const op = {
    before: { op: "insert_series", target: target.elem, position: "before", text },
    after: { op: "insert_series", target: target.elem, position: "after", text },
    parallel: { op: "insert_parallel", target: target.elem, text },
    replace: { op: "replace_element", target: target.elem, text },
  }[target.zone];
  applyEdit([op]);
}

function wireParts() {
  const svg = $("svg"), tip = $("droptip"), canvas = svg.closest(".canvas");
  let dragged = "", over = null;
  const clear = () => {
    document.querySelectorAll("#svg .drop").forEach((x) => x.classList.remove("drop"));
    tip.hidden = true; over = null;
  };
  document.querySelectorAll(".part").forEach((b) => {
    b.addEventListener("dragstart", (e) => {
      dragged = b.dataset.part;
      e.dataTransfer.setData("text/plain", dragged);
      e.dataTransfer.effectAllowed = "copy";
      document.body.classList.add("dragging");
    });
    b.addEventListener("dragend", () => { dragged = ""; clear(); document.body.classList.remove("dragging"); });
    // A click places the part next to the selection: after a condition,
    // in parallel with an output, or as a new rung when nothing is selected.
    b.addEventListener("click", () => {
      const key = b.dataset.part;
      if (!sel.elem) { placePart(key, { zone: "rung", rung: sel.rung }); return; }
      placePart(key, { zone: PARTS[key].out ? "parallel" : "after", elem: sel.elem });
    });
  });
  svg.addEventListener("dragover", (e) => {
    if (!dragged) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = "copy";
    const g = e.target.closest("g.elem");
    const rung = e.target.closest("g.rung");
    document.querySelectorAll("#svg .drop").forEach((x) => x.classList.remove("drop"));
    if (g) {
      over = { zone: zoneOf(g, e.clientX, e.clientY), elem: g.dataset.elem };
      g.classList.add("drop");
    } else {
      over = { zone: "rung", rung: rung ? rung.dataset.rung : "" };
      if (rung) rung.classList.add("drop");
    }
    const key = { before: "dropBefore", after: "dropAfter", parallel: "dropParallel", replace: "dropReplace", rung: "dropRung" }[over.zone];
    const c = canvas.getBoundingClientRect();
    tip.textContent = t(key);
    tip.style.left = `${e.clientX - c.left + 14}px`;
    tip.style.top = `${e.clientY - c.top + 14}px`;
    tip.hidden = false;
  });
  svg.addEventListener("dragleave", (e) => { if (!svg.contains(e.relatedTarget)) clear(); });
  svg.addEventListener("drop", (e) => {
    e.preventDefault();
    const key = dragged || e.dataTransfer.getData("text/plain");
    const target = over;
    clear();
    document.body.classList.remove("dragging");
    if (key && target) placePart(key, target);
  });
}

// ---- view: zoom ----

let zoom = parseFloat(store.get("zoom")) || 1;
function applyZoom() {
  const svg = $("svg").querySelector("svg");
  $("zoomReset").textContent = Math.round(zoom * 100) + "%";
  if (!svg) return;
  const w = parseFloat(svg.getAttribute("width")), h = parseFloat(svg.getAttribute("height"));
  svg.style.width = `${w * zoom}px`;
  svg.style.height = `${h * zoom}px`;
}
function setZoom(z) {
  zoom = Math.min(3, Math.max(0.4, Math.round(z * 100) / 100));
  store.set("zoom", String(zoom));
  applyZoom();
}
function fitZoom() {
  const svg = $("svg").querySelector("svg");
  if (!svg) return;
  const box = $("svg");
  const avail = box.clientWidth - 32;
  setZoom(avail / parseFloat(svg.getAttribute("width")));
}

// ---- browser: structure tree ----

function treeNode(label, opts = {}) {
  const li = document.createElement("li");
  const row = document.createElement("div");
  row.className = "row";
  row.style.setProperty("--depth", opts.depth || 0);
  if (opts.children) {
    li.classList.add("open");
    row.insertAdjacentHTML("beforeend", '<svg class="chev"><use href="#i-chev"/></svg>');
  }
  if (opts.icon) row.insertAdjacentHTML("beforeend", `<svg><use href="#${opts.icon}"/></svg>`);
  if (opts.id) { const s = document.createElement("span"); s.className = "id"; s.textContent = opts.id; row.append(s); }
  const name = document.createElement("span"); name.textContent = label; row.append(name);
  if (opts.count !== undefined) { const c = document.createElement("span"); c.className = "count"; c.textContent = opts.count; row.append(c); }
  li.append(row);
  if (opts.children) {
    const ul = document.createElement("ul");
    opts.children.forEach((c) => ul.append(c));
    li.append(ul);
    row.onclick = () => li.classList.toggle("open");
  }
  return { li, row };
}

function buildTree(r) {
  const tree = $("tree");
  tree.replaceChildren();
  const rungs = [...document.querySelectorAll("#svg g.rung")].map((g) => {
    const title = g.querySelector("text.rung-title")?.textContent || g.dataset.rung;
    const label = title.replace(new RegExp("^" + g.dataset.rung + "\\s*"), "") || "—";
    const n = treeNode(label, { icon: "i-rung", id: g.dataset.rung, depth: 2 });
    n.row.dataset.rung = g.dataset.rung;
    n.row.onclick = () => {
      select(g.dataset.rung, "");
      g.scrollIntoView({ block: "nearest", behavior: "smooth" });
      const p = positions[g.dataset.rung];
      if (p) jumpTo($("src"), p[0], p[1]);
      $("svg").focus({ preventScroll: true });
    };
    return n.li;
  });
  const syms = [];
  $("src").value.split("\n").forEach((line, i) => {
    const m = line.match(/^\s*sym\s+(\S+)\s*(.*)$/);
    if (!m) return;
    const note = (m[2].match(/"([^"]*)"/) || [, ""])[1];
    const n = treeNode(note || m[2], { icon: "i-tag", id: m[1], depth: 2 });
    n.row.onclick = () => { showPane("text"); jumpTo($("src"), i + 1, 1); };
    syms.push(n.li);
  });
  const rungFolder = treeNode(t("treeRungs"), { icon: "i-folder", depth: 1, count: rungs.length, children: rungs });
  const symFolder = treeNode(t("treeSymbols"), { icon: "i-folder", depth: 1, count: syms.length, children: syms });
  const root = treeNode(`${t("treeProgram")} — ${fileName}`, { icon: "i-doc", depth: 0, children: [rungFolder.li, symFolder.li] });
  tree.append(root.li);
  if (sel.rung) document.querySelector(`#tree .row[data-rung="${sel.rung}"]`)?.classList.add("selected");
}

function showPane(name) {
  document.querySelectorAll(".seg-btn").forEach((b) => b.classList.toggle("active", b.dataset.pane === name));
  document.querySelector(".pane-text").hidden = name !== "text";
  document.querySelector(".pane-tree").hidden = name !== "tree";
  store.set("pane", name);
}

function showTab(name) {
  document.querySelectorAll(".rtab").forEach((b) => {
    b.classList.toggle("active", b.dataset.tab === name);
    b.setAttribute("aria-selected", b.dataset.tab === name);
  });
  document.querySelectorAll(".ribbon-body .group").forEach((g) => { g.hidden = g.dataset.tab !== name; });
}

// ---- scenarios ----

function runScenario() {
  store.set("scn", $("scn").value);
  const r = JSON.parse(rungkit.runScenario($("scn").value, $("trace").checked));
  const box = $("results");
  box.replaceChildren();
  $("summary").replaceChildren();
  if (!r.ok) {
    const p = document.createElement("p"); p.className = "error"; p.textContent = r.error;
    if (r.line) { p.style.cursor = "pointer"; p.onclick = () => jumpTo($("scn"), r.line, 1); }
    box.append(p);
    return;
  }
  const passed = r.results.filter((x) => x.pass).length;
  const sum = document.createElement("div"); sum.className = "sum";
  sum.textContent = t("passed", passed, r.results.length);
  box.append(sum);
  const sum2 = document.createElement("span");
  sum2.className = passed === r.results.length ? "ok" : "ng";
  sum2.textContent = `${passed}/${r.results.length} PASS`;
  $("summary").append(sum2);
  for (const x of r.results) {
    const div = document.createElement("div");
    div.className = "test " + (x.pass ? "pass" : "fail");
    const head = document.createElement("div");
    head.innerHTML = `<b>${x.pass ? "PASS" : "FAIL"}</b> `;
    head.append(`${x.name}  (${t("scans", x.scans)})`);
    div.append(head);
    if (!x.pass) {
      const m = document.createElement("div"); m.className = "msg";
      m.textContent = `${x.line}: ${x.message}`;
      m.onclick = () => jumpTo($("scn"), x.line, 1);
      div.append(m);
    }
    if (x.rows && x.rows.length) div.append(traceTable(x));
    box.append(div);
  }
}

function traceTable(x) {
  const table = document.createElement("table");
  const head = table.insertRow();
  for (const h of ["scan", "ms", ...x.watch]) {
    const th = document.createElement("th"); th.textContent = h; head.append(th);
  }
  let prev = null;
  for (const row of x.rows) {
    const tr = table.insertRow();
    row.forEach((v, i) => {
      const td = tr.insertCell(); td.textContent = v;
      if (i >= 2 && prev && prev[i] !== v) td.className = "changed";
    });
    prev = row;
  }
  if (x.omitted) {
    const cap = table.createCaption(); cap.textContent = t("omitted", x.omitted);
  }
  return table;
}

// ---- wiring ----

async function loadExample(name) {
  const [ldt, test] = EXAMPLES[name];
  const [a, b] = await Promise.all([fetch(ldt).then((r) => r.text()), fetch(test).then((r) => r.text())]);
  fileName = name + ".ldt"; store.set("name", fileName);
  $("src").value = a; $("scn").value = b;
  refresh(); runScenario();
}

function wire() {
  $("src").addEventListener("input", scheduleRefresh);
  $("src").addEventListener("keydown", (e) => {
    if (e.key === "Tab") { e.preventDefault(); document.execCommand("insertText", false, "  "); }
  });
  $("run").onclick = runScenario;
  $("example").onchange = (e) => { if (e.target.value) loadExample(e.target.value); e.target.value = ""; };
  $("fmt").onclick = () => {
    const r = JSON.parse(rungkit.format());
    if (r.ok) { $("src").value = r.text; refresh(); }
  };
  $("save").onclick = () => download(fileName.replace(/\.(json|xml)$/i, ".ldt"), JSON.parse(rungkit.format()).text || $("src").value);
  $("open").onclick = () => $("file").click();
  $("file").onchange = async (e) => {
    const f = e.target.files[0]; if (!f) return;
    const text = await f.text();
    fileName = f.name; store.set("name", fileName);
    if (/\.(json|xml)$/i.test(f.name)) {
      rungkit.update(fileName, text);
      const r = JSON.parse(rungkit.format());
      fileName = f.name.replace(/\.(json|xml)$/i, ".ldt");
      $("src").value = r.ok ? r.text : text;
    } else {
      $("src").value = text;
    }
    refresh(); e.target.value = "";
  };
  const exts = { melsec: "txt", gxw3: "csv", st: "st", plcopen: "xml", json: "json", ldt: "ldt", ascii: "txt", svg: "svg" };
  $("export").onclick = () => {
    const fmt = $("format").value;
    const r = JSON.parse(rungkit.export(fmt));
    if (!r.ok) { toast(t("exportFailed") + r.error); return; }
    const base = fileName.replace(/\.[^.]+$/, "");
    const name = `${base}.${fmt === "ascii" ? "ascii." : ""}${exts[fmt] || "txt"}`;
    if (fmt === "gxw3") { download(name, utf16le(r.text), "text/csv"); return; } // GX Works3 reads UTF-16
    download(name, r.text, fmt === "svg" ? "image/svg+xml" : undefined);
  };
  $("theme").onclick = toggleTheme;
  document.querySelectorAll(".rtab").forEach((b) => { b.onclick = () => showTab(b.dataset.tab); });
  document.querySelectorAll(".seg-btn").forEach((b) => { b.onclick = () => showPane(b.dataset.pane); });
  showPane(store.get("pane") || "text");
  $("runR").onclick = runScenario;
  const syncTrace = () => $("traceR").setAttribute("aria-pressed", String($("trace").checked));
  $("traceR").onclick = () => { $("trace").checked = !$("trace").checked; syncTrace(); };
  $("trace").addEventListener("change", syncTrace);
  $("zoomIn").onclick = () => setZoom(zoom * 1.2);
  $("zoomOut").onclick = () => setZoom(zoom / 1.2);
  $("zoomReset").onclick = () => setZoom(1);
  $("zoomFit").onclick = fitZoom;
  $("svg").addEventListener("wheel", (e) => {
    if (!e.ctrlKey) return;
    e.preventDefault();
    setZoom(zoom * (e.deltaY < 0 ? 1.1 : 1 / 1.1));
  }, { passive: false });
  wireParts();
  $("svg").addEventListener("keydown", (e) => {
    if ((e.key === "Delete" || e.key === "Backspace") && sel.elem) { e.preventDefault(); editAction("delete"); }
    else if (e.key === "Escape") select("", "");
  });
  $("lang").onclick = () => { lang = lang === "ja" ? "en" : "ja"; store.set("lang", lang); applyLanguage(); };
  $("svg").addEventListener("click", (e) => {
    const g = e.target.closest("g.elem");
    const rung = e.target.closest("g.rung")?.dataset.rung || "";
    if (!g) { select(rung, ""); return; }
    select(rung, g.dataset.elem);
    const p = positions[g.dataset.elem];
    if (p) jumpTo($("src"), p[0], p[1]);
    $("svg").focus({ preventScroll: true }); // keep Delete / Esc on the diagram
  });
  document.querySelectorAll(".edit-bar button").forEach((b) => { b.onclick = () => editAction(b.dataset.act); });
  $("src").addEventListener("input", () => select("", ""));
  select("", "");
}

async function start() {
  applyLanguage();
  $("status").textContent = t("loading");
  const go = new Go();
  const ready = new Promise((ok) => addEventListener("rungkit-ready", ok, { once: true }));
  const wasm = await WebAssembly.instantiateStreaming(fetch("rungkit.wasm"), go.importObject);
  go.run(wasm.instance);
  await ready;
  rungkit.setLanguage(lang);
  $("ver").textContent = "v" + rungkit.version;
  const fmt = $("format");
  for (const f of JSON.parse(rungkit.formats())) fmt.append(new Option(f, f));
  fmt.value = "melsec";
  wire();
  const saved = store.get("src");
  if (saved) {
    $("src").value = saved; $("scn").value = store.get("scn") || "";
    refresh();
  } else {
    await loadExample("conveyor");
  }
}

start().catch((e) => { $("status").textContent = String(e); $("status").classList.add("bad"); });
