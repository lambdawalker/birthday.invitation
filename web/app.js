const token = new URLSearchParams(location.search).get("i");
const form = document.querySelector("#rsvp-form"),
  loading = document.querySelector("#invitation-loading"),
  status = document.querySelector("#rsvp-status");
async function api(path, body) {
  const r = await fetch(path, {
    method: body ? "POST" : "GET",
    headers: body ? { "Content-Type": "application/json" } : {},
    body: body ? JSON.stringify(body) : undefined,
  });
  const data = await r.json();
  if (!r.ok)
    throw Error(data.error || "No pudimos conectar. Intenta de nuevo.");
  return data;
}
function counts() {
  document.querySelector("#counts").hidden = form.elements.rsvp.value !== "yes";
}
form.addEventListener("change", counts);
if (token && /^[a-f0-9]{32}$/.test(token)) {
  try {
    const f = await api(`/api/invitations/${token}`);
    document.querySelector("#greeting").textContent =
      `${f.name}, esta aventura también es tuya.`;
    document.querySelector("#family-name").textContent = f.name;
    loading.hidden = true;
    form.hidden = false;
    if (f.rsvp !== "pending") {
      form.elements.rsvp.value = f.rsvp;
      form.elements.adults.value = f.adults;
      form.elements.children.value = f.children;
      status.textContent =
        f.rsvp === "yes"
          ? "¡Gracias! Los esperamos con mucho cariño."
          : "Gracias por avisarnos. ¡Les mandamos un abrazo!";
    }
    counts();
  } catch (e) {
    loading.textContent = e.message;
  }
} else {
  loading.textContent =
    "Para confirmar, abre el enlace personal que te enviamos. Si no lo tienes, pídeselo a los anfitriones.";
}
form.addEventListener("submit", async (e) => {
  e.preventDefault();
  const button = form.querySelector("button");
  button.disabled = true;
  status.textContent = "Guardando…";
  try {
    const rsvp = form.elements.rsvp.value;
    const data = {
      rsvp,
      adults: rsvp === "yes" ? Number(form.elements.adults.value) : 0,
      children: rsvp === "yes" ? Number(form.elements.children.value) : 0,
    };
    if (
      !Number.isSafeInteger(data.adults) ||
      !Number.isSafeInteger(data.children)
    )
      throw Error("Escribe cantidades enteras.");
    await api(`/api/invitations/${token}/rsvp`, data);
    status.textContent =
      rsvp === "yes"
        ? "¡Confirmado! Los esperamos en la aventura."
        : "Gracias por avisarnos. ¡Les mandamos un abrazo!";
  } catch (err) {
    status.textContent = err.message;
  } finally {
    button.disabled = false;
  }
});
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
