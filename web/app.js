const token = new URLSearchParams(location.search).get("i");
const $ = (selector) => document.querySelector(selector);
const form = $("#rsvp-form"), loading = $("#invitation-loading"), status = $("#rsvp-status"), dialog = $("#rsvp-dialog");
let family = null, busy = false;
async function api(path, body) {
  const r = await fetch(path, {
    method: body ? "POST" : "GET",
    headers: body ? { "Content-Type": "application/json" } : {},
    body: body ? JSON.stringify(body) : undefined,
  });
  let data;
  try { data = await r.json(); } catch { throw Error("No pudimos conectar. Intenta de nuevo."); }
  if (!r.ok) throw Error(data.error || "No pudimos conectar. Intenta de nuevo.");
  return data;
}
function render(editing = false) {
  const answered = family && ["yes", "no"].includes(family.rsvp) && !editing;
  $("#rsvp-actions").hidden = answered;
  $("#rsvp-saved").hidden = !answered;
  if (answered) $("#rsvp-choice").textContent = family.rsvp === "yes"
    ? `Confirmado: ¡sí asistiremos! · ${family.adults + family.children} personas`
    : "Respuesta: no podremos asistir";
}
function setBusy(value) {
  busy = value;
  document.querySelectorAll(".rsvp-dock button, #rsvp-dialog button").forEach(b => b.disabled = value);
}
async function save(rsvp) {
  if (!family || busy) return;
  const data = { rsvp, adults: rsvp === "yes" ? Number(form.elements.adults.value) : 0,
    children: rsvp === "yes" ? Number(form.elements.children.value) : 0 };
  if (![data.adults, data.children].every(n => Number.isSafeInteger(n) && n >= 0) ||
      (rsvp === "yes" && data.adults + data.children === 0)) {
    $("#rsvp-dialog-status").textContent = "Indica al menos una persona y usa cantidades enteras.";
    return;
  }
  setBusy(true);
  status.textContent = "Guardando…";
  $("#rsvp-dialog-status").textContent = "Guardando…";
  try {
    await api(`/api/invitations/${token}/rsvp`, data);
    family = { ...family, ...data };
    render();
    dialog.close();
    status.textContent = "Tu respuesta quedó guardada. ¡Gracias!";
  } catch (err) {
    status.textContent = err.message;
    $("#rsvp-dialog-status").textContent = err.message;
  } finally { setBusy(false); }
}
$("#rsvp-yes").onclick = () => {
  if (!family || busy) return;
  form.elements.adults.value = family.rsvp === "yes" ? family.adults : 1;
  form.elements.children.value = family.rsvp === "yes" ? family.children : 0;
  $("#rsvp-dialog-status").textContent = "";
  dialog.showModal();
};
$("#rsvp-no").onclick = () => save("no");
$("#rsvp-change").onclick = () => { render(true); status.textContent = "Elige tu nueva respuesta."; };
$("#rsvp-cancel").onclick = () => dialog.close();
dialog.addEventListener("cancel", event => { if (busy) event.preventDefault(); });
form.addEventListener("submit", event => { event.preventDefault(); save("yes"); });
if (token && /^[a-f0-9]{32}$/.test(token)) {
  try {
    family = await api(`/api/invitations/${token}`);
    $("#family-name").textContent = family.name;
    loading.hidden = true;
    $("#rsvp-controls").hidden = false;
    render();
  } catch (e) { loading.textContent = e.message; }
} else {
  loading.textContent = "Para responder, abre el enlace personal que te enviamos. Si no lo tienes, pídeselo a los anfitriones.";
}
// Keep the last page content above the fixed controls at every screen size.
new ResizeObserver(([entry]) => {
  document.documentElement.style.setProperty("--rsvp-height", `${entry.target.offsetHeight}px`);
}).observe($(".rsvp-dock"));
document.querySelector("#map-house").onclick = () => {
  document.querySelector("#parking-map").src =
    "https://maps.google.com/maps?q=31.7413977,-106.4310122&z=17&output=embed";
  document.querySelector("#map-caption").textContent =
    "Casa: Calle Arquitectura #419. Puedes estacionarte frente a la propiedad.";
};
document.querySelector("#map-parking").onclick = () => {
  document.querySelector("#parking-map").src =
    "https://maps.google.com/maps?q=Calle%20Ingenier%C3%ADa%2C%20Universidad%2C%20Ciudad%20Ju%C3%A1rez%2C%20Chihuahua&z=17&output=embed";
  document.querySelector("#map-caption").textContent =
    "Más espacio sobre Calle Ingeniería, a un costado del parque. El mapa busca la calle; no indica cajones reservados.";
};
