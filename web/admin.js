const $ = (s) => document.querySelector(s);
let families = [],
  baseUrl = location.origin;
const isSend = document.body.dataset.page === "send";
const labels = {
  added: "Added from phone contacts",
  pending: "Pendiente",
  sending: "SMS en curso",
  sent: "SMS enviado",
  delivered: "SMS entregado",
  manual_sent: "Enviado · manual",
  failed: "Falló",
  uncertain: "Por verificar",
  whatsapp_opened: "WhatsApp abierto",
  yes: "Sí asistirán",
  no: "No asistirán",
};
const esc = (s) =>
  String(s ?? "").replace(
    /[&<>"']/g,
    (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        c
      ],
  );
async function api(path, method = "GET", body) {
  const r = await fetch("/api" + path, {
    method,
    headers: body ? { "Content-Type": "application/json" } : {},
    body: body ? JSON.stringify(body) : undefined,
  });
  if (r.status === 401) {
    $("#pairing").textContent = "";
    $("#pairing").hidden = true;
    $("#login").hidden = false;
    $("#dashboard").hidden = true;
    $("#logout").hidden = true;
  }
  const d = r.status === 204 ? null : await r.json();
  if (!r.ok) throw Error(d?.error || "No se pudo completar la operación");
  return d;
}
function notice(t, error = false) {
  $("#status").textContent = t;
  $("#status").className = error ? "error" : "";
}
const message = (f) =>
  `Hola ${f.name}, te invitamos con tu familia a los 2 años de Abi, el 24 de octubre a las 4 pm. Confirma aquí: ${baseUrl}/?i=${f.id}`;
function badge(value, text) {
  return `<span class="badge ${esc(value)}">${esc(text || labels[value] || value)}</span>`;
}
function render() {
  const q = $("#search").value.toLowerCase(),
    filter = $("#filter").value;
  const shown = families
    .filter((f) => (f.name + " " + f.phone).toLowerCase().includes(q))
    .filter(
      (f) =>
        filter === "all" ||
        (filter === "covered" && f.assignedTo && f.assignedTo !== "admin") ||
        (filter === "unanswered" && f.rsvp === "pending") ||
        (["yes", "no"].includes(filter) && f.rsvp === filter) ||
        (filter === "pending" && f.status === "pending"),
    );
  const attending = families.filter((f) => f.rsvp === "yes");
  const metrics = isSend
    ? [
        [families.length, "Familias"],
        [
          families.filter((f) =>
            ["sent", "delivered", "manual_sent"].includes(f.status),
          ).length,
          "Invitaciones enviadas",
        ],
        [
          families.filter((f) => f.assignedTo && f.assignedTo !== "admin")
            .length,
          "Asignadas a la app",
        ],
        [
          families.filter((f) => ["uncertain", "failed"].includes(f.status))
            .length,
          "Necesitan revisión",
        ],
      ]
    : [
        [attending.length, "Familias confirmadas"],
        [attending.filter(f => f.headcountUnknown).length, "Familias con cantidad pendiente"],
        [
          attending.reduce((n, f) => n + f.adults + f.children, 0),
          "Personas con cantidad registrada",
        ],
        [families.filter((f) => f.rsvp === "no").length, "No asistirán"],
        [families.filter((f) => f.rsvp === "pending").length, "Sin respuesta"],
      ];
  $("#metrics").innerHTML = metrics
    .map(([n, l]) => `<div class="metric"><b>${n}</b><span>${l}</span></div>`)
    .join("");
  $("#head").innerHTML =
    "<tr><th>Familia</th><th>Asistencia</th><th>Envío</th><th>Asignación</th>" +
    (isSend ? "<th>Acciones</th>" : "<th>Último cambio</th>") +
    "</tr>";
  $("#rows").innerHTML =
    shown
      .map(
        (f) =>
          `<tr data-id="${esc(f.id)}"><td><strong>${esc(f.name)}</strong><small>${esc(f.phone)}</small>${!isSend ? `<small>${esc(f.email)}</small>` : ""}</td><td>${badge(f.rsvp, f.rsvp === "pending" ? "Sin respuesta" : null)}${f.rsvp === "yes" ? `<small>${f.headcountUnknown ? "Cantidad de personas pendiente" : `${f.adults} adultos · ${f.children} niños`}</small>` : ""}</td><td>${badge(f.status)}<small>${esc(f.channel || "")}</small>${f.history?.length ? `<details><summary>Historial</summary>${f.history.map((e) => `<p>${esc(labels[e.status] || e.status)} · ${esc(e.at)}<br>${esc(e.actor)} ${esc(e.detail || "")}</p>`).join("")}</details>` : ""}</td><td>${esc(f.assignedName || "Sin asignar")}</td>${isSend ? `<td><div class="row-actions"><button data-action="copy">Copiar mensaje</button><a href="sms:${esc(f.phone)}?body=${encodeURIComponent(message(f))}">Abrir SMS</a><button data-action="whatsapp">WhatsApp</button><button data-action="manual-sms">Marcar SMS enviado</button><button data-action="manual-whatsapp">Marcar WhatsApp enviado</button>${f.assignedTo ? '<button data-action="release">Liberar asignación</button>' : ""}${f.status !== "pending" ? '<button data-action="reset">Preparar reenvío</button>' : ""}${f.status === "sending" ? '<button data-action="uncertain">Marcar por verificar</button>' : ""}</div></td>` : `<td>${f.updatedAt ? esc(new Date(f.updatedAt).toLocaleString("es-MX")) : "—"}</td>`}</tr>`,
      )
      .join("") ||
    '<tr><td colspan="5">No hay familias para este filtro.</td></tr>';
}
async function load() {
  const d = await api("/admin/families");
  families = d.families;
  baseUrl = d.baseUrl;
  $("#login").hidden = true;
  $("#dashboard").hidden = false;
  $("#logout").hidden = false;
  render();
  const users = await api("/admin/users");
  const states = {pin_ready: "PIN ready", signed_in: "Signed in", needs_pin: "Needs a new PIN", revoked: "Access revoked"};
  $("#users").innerHTML = users.map(u =>
    `<div class="device-row"><span><strong>${esc(u.username)}</strong> · ${esc(states[u.status] || u.status)}</span><button data-pin="${esc(u.id)}" class="button outline">Generate new PIN</button>${u.status !== "revoked" ? `<button data-revoke-user="${esc(u.id)}" class="button outline">Revoke access</button>` : ""}</div>`
  ).join("") || '<p class="muted">No sender users yet.</p>';
  const devices = await api("/admin/devices");
  $("#devices").innerHTML =
    devices
      .map(
        (d) =>
          `<div class="device-row"><span>${esc(d.name)} ${d.revoked ? "· Revocado" : ""}</span>${!d.revoked ? `<button data-device="${esc(d.id)}" class="button outline">Revocar</button>` : ""}</div>`,
      )
      .join("") || '<p class="muted">Todavía no hay teléfonos conectados.</p>';
}
$("#login-form").onsubmit = async (e) => {
  e.preventDefault();
  try {
    await api("/login", "POST", { key: $("#key").value });
    $("#key").value = "";
    await load();
    notice("");
  } catch (e) {
    notice(e.message, true);
  }
};
$("#logout").onclick = async () => {
  $("#pairing").textContent = "";
  $("#pairing").hidden = true;
  await api("/logout", "POST");
  location.reload();
};
$("#search").oninput = render;
$("#filter").onchange = render;
$("#refresh").onclick = () => load().catch((e) => notice(e.message, true));
$("#rows").onclick = async (e) => {
  const b = e.target.closest("button[data-action]");
  if (!b) return;
  const f = families.find((f) => f.id === b.closest("tr").dataset.id);
  try {
    if (b.dataset.action === "copy") {
      await navigator.clipboard.writeText(message(f));
      notice("Mensaje copiado. Márcalo como enviado después de enviarlo.");
      return;
    }
    if (b.dataset.action === "whatsapp") {
      window.open(
        `https://wa.me/${f.phone.replace(/\D/g, "")}?text=${encodeURIComponent(message(f))}`,
        "_blank",
        "noopener,noreferrer",
      );
      notice("WhatsApp abierto. Confirma manualmente después de enviar.");
      return;
    }
    let action;
    if (b.dataset.action.startsWith("manual-")) {
      if (!confirm("¿Ya enviaste esta invitación?")) return;
      action = { action: "manual", channel: b.dataset.action.split("-")[1] };
    } else if (b.dataset.action === "release") {
      if (
        !confirm(
          "¿Liberar esta familia? El teléfono deberá volver a asignarla antes de enviar.",
        )
      )
        return;
      action = { action: "release" };
    } else if (b.dataset.action === "uncertain") {
      if (
        !confirm(
          "Verifica que el teléfono dejó de enviar. ¿Marcar este intento para revisión?",
        )
      )
        return;
      action = {
        action: "result",
        attemptId: f.attemptId,
        status: "uncertain",
      };
    } else {
      const detail = prompt(
        "Confirma que no se enviará un duplicado. Motivo del reenvío:",
      );
      if (!detail) return;
      action = { action: "reset", detail };
    }
    b.disabled = true;
    await api(`/families/${f.id}/actions`, "POST", action);
    await load();
    notice("Cambio guardado.");
  } catch (e) {
    notice(e.message, true);
  } finally {
    b.disabled = false;
  }
};
function showPIN(d) {
  $("#pairing").hidden = false;
  $("#pairing").textContent =
    `Username: ${d.user.username}\nSingle-use PIN: ${d.pin}\nExpires: ${new Date(d.user.pinExpires * 1000).toLocaleString()}\nWeb sender: ${d.baseUrl}/sender.html\nServer: ${d.baseUrl}\nShare this PIN privately. It is shown only once.`;
}
$("#device-form").onsubmit = async (e) => {
  e.preventDefault();
  const button = e.currentTarget.querySelector("button");
  button.disabled = true;
  $("#pairing").hidden = true;
  $("#pairing").textContent = "";
  try {
    const d = await api("/admin/users", "POST", {username: $("#device-name").value});
    showPIN(d);
    $("#device-name").value = "";
    await load();
  } catch (e) { notice(e.message, true); }
  finally { button.disabled = false; }
};
$("#users").onclick = async (e) => {
  const b = e.target.closest("[data-pin], [data-revoke-user]");
  if (!b) return;
  const reset = !!b.dataset.pin;
  if (!confirm(reset
    ? "Generate a new PIN? This replaces any previous PIN and signs out the current sender session. Family assignments are kept. Stop any active sending first."
    : "Revoke this user's access? Family assignments and sending history are kept.")) return;
  b.disabled = true;
  $("#pairing").hidden = true;
  $("#pairing").textContent = "";
  try {
    if (reset) showPIN(await api("/admin/users/" + b.dataset.pin + "/pin", "POST", {}));
    else await api("/admin/users/" + b.dataset.revokeUser, "DELETE");
    await load();
  } catch (e) { notice(e.message, true); }
  finally { b.disabled = false; }
};
$("#devices").onclick = async (e) => {
  const b = e.target.closest("[data-device]");
  if (
    !b ||
    !confirm(
      "¿Revocar el acceso de este teléfono? Sus asignaciones seguirán visibles y puedes liberarlas manualmente.",
    )
  )
    return;
  try {
    await api("/admin/devices/" + b.dataset.device, "DELETE");
    await load();
  } catch (e) {
    notice(e.message, true);
  }
};
$("#export").onclick = () => {
  const safe = (v) =>
    '"' +
    String(v ?? "")
      .replace(/^[=+@\-]/, "'$&")
      .replace(/"/g, '""') +
    '"';
  const rows = [
    [
      "Nombre",
      "Teléfono",
      "Email",
      "Asistencia",
      "Adultos",
      "Niños",
      "Cantidad pendiente",
      "Envío",
      "Canal",
      "Asignado",
    ],
    ...families.map((f) => [
      f.name,
      f.phone,
      f.email,
      f.rsvp,
      f.headcountUnknown ? "" : f.adults,
      f.headcountUnknown ? "" : f.children,
      f.headcountUnknown ? "Sí" : "No",
      f.status,
      f.channel,
      f.assignedName,
    ]),
  ];
  const url = URL.createObjectURL(
    new Blob(["\ufeff" + rows.map((r) => r.map(safe).join(",")).join("\r\n")], {
      type: "text/csv;charset=utf-8",
    }),
  );
  const a = document.createElement("a");
  a.href = url;
  a.download = "invitaciones.csv";
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
};
load().catch((e) => {
  if (!$("#dashboard").hidden) notice(e.message, true);
});
