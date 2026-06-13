// Living-sky WebGL renderer. Draws a procedural sky (gradient, sun/moon glow,
// drifting clouds) on a fullscreen <canvas class="sky">.
// All weather/time reasoning happens on the server; this only reads the
// uniform targets the server pushes into #sky-uniforms and lerps toward them.

const VERT = `
attribute vec2 aPos;
varying vec2 vUv;
void main() {
	vUv = aPos * 0.5 + 0.5;
	gl_Position = vec4(aPos, 0.0, 1.0);
}`;

const FRAG = `
precision highp float;
varying vec2 vUv;
uniform vec2 uRes;
uniform float uTime;     // elapsed seconds (clouds only; tiny coeff, precision-safe)
uniform vec3 uC1;        // sky top
uniform vec3 uC2;        // sky middle
uniform vec3 uC3;        // horizon
uniform float uArc;      // 0..1 luminary position along its arc
uniform float uNight;    // 0 day .. 1 night
uniform float uCloud;    // 0..1 cloud coverage

float hash2(vec2 p) { return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453123); }

float noise(vec2 p) {
	vec2 i = floor(p);
	vec2 f = fract(p);
	f = f * f * (3.0 - 2.0 * f);
	float a = hash2(i);
	float b = hash2(i + vec2(1.0, 0.0));
	float c = hash2(i + vec2(0.0, 1.0));
	float d = hash2(i + vec2(1.0, 1.0));
	return mix(mix(a, b, f.x), mix(c, d, f.x), f.y);
}

float fbm(vec2 p) {
	float v = 0.0;
	float a = 0.5;
	for (int i = 0; i < 4; i++) {
		v += a * noise(p);
		p *= 2.0;
		a *= 0.5;
	}
	return v;
}

void main() {
	vec2 uv = vUv;
	float aspect = uRes.x / uRes.y;
	vec2 p = vec2(uv.x * aspect, uv.y);

	// Vertical three-stop gradient.
	vec3 col = uv.y > 0.5
		? mix(uC2, uC1, (uv.y - 0.5) * 2.0)
		: mix(uC3, uC2, uv.y * 2.0);

	// Sun / moon glow travelling along an arc.
	vec2 lum = vec2(uArc * aspect, 0.1 + 0.74 * sin(uArc * 3.14159265));
	float d = distance(p, lum);
	float core = smoothstep(0.07, 0.0, d);
	float halo = smoothstep(0.6, 0.0, d);
	vec3 sunCol = mix(vec3(1.0, 0.96, 0.82), vec3(0.86, 0.9, 1.0), uNight);
	col += sunCol * core;
	col += sunCol * halo * halo * mix(0.35, 0.12, uNight);

	// Drifting clouds.
	float cl = fbm(vec2(p.x * 1.6 + uTime * 0.012, uv.y * 2.2 - uTime * 0.004));
	float cov = smoothstep(0.55, 0.95, cl + uCloud * 0.5 - 0.25);
	vec3 cloudCol = mix(vec3(0.97, 0.98, 1.0), vec3(0.3, 0.34, 0.44), uNight);
	col = mix(col, cloudCol, cov * uCloud);

	gl_FragColor = vec4(clamp(col, 0.0, 1.0), 1.0);
}`;

interface SkyUniforms {
	c1: [number, number, number];
	c2: [number, number, number];
	c3: [number, number, number];
	arc: number;
	night: number;
	cloud: number;
}

const DEFAULTS: SkyUniforms = {
	c1: [0.26, 0.54, 0.85],
	c2: [0.5, 0.72, 0.93],
	c3: [0.83, 0.92, 0.98],
	arc: 0.5,
	night: 0,
	cloud: 0.1,
};

function compile(
	gl: WebGLRenderingContext,
	type: number,
	src: string,
): WebGLShader {
	const sh = gl.createShader(type);
	if (!sh) throw new Error('shader creation failed');
	gl.shaderSource(sh, src);
	gl.compileShader(sh);
	if (!gl.getShaderParameter(sh, gl.COMPILE_STATUS)) {
		throw new Error(gl.getShaderInfoLog(sh) ?? 'shader compile failed');
	}
	return sh;
}

function parse3(s: string | undefined): [number, number, number] | null {
	if (!s) return null;
	const parts = s.split(',').map(Number);
	if (parts.length !== 3 || parts.some(Number.isNaN)) return null;
	return [parts[0], parts[1], parts[2]];
}

