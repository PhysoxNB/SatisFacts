// Unit-tests the real OBJ_VS/OBJ_FS shader source from map_webgl.js in a live
// WebGL2 context: verifies it compiles and links, that u_purityVisible survives
// as an active uniform, and that toggling a purity culls exactly the resource
// node instances with that purity (and nothing else).
//
// Prerequisites:
//   npx playwright install chromium
//
// Run:
//   node extraction/viewer/tests/verify_shader.mjs
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { createRequire } from 'node:module';

const __dirname = dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);

// Resolve playwright from node_modules, walking up the tree. Falls back to
// the npx cache if it was installed there.
let chromium;
try {
    chromium = require('playwright').chromium;
} catch {
    const os = require('node:os');
    const npxCache = join(os.homedir(), '.npm', '_npx');
    const { readdirSync } = require('node:fs');
    let found = null;
    for (const dir of readdirSync(npxCache)) {
        const p = join(npxCache, dir, 'node_modules', 'playwright', 'index.js');
        try { require('node:fs').accessSync(p); found = p; break; } catch {}
    }
    if (!found) throw new Error('playwright not found — run: npx playwright install chromium');
    chromium = require(found).chromium;
}

const SRC = readFileSync(join(__dirname, '..', 'map_webgl.js'), 'utf8');

function extract(name) {
    const m = SRC.match(new RegExp('var ' + name + ' = `([\\s\\S]*?)`;'));
    if (!m) throw new Error('could not extract ' + name);
    return m[1];
}
const OBJ_VS = extract('OBJ_VS');
const OBJ_FS = extract('OBJ_FS');

const browser = await chromium.launch({
    args: ['--use-gl=angle', '--use-angle=swiftshader', '--enable-unsafe-swiftshader'],
});
const page = await browser.newPage();
page.on('pageerror', e => console.log('PAGEERROR', e.message));
await page.setContent('<canvas id="c" width="64" height="64"></canvas>');

const result = await page.evaluate(({ vsSrc, fsSrc }) => {
    const out = { log: [] };
    const gl = document.getElementById('c').getContext('webgl2');
    if (!gl) return { fatal: 'no webgl2' };

    // Same patch initWebGL applies before compiling.
    const vs = vsSrc.replace('u_typeVisible[512]', 'u_typeVisible[16]');

    function sh(type, src) {
        const s = gl.createShader(type);
        gl.shaderSource(s, src);
        gl.compileShader(s);
        if (!gl.getShaderParameter(s, gl.COMPILE_STATUS)) {
            out.log.push('COMPILE FAIL: ' + gl.getShaderInfoLog(s));
            return null;
        }
        return s;
    }
    const v = sh(gl.VERTEX_SHADER, vs);
    const f = sh(gl.FRAGMENT_SHADER, fsSrc);
    out.vsCompiled = !!v;
    out.fsCompiled = !!f;
    if (!v || !f) return out;

    const p = gl.createProgram();
    gl.attachShader(p, v);
    gl.attachShader(p, f);
    gl.linkProgram(p);
    out.linked = gl.getProgramParameter(p, gl.LINK_STATUS);
    if (!out.linked) { out.log.push('LINK FAIL: ' + gl.getProgramInfoLog(p)); return out; }
    gl.useProgram(p);

    const U = n => gl.getUniformLocation(p, n);
    out.hasPurityUniform = U('u_purityVisible[0]') !== null || U('u_purityVisible') !== null;
    out.hasDprUniform = U('u_dpr') !== null; // should be gone

    // Three resource-node instances (cat 9) with purity 0/1/2 in the yaw slot,
    // spread horizontally so each lands in its own screen third.
    const stride = 11;
    const inst = new Float32Array(3 * stride);
    const purityOf = [0, 1, 2];
    for (let i = 0; i < 3; i++) {
        const o = i * stride;
        inst[o] = (i - 1) * 40;       // world x: -40, 0, 40
        inst[o + 1] = 0;              // world y
        inst[o + 2] = 0;              // z
        inst[o + 3] = 9;              // cat = resource node
        inst[o + 4] = 0;              // typeIdx
        inst[o + 5] = purityOf[i];    // yaw slot carries purity
        inst[o + 6] = 8;              // halfW
        inst[o + 7] = 8;              // halfD
        inst[o + 8] = 1;              // color r
        inst[o + 9] = 1;              // g
        inst[o + 10] = 1;             // b
    }

    const quad = new Float32Array([-1, -1, 1, -1, 1, 1, -1, -1, 1, 1, -1, 1]);
    const qb = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, qb);
    gl.bufferData(gl.ARRAY_BUFFER, quad, gl.STATIC_DRAW);
    const aQuad = gl.getAttribLocation(p, 'a_quadPos');
    gl.enableVertexAttribArray(aQuad);
    gl.vertexAttribPointer(aQuad, 2, gl.FLOAT, false, 0, 0);
    gl.vertexAttribDivisor(aQuad, 0);

    const ib = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, ib);
    gl.bufferData(gl.ARRAY_BUFFER, inst, gl.STATIC_DRAW);
    const FS = 4, S = stride * FS;
    const bind = (name, size, off) => {
        const l = gl.getAttribLocation(p, name);
        if (l < 0) { out.log.push('missing attrib ' + name); return; }
        gl.enableVertexAttribArray(l);
        gl.vertexAttribPointer(l, size, gl.FLOAT, false, S, off * FS);
        gl.vertexAttribDivisor(l, 1);
    };
    bind('a_instPos', 2, 0);
    bind('a_instZ', 1, 2);
    bind('a_instCat', 1, 3);
    bind('a_instTypeIdx', 1, 4);
    bind('a_instYaw', 1, 5);
    bind('a_instHalfDim', 2, 6);
    bind('a_instColor', 3, 8);

    gl.uniform2f(U('u_camCenter'), 0, 0);
    gl.uniform1f(U('u_zoom'), 0.5);
    gl.uniform2f(U('u_viewport'), 64, 64);
    gl.uniform1iv(U('u_catVisible'), new Int32Array(22).fill(1));
    gl.uniform1iv(U('u_typeVisible'), new Int32Array(16).fill(1));
    gl.uniform1i(U('u_zEnabled'), 0);
    gl.uniform1f(U('u_zMin'), -1e9);
    gl.uniform1f(U('u_zMax'), 1e9);
    gl.uniform1i(U('u_radiusActive'), 0);
    gl.uniform2f(U('u_radiusCenter'), 0, 0);
    gl.uniform1f(U('u_radiusMeters'), 1e9);
    gl.uniform3fv(U('u_catColors'), new Float32Array(22 * 3).fill(1));
    gl.uniform1f(U('u_foundationAlpha'), 1);

    // Which screen thirds have anything drawn, for a given purity mask.
    function drawnThirds(mask) {
        gl.uniform1iv(U('u_purityVisible'), new Int32Array(mask));
        gl.viewport(0, 0, 64, 64);
        gl.clearColor(0, 0, 0, 1);
        gl.clear(gl.COLOR_BUFFER_BIT);
        gl.drawArraysInstanced(gl.TRIANGLES, 0, 6, 3);
        const px = new Uint8Array(64 * 64 * 4);
        gl.readPixels(0, 0, 64, 64, gl.RGBA, gl.UNSIGNED_BYTE, px);
        const hit = [0, 0, 0];
        for (let y = 0; y < 64; y++) {
            for (let x = 0; x < 64; x++) {
                const i = (y * 64 + x) * 4;
                if (px[i] > 20 || px[i + 1] > 20 || px[i + 2] > 20) {
                    hit[Math.min(2, Math.floor(x / (64 / 3)))]++;
                }
            }
        }
        return hit.map(n => n > 0);
    }

    out.all      = drawnThirds([1, 1, 1]);
    out.none     = drawnThirds([0, 0, 0]);
    out.onlyPure = drawnThirds([0, 0, 1]);
    out.noImpure = drawnThirds([0, 1, 1]);
    out.onlyImp  = drawnThirds([1, 0, 0]);
    return out;
}, { vsSrc: OBJ_VS, fsSrc: OBJ_FS });

