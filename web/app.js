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
    program: "プログラム（.ldt）", ladder: "ラダー図", clickHint: "要素をクリックすると、その行に移動します",
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
    program: "Program (.ldt)", ladder: "Ladder", clickHint: "Click an element to jump to its line",
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
  $("lang").textContent = lang === "ja" ? "EN" : "日本語";
  if (window.rungkit) { rungkit.setLanguage(lang); refresh(); }
}

const EXAMPLES = {
  conveyor: ["examples/conveyor.ldt", "examples/conveyor.ldtest"],
  "melsec-semantics": ["examples/melsec-semantics.ldt", "examples/melsec-semantics.ldtest"],
  iec: ["examples/iec.ldt", "examples/iec.ldtest"],
};

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
  list.replaceChildren();
  if (!r.ok) {
    status.textContent = r.error; status.classList.add("bad");
    svg.innerHTML = `<div class="placeholder">${t("noDrawing")}</div>`;
    const li = document.createElement("li");
    li.className = "error";
    li.innerHTML = `<code>SYNTAX</code>`;
    li.append(r.error);
    if (r.line) li.onclick = () => jumpTo($("src"), r.line, r.col);
    list.append(li);
    positions = {};
    return;
  }
  positions = r.positions || {};
  const errs = r.diagnostics.filter((d) => d.severity === "error").length;
  status.textContent = `${r.profile} · ${t("rungs", r.rungs)} · ${t("errors", errs, r.diagnostics.length - errs)}`;
  status.classList.toggle("bad", errs > 0);
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
}

let timer = 0;
function scheduleRefresh() { clearTimeout(timer); timer = setTimeout(refresh, 200); }

// ---- scenarios ----

function runScenario() {
  store.set("scn", $("scn").value);
  const r = JSON.parse(rungkit.runScenario($("scn").value, $("trace").checked));
  const box = $("results");
  box.replaceChildren();
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
    if (!r.ok) { alert(t("exportFailed") + r.error); return; }
    const base = fileName.replace(/\.[^.]+$/, "");
    const name = `${base}.${fmt === "ascii" ? "ascii." : ""}${exts[fmt] || "txt"}`;
    if (fmt === "gxw3") { download(name, utf16le(r.text), "text/csv"); return; } // GX Works3 reads UTF-16
    download(name, r.text, fmt === "svg" ? "image/svg+xml" : undefined);
  };
  $("lang").onclick = () => { lang = lang === "ja" ? "en" : "ja"; store.set("lang", lang); applyLanguage(); };
  $("svg").addEventListener("click", (e) => {
    const g = e.target.closest("g.elem"); if (!g) return;
    document.querySelectorAll("#svg g.elem.selected").forEach((x) => x.classList.remove("selected"));
    g.classList.add("selected");
    const p = positions[g.dataset.elem];
    if (p) jumpTo($("src"), p[0], p[1]);
  });
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
