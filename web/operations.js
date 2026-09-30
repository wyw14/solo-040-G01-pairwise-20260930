const { api, shortID, setMessage, withBusy } = window.LockFlow;
const form = document.querySelector("#plan-form");
const message = document.querySelector("#plan-message");
const stowMessage = document.querySelector("#stow-message");
const vesselRows = document.querySelector("#vessel-rows");
const previewBox = document.querySelector("#stow-preview");

const defaultVessel = { name: "River Star", call_sign: "RS-204", length_m: 86 };

const escapeHTML = (value) => String(value ?? "").replace(/[&<>"]/g, (c) => ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;"}[c]));
const escapeCell = (value) => escapeHTML(value === undefined || value === null ? "" : value);

function renderVesselRows(vessels) {
  vesselRows.innerHTML = vessels.map((vessel, index) => `<tr>
    <td>${index + 1}</td>
    <td><input data-field="name" value="${escapeHTML(vessel.name || "")}"></td>
    <td><input data-field="call_sign" value="${escapeHTML(vessel.call_sign || "")}"></td>
    <td><input data-field="length_m" type="number" min="1" step="0.1" value="${escapeHTML(vessel.length_m || "")}"></td>
    <td>${index > 0 ? '<button type="button" class="danger" data-remove="' + index + '">Remove</button>' : ""}</td>
  </tr>`).join("");
}

function readVessels() {
  return [...vesselRows.querySelectorAll("tr")].map((row) => ({
    name: row.querySelector('[data-field="name"]').value,
    call_sign: row.querySelector('[data-field="call_sign"]').value,
    length_m: Number(row.querySelector('[data-field="length_m"]').value),
  }));
}

function selectedChamber() { return document.querySelector("#chamber").value; }

