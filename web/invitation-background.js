// Adapted from the shader supplied by the owner: https://www.shadertoy.com/view/s3XGWs
// Preserve domain-warped FBM with a pastel palette and smooth, grain-free motion.
const vertex = `attribute vec2 position;
void main() { gl_Position = vec4(position, 0.0, 1.0); }`;
const fragment = `precision highp float;
uniform vec2 iResolution;
uniform float iTime;
#define NUM_OCTAVES 5
float hash(vec2 p) {
  vec3 p3 = fract(vec3(p.xyx) * 0.1031);
  p3 += dot(p3, p3.yzx + 33.33);
  return fract((p3.x + p3.y) * p3.z);
}
float noise(vec2 p) {
  vec2 i = floor(p), f = fract(p);
  f = f * f * (3.0 - 2.0 * f);
  return mix(mix(hash(i), hash(i + vec2(1.,0.)), f.x),
    mix(hash(i + vec2(0.,1.)), hash(i + vec2(1.,1.)), f.x), f.y);
}
float fbm(vec2 p) {
  float val = 0.0, amp = 0.5, freq = 1.0;
  for (int i = 0; i < NUM_OCTAVES; i++) {
    val += amp * noise(p * freq); freq *= 2.0; amp *= 0.5;
  }
  return val;
}
float warp(vec2 p) {
  // Independent continuous offsets deform each layer instead of only translating it.
  vec2 flowA = 0.55 * vec2(sin(iTime * 0.71), cos(iTime * 0.53));
  vec2 flowB = 0.45 * vec2(cos(iTime * 0.43), sin(iTime * 0.67));
  vec2 q = vec2(fbm(p + flowA), fbm(p + vec2(5.2,1.3) - flowB));
  vec2 r = vec2(fbm(p + 4.0*q + vec2(1.7,9.2) + flowB),
                fbm(p + 4.0*q + vec2(8.3,2.8) - flowA));
  return fbm(p + (9.0 + sin(iTime * 0.31)) * r);
}
// Powder blue, lavender, blush and warm cream. Keep dark plum text legible.
vec3 palette(float t) {
  vec3 blue = vec3(0.776,0.851,0.914), lavender = vec3(0.824,0.773,0.898);
  vec3 blush = vec3(0.914,0.776,0.839), cream = vec3(0.937,0.863,0.733);
  float x = clamp(t, 0.0, 1.0) * 3.0;
  return x < 1.0 ? mix(blue,lavender,smoothstep(0.,1.,x)) :
    x < 2.0 ? mix(lavender,blush,smoothstep(1.,2.,x)) : mix(blush,cream,smoothstep(2.,3.,x));
}
void main() {
  vec2 uv = gl_FragCoord.xy / iResolution;
  vec2 p = uv * 3.0 - 1.5;
  p.x *= iResolution.x / iResolution.y;
  vec2 pt = p + vec2(iTime * 0.3, iTime * 0.15);
  float n = warp(pt), n2 = fbm(pt * 1.5 + vec2(n * 0.5));
  float density = n * 0.7 + n2 * 0.3;
  float colorIndex = density + 0.2*sin(p.x*0.5+p.y*0.3) + 0.1*n2;
  vec3 col = palette(colorIndex);
  col *= 0.94 + 0.06*smoothstep(0.0,1.0,density);
  gl_FragColor = vec4(clamp(col,0.65,0.96),1.0);
}`;

function startBackground() {
  const hero = document.querySelector('.invitation-intro');
  const canvas = document.querySelector('#invitation-background');
  if (!hero || !canvas) return;
  const gl = canvas.getContext('webgl', { alpha: false, antialias: false, powerPreference: 'low-power' });
  if (!gl) return; // The CSS gradient is the fallback.
  const compile = (type, source) => {
    const shader = gl.createShader(type);
    gl.shaderSource(shader, source); gl.compileShader(shader);
    if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) { gl.deleteShader(shader); throw Error('Background shader unavailable'); }
    return shader;
  };
  const program = gl.createProgram();
  const vs = compile(gl.VERTEX_SHADER, vertex), fs = compile(gl.FRAGMENT_SHADER, fragment);
  gl.attachShader(program, vs); gl.attachShader(program, fs); gl.linkProgram(program);
  gl.deleteShader(vs); gl.deleteShader(fs);
  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) { gl.deleteProgram(program); return; }
  gl.useProgram(program);
  const buffer = gl.createBuffer(); gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
  gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1,-1, 1,-1, -1,1, -1,1, 1,-1, 1,1]), gl.STATIC_DRAW);
  const position = gl.getAttribLocation(program, 'position');
  gl.enableVertexAttribArray(position); gl.vertexAttribPointer(position, 2, gl.FLOAT, false, 0, 0);
  const resolution = gl.getUniformLocation(program, 'iResolution'), time = gl.getUniformLocation(program, 'iTime');
  const reduced = matchMedia('(prefers-reduced-motion: reduce)');
  let frame = 0, last = 0, elapsed = 0, visible = true, lost = false;
  function draw() {
    if (lost) return;
    gl.viewport(0, 0, canvas.width, canvas.height);
    gl.uniform2f(resolution, canvas.width, canvas.height);
    gl.uniform1f(time, elapsed * 0.04); // Slow drifting texture, independent of page scrolling.
    gl.drawArrays(gl.TRIANGLES, 0, 6);
    canvas.classList.add('ready');
  }
  function tick(now) {
    frame = 0;
    if (lost || reduced.matches || document.hidden || !visible) return;
    if (!last || now-last >= 1000/30) { elapsed += last ? Math.min(now-last, 100)/1000 : 0; last = now; draw(); }
    frame = requestAnimationFrame(tick);
  }
  function sync() {
    cancelAnimationFrame(frame); frame = 0; last = 0;
    if (!lost && !document.hidden && visible) { draw(); if (!reduced.matches) frame = requestAnimationFrame(tick); }
  }
  function resize() {
    const { width, height } = hero.getBoundingClientRect();
    // Cap the drawing buffer to 100k pixels to keep mobile GPU cost bounded.
    const scale = Math.min(1, Math.sqrt(100000 / Math.max(1,width*height)));
    canvas.width = Math.max(1,Math.round(width*scale)); canvas.height = Math.max(1,Math.round(height*scale));
    sync();
  }
  new ResizeObserver(resize).observe(hero);
  new IntersectionObserver(([entry]) => { visible = entry.isIntersecting; sync(); }).observe(hero);
  reduced.addEventListener('change', sync);
  document.addEventListener('visibilitychange', sync);
  canvas.addEventListener('webglcontextlost', () => { lost = true; cancelAnimationFrame(frame); canvas.classList.remove('ready'); });
  resize();
}
try { startBackground(); } catch { /* Keep the static CSS background on unsupported devices. */ }
