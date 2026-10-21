// Scroll-scrubbed portrait: no timer, no layout changes, and no animation in reduced motion.
const portrait = document.querySelector('.abi-portrait');
if (portrait) {
  const reduced = matchMedia('(prefers-reduced-motion: reduce)');
  const base = portrait.querySelector('img');
  const canvas = portrait.querySelector('canvas');
  const ctx = canvas.getContext('2d');
  let ready = false, pending = 0;
  const frames = [base];
  function render() {
    pending = 0;
    if (!ready || !ctx || reduced.matches) { canvas.hidden = true; base.style.visibility = ''; return; }
    const rect = portrait.getBoundingClientRect();
    const top = rect.top + window.scrollY;
    // Finish the smile while the portrait is still on screen; reverse on upward scroll.
    const progress = Math.max(0, Math.min(1, window.scrollY / Math.max(1, top + rect.height * 0.35)));
    const position = progress * (frames.length - 1);
    const index = Math.min(frames.length - 2, Math.floor(position));
    const mix = position - index;
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    ctx.globalCompositeOperation = 'source-over';
    ctx.globalAlpha = 1 - mix;
    ctx.drawImage(frames[index], 0, 0, canvas.width, canvas.height);
    // Add weighted premultiplied colors without darkening transparent edges.
    ctx.globalCompositeOperation = 'lighter';
    ctx.globalAlpha = mix;
    ctx.drawImage(frames[index + 1], 0, 0, canvas.width, canvas.height);
    ctx.globalAlpha = 1; ctx.globalCompositeOperation = 'source-over';
    canvas.hidden = false; base.style.visibility = 'hidden';
  }
  function schedule() { if (!pending) pending = requestAnimationFrame(render); }
  async function loadFrames() {
    if (!ctx || ready || reduced.matches) return;
    try {
      const images = await Promise.all(['assets/abi-smile-1.webp', 'assets/abi-smile-2.webp'].map(async src => {
        const img = new Image(); img.src = src; await img.decode(); return img;
      }));
      await base.decode();
      frames.splice(1, frames.length, ...images); ready = true; schedule();
    } catch { /* Keep the original portrait if any frame cannot load. */ }
  }
  window.addEventListener('scroll', schedule, { passive: true });
  window.addEventListener('resize', schedule, { passive: true });
  reduced.addEventListener('change', () => { schedule(); loadFrames(); });
  loadFrames();
}