function renderPreview(report) {
  if (!report) { previewBox.innerHTML = ""; return; }
  const loaded = report.loadings.map((item) => `<tr><td>${item.queue_position}</td><td>${escapeCell(item.name)}</td><td>${escapeCell(item.call_sign)}</td><td>${item.length_m}</td><td>${item.start_m}</td><td>${item.end_m}</td></tr>`).join("");
  const deferred = (report.deferred || []).map((item) => `<tr><td>${item.queue_position}</td><td>${escapeCell(item.name)}</td><td>${escapeCell(item.call_sign)}</td><td>${item.length_m}</td><td>${item.would_end_m || "-"}</td><td>${item.overflow_m || "-"}</td><td>${escapeCell(item.reason)}</td></tr>`).join("");
  previewBox.innerHTML = `<div class="stow-result ${report.exceeded ? "exceeded" : "fitted"}">
    <h3>Stowage preview — ${report.vessel_count} vessel(s) in lockage</h3>
    <div class="muted">Effective length ${report.effective_length_m}m · safety gap ${report.safety_gap_m}m · occupied ${report.occupied_m}m · remaining ${report.remaining_m}m${report.exceeded ? " · <strong>CAPACITY EXCEEDED — no plan will be created</strong>" : ""}</div>
    <table><thead><tr><th>#</th><th>Vessel</th><th>Call sign</th><th>Length</th><th>Start m</th><th>End m</th></tr></thead><tbody>${loaded || '<tr><td colspan="6" class="muted">No vessel fits alone</td></tr>'}</tbody></table>
    ${deferred ? `<h3>Deferred vessels</h3><table><thead><tr><th>#</th><th>Vessel</th><th>Call sign</th><th>Length</th><th>Far end m</th><th>Overflow m</th><th>Reason</th></tr></thead><tbody>${deferred}</tbody></table>` : ""}
    ${report.message ? `<div class="message">${report.message}</div>` : ""}
  </div>`;
}

async function loadStowConfig() {
  const configs = await api("/api/stowage/config");
  const config = configs[document.querySelector("#stow-chamber").value];
  if (config) {
    document.querySelector("#effective-length").value = config.effective_length_m;
    document.querySelector("#safety-gap").value = config.safety_gap_m;
  }
}

async function refresh() {
  const [plans, chambers, health] = await Promise.all([api("/api/plans"), api("/api/chambers"), api("/healthz")]);
  document.querySelector("#health").textContent = health.status === "ok" ? "Devices online" : "Attention required";
  const rows = Object.entries(plans).flatMap(([chamber, items]) => items.map((plan) => `<tr><td>${chamber}</td><td>${shortID(plan.id)}</td><td>${plan.direction}</td><td>${plan.vessels.length}</td><td>${plan.stowage?.occupied_m ?? "-"} / ${plan.stowage?.effective_length_m ?? "-"}</td><td>${new Date(plan.created_at).toLocaleTimeString()}</td></tr>`));
  document.querySelector("#plans").innerHTML = rows.join("") || '<tr><td colspan="6" class="muted">No active plans</td></tr>';
  const current = chambers.find((item) => item.cycle);
  document.querySelector("#metric-chamber").textContent = current?.chamber_id || "Idle";
  document.querySelector("#metric-stage").textContent = current?.cycle?.stage || "-";
  document.querySelector("#metric-cycle").textContent = shortID(current?.cycle?.id);
}

document.querySelector("#stow-chamber").addEventListener("change", () => loadStowConfig().catch((e) => setMessage(stowMessage, e.message)));
document.querySelector("#chamber").addEventListener("change", () => { previewBox.innerHTML = ""; });
document.querySelector("#stow-save").addEventListener("click", async () => {
  await withBusy(document.querySelector("#stow-save"), async () => {
    try {
      await api("/api/stowage/config", { method: "PUT", body: JSON.stringify({
        chamber_id: document.querySelector("#stow-chamber").value,
        effective_length_m: Number(document.querySelector("#effective-length").value),
        safety_gap_m: Number(document.querySelector("#safety-gap").value),
      }) });
      setMessage(stowMessage, "Stowage configuration saved.");
    } catch (error) { setMessage(stowMessage, error.message); }
  });
});

document.querySelector("#add-vessel").addEventListener("click", () => {
  const vessels = readVessels();
  vessels.push({ name: "", call_sign: "", length_m: 60 });
  renderVesselRows(vessels);
});

vesselRows.addEventListener("click", (event) => {
  const remove = event.target.getAttribute("data-remove");
  if (remove === null) return;
  const vessels = readVessels();
  vessels.splice(Number(remove), 1);
  renderVesselRows(vessels);
});

document.querySelector("#preview-stow").addEventListener("click", async () => {
  await withBusy(document.querySelector("#preview-stow"), async () => {
    try {
      const report = await api("/api/stowage/preview", { method: "POST", body: JSON.stringify({
        chamber_id: selectedChamber(), vessels: readVessels(),
      }) });
      renderPreview(report);
      setMessage(message, report.exceeded ? "Capacity exceeded — adjust the vessel list." : "Proposed group fits the chamber.");
    } catch (error) { setMessage(message, error.message); }
  });
});

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  const button = form.querySelector('button[type="submit"]');
  await withBusy(button, async () => {
    try {
      const result = await api("/api/plans", { method: "POST", body: JSON.stringify({
        chamber_id: selectedChamber(),
        direction: document.querySelector("#direction").value,
        vessels: readVessels(),
      }) });
      setMessage(message, `Plan ${shortID(result.plan.id)} admitted to ${result.plan.chamber_id} (${result.plan.stowage.occupied_m}m occupied).`);
      renderPreview(result.plan.stowage);
      await refresh();
    } catch (error) {
      setMessage(message, error.message);
      if (error.stowage) renderPreview(error.stowage);
    }
  });
});

renderVesselRows([defaultVessel]);
loadStowConfig().catch((error) => setMessage(stowMessage, error.message));
refresh().catch((error) => setMessage(message, error.message));
setInterval(() => refresh().catch(() => {}), 5000);