await browser.close();

const j = JSON.stringify;
console.log('vs compiled      :', result.vsCompiled);
console.log('fs compiled      :', result.fsCompiled);
console.log('program linked   :', result.linked);
console.log('u_purityVisible  :', result.hasPurityUniform, '(active uniform present)');
console.log('u_dpr removed    :', result.hasDprUniform === false);
if (result.log && result.log.length) console.log('log:', j(result.log));
console.log('');
console.log('thirds drawn = [impure, normal, pure]');
console.log('  purities [1,1,1] ->', j(result.all));
console.log('  purities [0,0,0] ->', j(result.none));
console.log('  purities [0,0,1] ->', j(result.onlyPure));
console.log('  purities [0,1,1] ->', j(result.noImpure));
console.log('  purities [1,0,0] ->', j(result.onlyImp));

const eq = (a, b) => j(a) === j(b);
const checks = {
    'vs compiles': result.vsCompiled === true,
    'fs compiles': result.fsCompiled === true,
    'links': result.linked === true,
    'u_purityVisible is active': result.hasPurityUniform === true,
    'u_dpr is gone': result.hasDprUniform === false,
    'all purities visible -> all 3 quads': eq(result.all, [true, true, true]),
    'no purities visible -> 0 quads': eq(result.none, [false, false, false]),
    'only pure -> just the pure quad': eq(result.onlyPure, [false, false, true]),
    'impure hidden -> normal+pure only': eq(result.noImpure, [false, true, true]),
    'only impure -> just the impure quad': eq(result.onlyImp, [true, false, false]),
};
console.log('');
let ok = true;
for (const [k, v] of Object.entries(checks)) {
    console.log((v ? 'PASS  ' : 'FAIL  ') + k);
    if (!v) ok = false;
}
console.log('\n' + (ok ? 'ALL SHADER CHECKS PASSED' : 'SHADER CHECKS FAILED'));
process.exit(ok ? 0 : 1);
