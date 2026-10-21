// Auto-scroll only until the first interaction; never resume unexpectedly.
const reducedMotion = matchMedia("(prefers-reduced-motion: reduce)");
let stopped = reducedMotion.matches, frame = 0, timer = 0;
let expected = window.scrollY, position = expected, previous = 0;
function stopMotion() {
  stopped = true;
  cancelAnimationFrame(frame);
  clearTimeout(timer);
}
for (const event of ["pointerdown", "touchstart", "wheel", "keydown"]) {
  window.addEventListener(event, stopMotion, { passive: true, capture: true, once: true });
}
window.addEventListener("scroll", () => {
  if (Math.abs(window.scrollY - expected) > 2) stopMotion();
}, { passive: true });
document.addEventListener("visibilitychange", () => { if (document.hidden) stopMotion(); });
reducedMotion.addEventListener("change", () => { if (reducedMotion.matches) stopMotion(); });
function tick(time) {
  if (stopped) return;
  const elapsed = previous ? Math.min(time - previous, 50) : 0;
  previous = time;
  const bottom = document.documentElement.scrollHeight - window.innerHeight;
  position = Math.min(position + elapsed * 0.006, bottom); // 6 pixels/second
  window.scrollTo({ top: position, behavior: "instant" });
  expected = window.scrollY;
  if (position >= bottom) { stopMotion(); return; }
  frame = requestAnimationFrame(tick);
}
if (!stopped) timer = setTimeout(() => {
  position = expected = window.scrollY;
  frame = requestAnimationFrame(tick);
}, 1800);

// The deployment publishes this public contact number; no credentials belong here.
try {
  const response = await fetch("site-config.json", { cache: "no-cache" });
  if (!response.ok) throw Error("Contact configuration unavailable");
  const { contactPhone } = await response.json();
  if (typeof contactPhone === "string" && /^\+[1-9][0-9]{7,14}$/.test(contactPhone)) {
    document.querySelectorAll("[data-contact-link]").forEach(link => {
      link.textContent = contactPhone;
      link.href = `tel:${contactPhone}`;
    });
    document.querySelectorAll("[data-contact]").forEach(element => { element.hidden = false; });
  }
} catch { /* Keep the invitation usable if contact configuration cannot load. */ }