export function initSky(): void {
	const canvas = document.querySelector<HTMLCanvasElement>('canvas.sky');
	if (!canvas) return;
	const ctx = canvas.getContext('webgl', { antialias: true });
	if (!ctx) return; // No WebGL: panels just sit on the page background.
	// Stable non-null aliases so TS keeps the narrowing inside closures.
	const cv = canvas;
	const gl = ctx;

	const program = gl.createProgram();
	if (!program) throw new Error('program creation failed');
	gl.attachShader(program, compile(gl, gl.VERTEX_SHADER, VERT));
	gl.attachShader(program, compile(gl, gl.FRAGMENT_SHADER, FRAG));
	gl.linkProgram(program);
	if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
		throw new Error(gl.getProgramInfoLog(program) ?? 'program link failed');
	}
	gl.useProgram(program);

	// Fullscreen triangle.
	const buf = gl.createBuffer();
	gl.bindBuffer(gl.ARRAY_BUFFER, buf);
	gl.bufferData(
		gl.ARRAY_BUFFER,
		new Float32Array([-1, -1, 3, -1, -1, 3]),
		gl.STATIC_DRAW,
	);
	const aPos = gl.getAttribLocation(program, 'aPos');
	gl.enableVertexAttribArray(aPos);
	gl.vertexAttribPointer(aPos, 2, gl.FLOAT, false, 0, 0);

	const u = {
		res: gl.getUniformLocation(program, 'uRes'),
		time: gl.getUniformLocation(program, 'uTime'),
		c1: gl.getUniformLocation(program, 'uC1'),
		c2: gl.getUniformLocation(program, 'uC2'),
		c3: gl.getUniformLocation(program, 'uC3'),
		arc: gl.getUniformLocation(program, 'uArc'),
		night: gl.getUniformLocation(program, 'uNight'),
		cloud: gl.getUniformLocation(program, 'uCloud'),
	};

	// Render below display resolution: the sky is all soft gradients, glow and
	// clouds, so the CSS upscale is invisible but the GPU does ~3x less work.
	const RENDER_SCALE = 0.6;
	function resize() {
		cv.width = Math.round(window.innerWidth * RENDER_SCALE);
		cv.height = Math.round(window.innerHeight * RENDER_SCALE);
		gl.viewport(0, 0, cv.width, cv.height);
	}
	resize();
	window.addEventListener('resize', resize);

	const current: SkyUniforms = {
		...DEFAULTS,
		c1: [...DEFAULTS.c1],
		c2: [...DEFAULTS.c2],
		c3: [...DEFAULTS.c3],
	};
	const target: SkyUniforms = {
		...DEFAULTS,
		c1: [...DEFAULTS.c1],
		c2: [...DEFAULTS.c2],
		c3: [...DEFAULTS.c3],
	};
	let primed = false;

	function readTargets() {
		const el = document.getElementById('sky-uniforms');
		if (!el) return;
		const ds = el.dataset;
		const c1 = parse3(ds.c1);
		const c2 = parse3(ds.c2);
		const c3 = parse3(ds.c3);
		if (c1) target.c1 = c1;
		if (c2) target.c2 = c2;
		if (c3) target.c3 = c3;
		if (ds.arc) target.arc = Number(ds.arc);
		if (ds.night) target.night = Number(ds.night);
		if (ds.cloud) target.cloud = Number(ds.cloud);
		// On first read, snap current to target so we don't fade up from defaults.
		if (!primed) {
			primed = true;
			current.c1 = [...target.c1];
			current.c2 = [...target.c2];
			current.c3 = [...target.c3];
			current.arc = target.arc;
			current.night = target.night;
			current.cloud = target.cloud;
		}
	}

	// Re-read whenever the server pushes a new sky frame.
	document.body.addEventListener('htmx:sseMessage', ((evt: CustomEvent) => {
		if (evt.detail?.type === 'sky') requestAnimationFrame(readTargets);
	}) as EventListener);
	readTargets();

	const lerp = (a: number, b: number, k: number) => a + (b - a) * k;
	const FPS = 10;
	const frameInterval = 1000 / FPS;
	let last = 0;
	const start = performance.now();

	function frame(now: number) {
		requestAnimationFrame(frame);
		// Cap the framerate hard: the only continuous motion is slow cloud drift,
		// which looks fine at 10fps, and each rendered frame forces the three
		// glass panels to re-run their (expensive, esp. on Firefox/Safari)
		// backdrop-filter blur over the canvas. 10fps cuts that reblur load 3x.
		if (last && now - last < frameInterval) return;
		const dt = last ? Math.min((now - last) / 1000, 0.1) : 0.016;
		last = now;
		const k = 1 - Math.exp(-dt / 0.6); // ~2s settle

		for (let i = 0; i < 3; i++) {
			current.c1[i] = lerp(current.c1[i], target.c1[i], k);
			current.c2[i] = lerp(current.c2[i], target.c2[i], k);
			current.c3[i] = lerp(current.c3[i], target.c3[i], k);
		}
		current.arc = lerp(current.arc, target.arc, k);
		current.night = lerp(current.night, target.night, k);
		current.cloud = lerp(current.cloud, target.cloud, k);

		gl.uniform2f(u.res, cv.width, cv.height);
		gl.uniform1f(u.time, (now - start) / 1000);
		gl.uniform3fv(u.c1, current.c1);
		gl.uniform3fv(u.c2, current.c2);
		gl.uniform3fv(u.c3, current.c3);
		gl.uniform1f(u.arc, current.arc);
		gl.uniform1f(u.night, current.night);
		gl.uniform1f(u.cloud, current.cloud);

		gl.drawArrays(gl.TRIANGLES, 0, 3);
	}
	requestAnimationFrame(frame);
}
