const preference = matchMedia('(prefers-reduced-motion: reduce)');
let enabled = !preference.matches, frame = 0;
const layers = [...document.querySelectorAll('[data-depth]')];
const toggle = document.querySelector('#motion');
function paint() {
  frame = 0;
  for (const layer of layers) {
    const middle = layer.offsetTop + layer.offsetHeight / 2;
    const distance = scrollY + innerHeight / 2 - middle;
    const offset = enabled && !preference.matches ? Math.max(-70, Math.min(70, distance * Number(layer.dataset.depth))) : 0;
    layer.style.transform = `translate3d(0, ${offset}px, 0)`;
  }
}
function schedule() { if (!frame) frame = requestAnimationFrame(paint); }
function updateControl() {
  toggle.setAttribute('aria-pressed', String(enabled && !preference.matches));
  toggle.textContent = preference.matches ? 'Reduced motion' : enabled ? 'Motion on' : 'Motion off';
  toggle.disabled = preference.matches;
  schedule();
}
toggle.addEventListener('click', () => { enabled = !enabled; updateControl(); });
preference.addEventListener('change', () => { enabled = !preference.matches; updateControl(); });
addEventListener('scroll', schedule, {passive:true});
addEventListener('resize', schedule, {passive:true});
updateControl();
function answer(yes) {
  document.querySelector('#choices').hidden = true;
  document.querySelector('#response').hidden = false;
  document.querySelector('#answer').textContent = yes ? 'Vista previa: sí asistiré.' : 'Vista previa: no podré asistir.';
}
document.querySelector('#yes').onclick = () => answer(true);
document.querySelector('#no').onclick = () => answer(false);
document.querySelector('#reset').onclick = () => {
  document.querySelector('#choices').hidden = false;
  document.querySelector('#response').hidden = true;
};
