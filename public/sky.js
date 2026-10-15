// Living-sky WebGL renderer. Draws a procedural sky (gradient, sun/moon glow,
// drifting clouds) on a fullscreen <canvas class="sky">.
// All weather/time reasoning happens on the server; this only reads the
// uniform targets the server pushes into #sky-uniforms and lerps toward them.

import { report } from './report.js';

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

const DEFAULTS = {
	c1: [0.26, 0.54, 0.85],
	c2: [0.5, 0.72, 0.93],
	c3: [0.83, 0.92, 0.98],
	arc: 0.5,
	night: 0,
	cloud: 0.1,
};

function compile(gl, type, src) {
	const sh = gl.createShader(type);
	if (!sh) throw new Error('shader creation failed');
	gl.shaderSource(sh, src);
	gl.compileShader(sh);
	if (!gl.getShaderParameter(sh, gl.COMPILE_STATUS)) {
		throw new Error(gl.getShaderInfoLog(sh) ?? 'shader compile failed');
	}
	return sh;
}

function parse3(s) {
	if (!s) return null;
	const parts = s.split(',').map(Number);
	if (parts.length !== 3 || parts.some(Number.isNaN)) return null;
	return [parts[0], parts[1], parts[2]];
}

export function initSky() {
	let cv = document.querySelector('canvas.sky');
	if (!cv) return;
	// Without a context (no WebGL yet, or it was lost) the canvas is blank and
	// the themed page background behind it shows instead.
	let gl = null;
	let u = null;
	let lostAt = 0;

	// Builds the program and buffers. Runs again after a lost context is
	// restored, since everything on the old context is gone.
	function setup() {
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

		u = {
			res: gl.getUniformLocation(program, 'uRes'),
			time: gl.getUniformLocation(program, 'uTime'),
			c1: gl.getUniformLocation(program, 'uC1'),
			c2: gl.getUniformLocation(program, 'uC2'),
			c3: gl.getUniformLocation(program, 'uC3'),
			arc: gl.getUniformLocation(program, 'uArc'),
			night: gl.getUniformLocation(program, 'uNight'),
			cloud: gl.getUniformLocation(program, 'uCloud'),
		};
		resize();
		report(`sky: ready ${cv.width}x${cv.height}`);
	}

	// The kiosk sleeps its display overnight, and the GPU can drop the context
	// then. preventDefault lets the browser restore it instead of giving up.
	function listen(c) {
		c.addEventListener('webglcontextlost', (e) => {
			e.preventDefault();
			u = null;
			lostAt = performance.now();
			report('sky: context lost');
		});
		c.addEventListener('webglcontextrestored', () => {
			lostAt = 0;
			report('sky: context restored');
			setup();
		});
	}
	listen(cv);

	function acquire() {
		gl = cv.getContext('webgl', { antialias: true });
		if (gl) setup();
		else report('sky: no WebGL context');
	}

	// Chrome doesn't always restore a lost context. A fresh canvas gets a
	// fresh context, so swap one in rather than staying blank.
	function replaceCanvas() {
		const fresh = cv.cloneNode(false);
		listen(fresh);
		cv.replaceWith(fresh);
		cv = fresh;
		gl = null;
		u = null;
		lostAt = 0;
		report('sky: new canvas');
		acquire();
	}

	// getContext can also fail outright, e.g. on the midnight reload while the
	// display is off. Check every minute and when the page is shown again.
	function heal() {
		if (!gl) acquire();
		else if (lostAt && performance.now() - lostAt > 15000) replaceCanvas();
	}
	setInterval(heal, 60000);
	document.addEventListener('visibilitychange', () => {
		if (document.visibilityState === 'visible') heal();
	});

	// Render below display resolution: the sky is all soft gradients, glow and
	// clouds, so the CSS upscale is invisible but the GPU does ~3x less work.
	const RENDER_SCALE = 0.6;
	function resize() {
		cv.width = Math.round(window.innerWidth * RENDER_SCALE);
		cv.height = Math.round(window.innerHeight * RENDER_SCALE);
		if (gl) gl.viewport(0, 0, cv.width, cv.height);
	}
	window.addEventListener('resize', resize);

	const current = {
		...DEFAULTS,
		c1: [...DEFAULTS.c1],
		c2: [...DEFAULTS.c2],
		c3: [...DEFAULTS.c3],
	};
	const target = {
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
	document.addEventListener('fragment', (e) => {
		if (e.detail === 'sky') readTargets();
	});
	readTargets();

	const lerp = (a, b, k) => a + (b - a) * k;
	const FPS = 10;
	const frameInterval = 1000 / FPS;
	let last = 0;
	const start = performance.now();

	function frame(now) {
		requestAnimationFrame(frame);
		// Cap the framerate hard: the only continuous motion is slow cloud drift,
		// which looks fine at 10fps, and each rendered frame forces the three
		// glass panels to re-run their expensive backdrop-filter blur over the
		// canvas. 10fps cuts that reblur load 3x.
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

		if (!u || gl.isContextLost()) return;
		// The page can load while the display sleeps and get a bogus size;
		// the resize event doesn't always follow when it wakes.
		if (cv.width !== Math.round(window.innerWidth * RENDER_SCALE)) resize();
		gl.uniform2f(u.res, cv.width, cv.height);
		gl.uniform1f(u.time, (now - start) / 1000);
		gl.uniform3fv(u.c1, current.c1);
		gl.uniform3fv(u.c2, current.c2);
		gl.uniform3fv(u.c3, current.c3);
		gl.uniform1f(u.arc, current.arc);
		gl.uniform1f(u.night, current.night);
		gl.uniform1f(u.cloud, current.cloud);

		gl.drawArrays(gl.TRIANGLES, 0, 3);
		if (now - lastProbe > PROBE_INTERVAL) {
			lastProbe = now;
			probe();
		}
	}

	// Every so often, log what the canvas actually shows, so a wrong-looking
	// sky can be told apart from a blank one in the server log.
	const PROBE_INTERVAL = 30 * 60 * 1000;
	let lastProbe = -PROBE_INTERVAL;
	function probe() {
		const px = new Uint8Array(4);
		gl.readPixels(
			cv.width >> 1,
			cv.height >> 1,
			1,
			1,
			gl.RGBA,
			gl.UNSIGNED_BYTE,
			px,
		);
		report(
			`sky: ${cv.width}x${cv.height} centre=${px.join(',')} night=${target.night} ${document.visibilityState}`,
		);
	}
	acquire();
	requestAnimationFrame(frame);
}
