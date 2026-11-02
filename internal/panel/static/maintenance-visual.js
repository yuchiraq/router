import Fluid from './fluid.js';

const surface = document.getElementById('fluid-surface');
const button = document.getElementById('motion-button');
const reduced = matchMedia('(prefers-reduced-motion: reduce)');
let fluid, frame = 0, paused = reduced.matches, running = false, failed = false, lastFeed = 0, lastPointer = null;
const colors = ['#114737', '#431b32', '#44351b', '#193c49'];

function controls() {
    button.setAttribute('aria-pressed', String(paused));
    button.title = button.ariaLabel = paused ? 'Продолжить движение' : 'Остановить движение';
}

function splat(x, y, dx, dy, color) {
    if (!running) return;
    const canvas = surface.querySelector('canvas');
    // The library's public API uses backing-store X but CSS-pixel Y coordinates.
    fluid.setConfig({ colorPalette: [color] });
    fluid.splatAtLocation(x * canvas.width / canvas.clientWidth, y, dx, dy);
}

function seed() {
    const mobile = innerWidth < 700;
    const cx = innerWidth * (mobile ? .54 : .72), cy = innerHeight * (mobile ? .28 : .47);
    const scale = Math.min(innerWidth * .36, innerHeight * .31);
    for (let i = 0; i < 40; i++) {
        const t = i / 39 * Math.PI * 3.6;
        const radius = scale * (.35 + i / 70);
        splat(cx + Math.cos(t) * radius, cy + Math.sin(t) * radius * .65,
            -Math.sin(t) * 420, Math.cos(t) * 420, colors[Math.floor(i / 10)]);
    }
}

function feed(now) {
    if (!running) return;
    if (now - lastFeed > 110) {
        lastFeed = now;
        const t = now / 6000, mobile = innerWidth < 700;
        const x = innerWidth * (mobile ? .55 : .73) + Math.sin(t * 1.3) * innerWidth * .17;
        const y = innerHeight * (mobile ? .28 : .48) + Math.cos(t * .9) * innerHeight * .2;
        splat(x, y, Math.cos(t * 1.3) * 90, Math.sin(t * .9) * 90, colors[Math.floor(now / 5000) % colors.length]);
    }
    frame = requestAnimationFrame(feed);
}

function sync() {
    if (failed) return;
    controls();
    cancelAnimationFrame(frame);
    if (paused || document.hidden) {
        fluid.stop(); running = false;
    } else if (!running) {
        fluid.start(); running = true; seed(); frame = requestAnimationFrame(feed);
    } else frame = requestAnimationFrame(feed);
}

function theme() {
    if (failed) return;
    const light = document.documentElement.dataset.theme === 'light';
    fluid.setConfig({ backgroundColor: light ? '#100d0f' : '#101213', inverted: light });
}

async function fallback() {
    failed = true;
    cancelAnimationFrame(frame); fluid?.stop(); running = false;
    const old = surface.querySelector('canvas'), canvas = document.createElement('canvas');
    canvas.id = 'flow-field'; canvas.setAttribute('aria-hidden', 'true'); old.replaceWith(canvas);
    surface.dataset.renderer = 'canvas';
    await import('./fallback.js');
}

try {
    fluid = new Fluid(surface);
    surface.dataset.renderer = 'webgl';
    fluid.setConfig({ simResolution: 128, dyeResolution: innerWidth < 700 ? 768 : 1024,
        densityDissipation: .8, velocityDissipation: .3, pressureIterations: 18,
        curl: 24, splatRadius: .18, bloom: false, sunrays: false, shading: true,
        hover: false, colorful: false, brightness: .45, transparent: false });
    theme();
    if (paused) {
        // Render a single initial field without continuous motion.
        fluid.start(); running = true; seed();
        requestAnimationFrame(() => requestAnimationFrame(sync));
        controls();
    } else sync();
    button.onclick = () => { paused = !paused; sync(); };
    reduced.addEventListener('change', () => { paused = reduced.matches; sync(); });
    document.addEventListener('visibilitychange', sync);
    new MutationObserver(theme).observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
    addEventListener('pointermove', event => {
        if (!running || event.target.closest('button,a,input,select,textarea')) { lastPointer = null; return; }
        const now = performance.now();
        if (lastPointer && now - lastPointer.time < 16) return;
        const dx = lastPointer ? event.clientX - lastPointer.x : 0;
        const dy = lastPointer ? lastPointer.y - event.clientY : 0;
        splat(event.clientX, event.clientY, Math.max(-1000, Math.min(1000, dx * 24)), Math.max(-1000, Math.min(1000, dy * 24)), colors[Math.floor(now / 1800) % colors.length]);
        lastPointer = { x: event.clientX, y: event.clientY, time: now };
    }, { passive: true });
    addEventListener('pointerdown', event => {
        if (!running || event.target.closest('button,a,input,select,textarea')) return;
        for (let i = 0; i < 12; i++) {
            const angle = i / 12 * Math.PI * 2;
            splat(event.clientX + Math.cos(angle) * 14, event.clientY + Math.sin(angle) * 14,
                Math.cos(angle) * 720, -Math.sin(angle) * 720, colors[i % colors.length]);
        }
    }, { passive: true });
    surface.querySelector('canvas').addEventListener('webglcontextlost', event => { event.preventDefault(); fallback(); }, { once: true });
} catch {
    await fallback();
}
