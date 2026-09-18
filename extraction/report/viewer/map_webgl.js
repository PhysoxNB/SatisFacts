    // ============================================================
    // WebGL2 Renderer (instanced quads for objects + thick lines)
    // ============================================================

    // Line width per category in world meters (at zoom=1)
    var CAT_LINE_WIDTH = {};
    CAT_LINE_WIDTH[CAT.BELT] = 1.5;
    CAT_LINE_WIDTH[CAT.PIPE] = 1.5;
    CAT_LINE_WIDTH[CAT.RAIL] = 5.0;
    CAT_LINE_WIDTH[CAT.HYPERTUBE] = 1.5;
    CAT_LINE_WIDTH[CAT.POWER_POLE] = 0.5;

    // Structure rectangle thickness scale (category 10) — only the shorter dimension
    // (thickness) is shrunk; the longer dimension (length) stays full so walls stay connected.
    var STRUCTURE_THICKNESS_SCALE = 1.0; // full thickness — walls connect precisely

    // Render-scaled half-width. For structures, only the shorter dimension is shrunk.
    function getRenderHw(state, i) {
        var hw = getEffHw(state, i);
        if (state.cats[i] === CAT.STRUCTURE && hw > 0) {
            var hd = getEffHd(state, i);
            if (hd > 0 && hw >= hd) return hw; // this is the length — keep full
            return hw * STRUCTURE_THICKNESS_SCALE; // this is the thickness — shrink
        }
        return hw;
    }

    // Render-scaled half-depth. For structures, only the shorter dimension is shrunk.
    function getRenderHd(state, i) {
        var hd = getEffHd(state, i);
        if (state.cats[i] === CAT.STRUCTURE && hd > 0) {
            var hw = getEffHw(state, i);
            if (hw > 0 && hd >= hw) return hd; // this is the length — keep full
            return hd * STRUCTURE_THICKNESS_SCALE; // this is the thickness — shrink
        }
        return hd;
    }

    // Line segment extension per category (fraction of width)
    var CAT_LINE_EXTEND = {};
    CAT_LINE_EXTEND[CAT.BELT] = 0.20;
    CAT_LINE_EXTEND[CAT.PIPE] = 0.20;
    CAT_LINE_EXTEND[CAT.RAIL] = 0.15;
    CAT_LINE_EXTEND[CAT.HYPERTUBE] = 0.2;
    CAT_LINE_EXTEND[CAT.POWER_POLE] = 0.15;

    // Category colors as float arrays [r, g, b]
    var CAT_COLORS_GL = {};
    CAT_COLORS_GL[CAT.BUILDING] = [0.29, 0.62, 1.0];
    CAT_COLORS_GL[CAT.MINER] = [1.0, 0.67, 0.27];
    CAT_COLORS_GL[CAT.STATION] = [1.0, 0.4, 0.67];
    CAT_COLORS_GL[CAT.POWER_POLE] = [0.87, 0.67, 0.27];
    CAT_COLORS_GL[CAT.BELT] = [0.53, 0.87, 0.53];
    CAT_COLORS_GL[CAT.PIPE] = [1.0, 0.53, 0.0];
    CAT_COLORS_GL[CAT.RAIL] = [0.67, 0.53, 1.0];
    CAT_COLORS_GL[CAT.HYPERTUBE] = [1.0, 0.53, 0.4];
    CAT_COLORS_GL[CAT.RESOURCE_NODE] = [0.53, 1.0, 0.53];
    CAT_COLORS_GL[CAT.STRUCTURE] = [0.53, 0.53, 0.53];
    CAT_COLORS_GL[CAT.FOUNDATION] = [0.8, 0.8, 0.8];
    CAT_COLORS_GL[CAT.BLUE_SLUG] = [0.29, 0.62, 1.0];
    CAT_COLORS_GL[CAT.YELLOW_SLUG] = [0.96, 0.65, 0.14];
    CAT_COLORS_GL[CAT.PURPLE_SLUG] = [0.61, 0.35, 0.71];
    CAT_COLORS_GL[CAT.SOMERSLOOP] = [0.91, 0.12, 0.39];
    CAT_COLORS_GL[CAT.MERCER_SPHERE] = [0.66, 0.33, 0.97];
    CAT_COLORS_GL[CAT.CRASH_SITE] = [0.9, 0.5, 0.13];
    CAT_COLORS_GL[CAT.POWER_GEN] = [1.0, 0.4, 0.0];
    CAT_COLORS_GL[CAT.POWER_STORAGE] = [1.0, 0.87, 0.27];
    CAT_COLORS_GL[CAT.SIGNAL] = [1.0, 0.27, 0.27];
    CAT_COLORS_GL[CAT.SPECIAL] = [0.98, 0.75, 0.14];

    // Resource type colors as float [r, g, b] for WebGL (matches RESOURCE_TYPE_COLORS hex)
    var RESOURCE_TYPE_COLORS_GL = {
        '__resource_iron_ore__':      [0.69, 0.69, 0.69],
        '__resource_stone__':         [0.78, 0.72, 0.63],
        '__resource_coal__':          [0.25, 0.25, 0.25],
        '__resource_water__':         [0.27, 0.53, 1.0],
        '__resource_copper_ore__':    [1.0, 0.53, 0.27],
        '__resource_crude_oil__':     [0.33, 0.40, 0.13],
        '__resource_nitrogen_gas__':  [0.27, 0.87, 0.67],
        '__resource_geyser__':        [1.0, 0.27, 0.27],
        '__resource_sam__':           [0.67, 0.27, 1.0],
        '__resource_caterium_ore__':  [1.0, 0.87, 0.27],
        '__resource_quartz__':        [0.87, 0.67, 1.0],
        '__resource_bauxite__':       [0.80, 0.40, 0.27],
        '__resource_sulfur__':        [0.87, 1.0, 0.27],
        '__resource_uranium_ore__':   [0.27, 1.0, 0.27],
        '__resource_unknown__':       [0.53, 0.53, 0.53]
    };

    function resourceTypeColorGL(typePath) {
        return RESOURCE_TYPE_COLORS_GL[typePath] || [0.53, 0.53, 0.53];
    }

    var OBJ_VS = `#version 300 es
    precision highp float;
    in vec2 a_quadPos; // -1..1 quad coords
    in vec2 a_instPos; // world x,y meters
    in float a_instZ;  // world z meters
    in float a_instCat; // category
    in float a_instTypeIdx; // type index
    in float a_instYaw; // yaw in degrees*10 (resource nodes: purity 0/1/2)
    in vec2 a_instHalfDim; // halfWidth, halfDepth meters
    in vec3 a_instColor; // per-instance color (resource nodes only)

    uniform vec2 u_camCenter;
    uniform float u_zoom;
    uniform vec2 u_viewport; // pixels
    uniform int u_catVisible[22];
    uniform int u_typeVisible[512];
    uniform int u_purityVisible[3];
    uniform int u_zEnabled;
    uniform float u_zMin, u_zMax;
    uniform int u_radiusActive;
    uniform vec2 u_radiusCenter;
    uniform float u_radiusMeters;

    flat out int v_cat;
    flat out vec3 v_instColor;
    out vec2 v_uv;

    void main() {
        int cat = int(a_instCat + 0.5);
        if (u_catVisible[cat] == 0) { gl_Position = vec4(0.0, 0.0, -2.0, 1.0); return; }
        int typeIdx = int(a_instTypeIdx + 0.5);
        if (u_typeVisible[typeIdx] == 0) { gl_Position = vec4(0.0, 0.0, -2.0, 1.0); return; }
        // Resource nodes encode purity in the yaw slot instead of a rotation.
        // Without this the quad stayed drawn while the overlay arrow was hidden.
        if (cat == 9) {
            int pur = int(a_instYaw + 0.5);
            if (pur < 0 || pur > 2) pur = 0;
            if (u_purityVisible[pur] == 0) { gl_Position = vec4(0.0, 0.0, -2.0, 1.0); return; }
        }
        if (u_zEnabled != 0 && (a_instZ < u_zMin || a_instZ > u_zMax)) { gl_Position = vec4(0.0, 0.0, -2.0, 1.0); return; }
        if (u_radiusActive != 0) {
            vec2 d = a_instPos - u_radiusCenter;
            if (dot(d, d) > u_radiusMeters * u_radiusMeters) { gl_Position = vec4(0.0, 0.0, -2.0, 1.0); return; }
        }

        float yaw = a_instYaw * 0.1 * 3.14159265 / 180.0;
        float c = cos(yaw), s = sin(yaw);
        vec2 local = a_quadPos * a_instHalfDim;
        vec2 rotated = vec2(local.x * c - local.y * s, local.x * s + local.y * c);
        vec2 worldPos = a_instPos + rotated;
        vec2 screenPx = (worldPos - u_camCenter) * u_zoom + u_viewport * 0.5;
        vec2 ndc = screenPx / u_viewport * 2.0 - 1.0;
        gl_Position = vec4(ndc.x, -ndc.y, 0.0, 1.0);
        v_cat = cat;
        v_instColor = a_instColor;
        v_uv = a_quadPos * 0.5 + 0.5;
    }`;

    var OBJ_FS = `#version 300 es
    precision highp float;
    flat in int v_cat;
    flat in vec3 v_instColor;
    in vec2 v_uv;
    uniform vec3 u_catColors[22];
    uniform float u_foundationAlpha;
    out vec4 fragColor;
    void main() {
        vec3 color = (v_cat == 9) ? v_instColor : u_catColors[v_cat];
        // Darker outline for buildings, miners, stations, power poles, foundations
        float edge = (v_cat == 0) ? 0.04 : 0.02;
        if (v_cat == 0 || v_cat == 1 || v_cat == 2 || v_cat == 3 || v_cat == 11) {
            if (v_uv.x < edge || v_uv.x > 1.0 - edge || v_uv.y < edge || v_uv.y > 1.0 - edge) {
                color *= 0.5;
            }
        }
        if (v_cat == 10) { fragColor = vec4(color, 0.7); return; }
        if (v_cat == 11) { fragColor = vec4(color, u_foundationAlpha); return; }
        fragColor = vec4(color, 0.85);
    }`;

    var LINE_VS = `#version 300 es
    precision highp float;
    in vec2 a_quadPos; // x: -0.5..0.5 along segment, y: -0.5..0.5 perpendicular
    in vec2 a_instStart; // world x,y
    in vec2 a_instEnd;
    in float a_instCat;
    in float a_instTypeIdx;
    in float a_instZ; // average Z (meters)

    uniform vec2 u_camCenter;
    uniform float u_zoom;
    uniform vec2 u_viewport;
    uniform float u_catWidth[22];
    uniform float u_catExtend[22];
    uniform int u_catVisible[22];
    uniform int u_typeVisible[512];
    uniform int u_zEnabled;
    uniform float u_zMin, u_zMax;
    uniform int u_radiusActive;
    uniform vec2 u_radiusCenter;
    uniform float u_radiusMeters;

    flat out int v_cat;

    void main() {
        int cat = int(a_instCat + 0.5);
        if (u_catVisible[cat] == 0) { gl_Position = vec4(0.0, 0.0, -2.0, 1.0); return; }
        int typeIdx = int(a_instTypeIdx + 0.5);
        if (u_typeVisible[typeIdx] == 0) { gl_Position = vec4(0.0, 0.0, -2.0, 1.0); return; }
        if (u_zEnabled != 0 && (a_instZ < u_zMin || a_instZ > u_zMax)) { gl_Position = vec4(0.0, 0.0, -2.0, 1.0); return; }

        vec2 dir = a_instEnd - a_instStart;
        float len = length(dir);
        vec2 ndir = len > 0.001 ? dir / len : vec2(1.0, 0.0);
        vec2 perp = vec2(-ndir.y, ndir.x);

        float width = u_catWidth[cat];
        // Scale width with zoom but clamp to reasonable pixel range
        float widthPx = width * u_zoom;
        if (widthPx < 1.0) widthPx = 1.0;
        float widthWorld = widthPx / u_zoom;

        // Extend segment by per-category fraction of width to fill gaps at joints
        float extend = widthWorld * u_catExtend[cat];
        // a_quadPos.x: -0.5..0.5 -> map to -extend..len+extend
        float t = a_quadPos.x + 0.5; // 0..1
        float along = t * (len + 2.0 * extend) - extend;
        vec2 worldPos = a_instStart
            + ndir * along
            + perp * (widthWorld * a_quadPos.y);

        if (u_radiusActive != 0) {
            vec2 mid = (a_instStart + a_instEnd) * 0.5;
            vec2 d = mid - u_radiusCenter;
            if (dot(d, d) > u_radiusMeters * u_radiusMeters) { gl_Position = vec4(0.0, 0.0, -2.0, 1.0); return; }
        }

        vec2 screenPx = (worldPos - u_camCenter) * u_zoom + u_viewport * 0.5;
        vec2 ndc = screenPx / u_viewport * 2.0 - 1.0;
        gl_Position = vec4(ndc.x, -ndc.y, 0.0, 1.0);
        v_cat = cat;
    }`;

    var LINE_FS = `#version 300 es
    precision highp float;
    flat in int v_cat;
    uniform vec3 u_catColors[22];
    out vec4 fragColor;
    void main() {
        vec3 color = u_catColors[v_cat];
        fragColor = vec4(color, 1.0);
    }`;

    var MAP_VS = `#version 300 es
    precision highp float;
    in vec2 a_pos; // -1..1 fullscreen quad
    out vec2 v_texCoord;
    uniform vec2 u_mapWorldTL;
    uniform vec2 u_mapWorldBR;
    uniform vec2 u_camCenter;
    uniform float u_zoom;
    uniform vec2 u_viewport;
    void main() {
        // Map image corners in world -> screen -> NDC
        vec2 worldPos = mix(u_mapWorldTL, u_mapWorldBR, a_pos * 0.5 + 0.5);
        // Y flip: map image +Y is down, world +Y is south (also down in screen)
        vec2 screenPx = (worldPos - u_camCenter) * u_zoom + u_viewport * 0.5;
        vec2 ndc = screenPx / u_viewport * 2.0 - 1.0;
        gl_Position = vec4(ndc.x, -ndc.y, 0.0, 1.0);
        v_texCoord = a_pos * 0.5 + 0.5;
    }`;

    var MAP_FS = `#version 300 es
    precision highp float;
    in vec2 v_texCoord;
    out vec4 fragColor;
    uniform sampler2D u_mapTex;
    void main() {
        fragColor = texture(u_mapTex, v_texCoord) * 0.92;
    }`;

    function compileShader(gl, type, source) {
        var shader = gl.createShader(type);
        gl.shaderSource(shader, source);
        gl.compileShader(shader);
        if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
            console.error('Shader compile error:', gl.getShaderInfoLog(shader));
            console.error(source);
            gl.deleteShader(shader);
            return null;
        }
        return shader;
    }

    function createProgram(gl, vsSource, fsSource) {
        var vs = compileShader(gl, gl.VERTEX_SHADER, vsSource);
        var fs = compileShader(gl, gl.FRAGMENT_SHADER, fsSource);
        if (!vs || !fs) return null;
        var prog = gl.createProgram();
        gl.attachShader(prog, vs);
        gl.attachShader(prog, fs);
        gl.linkProgram(prog);
        if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) {
            console.error('Program link error:', gl.getProgramInfoLog(prog));
            gl.deleteProgram(prog);
            return null;
        }
        return prog;
    }

    function initWebGL(state) {
        var gl = state.gl;
        if (!gl) return;

        try {
            _initWebGLImpl(state, gl);
        } catch (e) {
            console.error('WebGL2 init failed:', e);
        }
    }

    function _initWebGLImpl(state, gl) {
        // The u_typeVisible uniform array must be exactly as long as state.typeVisible,
        // which is sized from the save's actual type count. Patch the declared length
        // into the shader source before compiling.
        var nTypes = state.typeVisible ? state.typeVisible.length : 512;
        var withTypeLen = function(src) {
            return src.replace('u_typeVisible[512]', 'u_typeVisible[' + nTypes + ']');
        };

        // Create programs
        state.glObjProgram = createProgram(gl, withTypeLen(OBJ_VS), OBJ_FS);
        state.glLineProgram = createProgram(gl, withTypeLen(LINE_VS), LINE_FS);
        state.glMapProgram = createProgram(gl, MAP_VS, MAP_FS);
        if (!state.glObjProgram || !state.glLineProgram || !state.glMapProgram) {
            throw new Error('Failed to create shader programs');
        }

        // --- Object VBOs ---
        // Quad: 6 vertices (2 triangles), positions -1..1
        var quadVerts = new Float32Array([
            -1, -1,  1, -1,  1, 1,
            -1, -1,  1, 1,  -1, 1
        ]);
        state.glQuadVBO = gl.createBuffer();
        gl.bindBuffer(gl.ARRAY_BUFFER, state.glQuadVBO);
        gl.bufferData(gl.ARRAY_BUFFER, quadVerts, gl.STATIC_DRAW);

        // Instance data: per object [posX, posY, posZ, cat, typeIdx, yaw, halfW, halfD]
        // Stable partition: foundations (cat=11) first so they render as a
        // background layer, everything else after, original order kept within
        // each group. Written straight into the Int32Array with two cursors —
        // the intermediate plain arrays boxed every index.
        var n = state.count;
        var objOrder = new Int32Array(n);
        var nFoundations = 0;
        for (let i2 = 0; i2 < n; i2++) {
            if (state.cats[i2] === 11) nFoundations++;
        }
        var fCursor = 0, oCursor = nFoundations;
        for (let i2 = 0; i2 < n; i2++) {
            if (state.cats[i2] === 11) objOrder[fCursor++] = i2;
            else objOrder[oCursor++] = i2;
        }

        var objInstData = new Float32Array(n * 11);
        let i;
        for (let si = 0; si < n; si++) {
            i = objOrder[si];
            objInstData[si * 11]     = state.positions[i * 2];
            objInstData[si * 11 + 1] = state.positions[i * 2 + 1];
            objInstData[si * 11 + 2] = state.zValues[i];
            objInstData[si * 11 + 3] = state.cats[i];
            objInstData[si * 11 + 4] = state.types[i];
            objInstData[si * 11 + 5] = state.yaws[i];
            var effHw = getRenderHw(state, i);
            var effHd = getRenderHd(state, i);
            objInstData[si * 11 + 6] = effHw > 0 ? effHw : 1.0;
            objInstData[si * 11 + 7] = effHd > 0 ? effHd : 1.0;
            // Per-instance color: resource nodes get type color, others get [0,0,0] (unused)
            if (state.cats[i] === CAT.RESOURCE_NODE) {
                var info = state.typeInfo[state.types[i]];
                var path = info ? info.path : '';
                var glCol = resourceTypeColorGL(path);
                objInstData[si * 11 + 8] = glCol[0];
                objInstData[si * 11 + 9] = glCol[1];
                objInstData[si * 11 + 10] = glCol[2];
            } else {
                objInstData[si * 11 + 8] = 0;
                objInstData[si * 11 + 9] = 0;
                objInstData[si * 11 + 10] = 0;
            }
        }
        state.glObjInstVBO = gl.createBuffer();
        gl.bindBuffer(gl.ARRAY_BUFFER, state.glObjInstVBO);
        gl.bufferData(gl.ARRAY_BUFFER, objInstData, gl.STATIC_DRAW);
        state.glObjCount = n;

        // --- Line segment VBOs ---
        // Expand polylines to segments. Counted first so the buffers can be
        // allocated as typed arrays directly — plain JS arrays box every number
        // and cost ~8 bytes each, which is tens of MB of transient garbage on
        // large saves.
        var segTotal = 0;
        for (let p = 0; p < state.polylines.length; p++) {
            var pc = state.polylines[p].pointCount;
            if (pc > 1) segTotal += pc - 1;
        }
        var segStart = new Float32Array(segTotal * 2);
        var segEnd = new Float32Array(segTotal * 2);
        var segCat = new Float32Array(segTotal);
        var segType = new Float32Array(segTotal);
        var segZ = new Float32Array(segTotal);
        var si = 0;
        for (let p = 0; p < state.polylines.length; p++) {
            var pl = state.polylines[p];
            var plZ = pl.z || 0;
            for (let j = 0; j < pl.pointCount - 1; j++) {
                segStart[si * 2] = pl.points[j * 2];
                segStart[si * 2 + 1] = pl.points[j * 2 + 1];
                segEnd[si * 2] = pl.points[(j + 1) * 2];
                segEnd[si * 2 + 1] = pl.points[(j + 1) * 2 + 1];
                segCat[si] = pl.cat;
                segType[si] = pl.typeIdx;
                segZ[si] = plZ;
                si++;
            }
        }
        state.glLineSegCount = si;

        // Line quad: 6 vertices, x: -0.5..0.5 along segment, y: -0.5..0.5 perpendicular
        var lineQuadVerts = new Float32Array([
            -0.5, -0.5,  0.5, -0.5,  0.5, 0.5,
            -0.5, -0.5,  0.5, 0.5,  -0.5, 0.5
        ]);
        state.glLineQuadVBO = gl.createBuffer();
        gl.bindBuffer(gl.ARRAY_BUFFER, state.glLineQuadVBO);
        gl.bufferData(gl.ARRAY_BUFFER, lineQuadVerts, gl.STATIC_DRAW);

        state.glLineStartVBO = gl.createBuffer();
        gl.bindBuffer(gl.ARRAY_BUFFER, state.glLineStartVBO);
        gl.bufferData(gl.ARRAY_BUFFER, segStart, gl.STATIC_DRAW);

        state.glLineEndVBO = gl.createBuffer();
        gl.bindBuffer(gl.ARRAY_BUFFER, state.glLineEndVBO);
        gl.bufferData(gl.ARRAY_BUFFER, segEnd, gl.STATIC_DRAW);

        state.glLineCatVBO = gl.createBuffer();
        gl.bindBuffer(gl.ARRAY_BUFFER, state.glLineCatVBO);
        gl.bufferData(gl.ARRAY_BUFFER, segCat, gl.STATIC_DRAW);

        state.glLineTypeVBO = gl.createBuffer();
        gl.bindBuffer(gl.ARRAY_BUFFER, state.glLineTypeVBO);
        gl.bufferData(gl.ARRAY_BUFFER, segType, gl.STATIC_DRAW);

        state.glLineZVBO = gl.createBuffer();
        gl.bindBuffer(gl.ARRAY_BUFFER, state.glLineZVBO);
        gl.bufferData(gl.ARRAY_BUFFER, segZ, gl.STATIC_DRAW);

        // --- Map texture ---
        if (state.mapImg && state.mapImg.complete) {
            _uploadMapTexture(state, gl);
        } else if (state.mapImgSrc) {
            state.mapImg = new Image();
            state.mapImg.onload = function() {
                // The context may have been replaced while the image was loading.
                if (state.gl !== gl) return;
                _uploadMapTexture(state, gl);
                renderMapFrame(state);
            };
            state.mapImg.src = state.mapImgSrc;
        }

        // Fullscreen quad for map image
        var fsQuad = new Float32Array([-1, -1, 1, -1, 1, 1, -1, -1, 1, 1, -1, 1]);
        state.glMapQuadVBO = gl.createBuffer();
        gl.bindBuffer(gl.ARRAY_BUFFER, state.glMapQuadVBO);
        gl.bufferData(gl.ARRAY_BUFFER, fsQuad, gl.STATIC_DRAW);

        // Get uniform locations
        var op = state.glObjProgram;
        state.glObjUniforms = {
            camCenter: gl.getUniformLocation(op, 'u_camCenter'),
            zoom: gl.getUniformLocation(op, 'u_zoom'),
            viewport: gl.getUniformLocation(op, 'u_viewport'),
            catVisible: gl.getUniformLocation(op, 'u_catVisible'),
            typeVisible: gl.getUniformLocation(op, 'u_typeVisible'),
            purityVisible: gl.getUniformLocation(op, 'u_purityVisible'),
            zEnabled: gl.getUniformLocation(op, 'u_zEnabled'),
            zMin: gl.getUniformLocation(op, 'u_zMin'),
            zMax: gl.getUniformLocation(op, 'u_zMax'),
            radiusActive: gl.getUniformLocation(op, 'u_radiusActive'),
            radiusCenter: gl.getUniformLocation(op, 'u_radiusCenter'),
            radiusMeters: gl.getUniformLocation(op, 'u_radiusMeters'),
            catColors: gl.getUniformLocation(op, 'u_catColors'),
            foundationAlpha: gl.getUniformLocation(op, 'u_foundationAlpha')
        };
        state.glObjAttrs = {
            quadPos: gl.getAttribLocation(op, 'a_quadPos'),
            instPos: gl.getAttribLocation(op, 'a_instPos'),
            instZ: gl.getAttribLocation(op, 'a_instZ'),
            instCat: gl.getAttribLocation(op, 'a_instCat'),
            instTypeIdx: gl.getAttribLocation(op, 'a_instTypeIdx'),
            instYaw: gl.getAttribLocation(op, 'a_instYaw'),
            instHalfDim: gl.getAttribLocation(op, 'a_instHalfDim'),
            instColor: gl.getAttribLocation(op, 'a_instColor')
        };

        var lp = state.glLineProgram;
        state.glLineUniforms = {
            camCenter: gl.getUniformLocation(lp, 'u_camCenter'),
            zoom: gl.getUniformLocation(lp, 'u_zoom'),
            viewport: gl.getUniformLocation(lp, 'u_viewport'),
            catWidth: gl.getUniformLocation(lp, 'u_catWidth'),
            catExtend: gl.getUniformLocation(lp, 'u_catExtend'),
            catVisible: gl.getUniformLocation(lp, 'u_catVisible'),
            typeVisible: gl.getUniformLocation(lp, 'u_typeVisible'),
            zEnabled: gl.getUniformLocation(lp, 'u_zEnabled'),
            zMin: gl.getUniformLocation(lp, 'u_zMin'),
            zMax: gl.getUniformLocation(lp, 'u_zMax'),
            radiusActive: gl.getUniformLocation(lp, 'u_radiusActive'),
            radiusCenter: gl.getUniformLocation(lp, 'u_radiusCenter'),
            radiusMeters: gl.getUniformLocation(lp, 'u_radiusMeters'),
            catColors: gl.getUniformLocation(lp, 'u_catColors')
        };
        state.glLineAttrs = {
            quadPos: gl.getAttribLocation(lp, 'a_quadPos'),
            instStart: gl.getAttribLocation(lp, 'a_instStart'),
            instEnd: gl.getAttribLocation(lp, 'a_instEnd'),
            instCat: gl.getAttribLocation(lp, 'a_instCat'),
            instTypeIdx: gl.getAttribLocation(lp, 'a_instTypeIdx'),
            instZ: gl.getAttribLocation(lp, 'a_instZ')
        };

        var mp = state.glMapProgram;
        state.glMapUniforms = {
            mapWorldTL: gl.getUniformLocation(mp, 'u_mapWorldTL'),
            mapWorldBR: gl.getUniformLocation(mp, 'u_mapWorldBR'),
            camCenter: gl.getUniformLocation(mp, 'u_camCenter'),
            zoom: gl.getUniformLocation(mp, 'u_zoom'),
            viewport: gl.getUniformLocation(mp, 'u_viewport'),
            mapTex: gl.getUniformLocation(mp, 'u_mapTex')
        };
        state.glMapAttrs = {
            pos: gl.getAttribLocation(mp, 'a_pos')
        };

        // Compute map world bounds
        var worldTL = mapPixelToWorld(0, 0);
        var worldBR = mapPixelToWorld(MAP_SIZE, MAP_SIZE);
        state.glMapWorldTL = worldTL;
        state.glMapWorldBR = worldBR;

    }

    // Releases every GPU object held by state and force-loses the context.
    // renderMap() rebuilds the map tab's DOM (and therefore the canvas + context)
    // on every language switch; without this the old context and its buffers are
    // never reclaimed and the browser eventually kills the oldest live context.
    function disposeWebGL(state) {
        var gl = state.gl;
        if (state._highlightAnim) { cancelAnimationFrame(state._highlightAnim); state._highlightAnim = null; }
        if (state._highlightTimer) { clearTimeout(state._highlightTimer); state._highlightTimer = null; }
        if (!gl) return;
        try {
            ['glObjProgram', 'glLineProgram', 'glMapProgram'].forEach(function(k) {
                if (state[k]) { gl.deleteProgram(state[k]); state[k] = null; }
            });
            ['glQuadVBO', 'glObjInstVBO', 'glLineQuadVBO', 'glLineStartVBO', 'glLineEndVBO',
             'glLineCatVBO', 'glLineTypeVBO', 'glLineZVBO', 'glMapQuadVBO'].forEach(function(k) {
                if (state[k]) { gl.deleteBuffer(state[k]); state[k] = null; }
            });
            if (state.mapTexture) { gl.deleteTexture(state.mapTexture); }
            var lose = gl.getExtension('WEBGL_lose_context');
            if (lose) lose.loseContext();
        } catch (e) {
            console.warn('WebGL dispose failed:', e);
        }
        // Must be cleared regardless: the next context cannot delete this texture.
        state.mapTexture = null;
        state.gl = null;
    }

    function _uploadMapTexture(state, gl) {
        if (state.mapTexture) gl.deleteTexture(state.mapTexture);
        state.mapTexture = gl.createTexture();
        gl.bindTexture(gl.TEXTURE_2D, state.mapTexture);
        gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, state.mapImg);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    }

    // The uniform-upload arrays below are refilled in place on every frame
    // rather than reallocated. renderWebGL runs on every pan, zoom and slider
    // drag, so a fresh typed array per call per array was steady GC pressure
    // for buffers that are at most 66 floats long.
    function _getCatVisibleArray(state) {
        var arr = state._catVisibleBuf || (state._catVisibleBuf = new Int32Array(NUM_CATS));
        for (let i = 0; i < NUM_CATS; i++) {
            arr[i] = isCatVisible(state, i) ? 1 : 0;
        }
        return arr;
    }

    function _getPurityVisibleArray(state) {
        var arr = state._purityVisibleBuf || (state._purityVisibleBuf = new Int32Array(3));
        for (let i = 0; i < 3; i++) {
            arr[i] = isPurityVisible(state, i) ? 1 : 0;
        }
        return arr;
    }

    // Mapping from cat ID to mapColors key (used by settings)
    var CAT_COLOR_KEYS = {
        0: 'production', 1: 'miner', 2: 'station', 3: 'power_pole',
        4: 'belt', 5: 'pipe', 6: 'rail', 7: 'hypertube',
        9: 'resource_node', 10: 'structure', 11: 'foundation',
        12: 'blue_slug', 13: 'yellow_slug', 14: 'purple_slug',
        15: 'somersloop', 16: 'mercer_sphere', 17: 'crash_site',
        18: 'power_gen', 19: 'power_storage', 20: 'signal', 21: 'special'
    };

    // Hardcoded colors per resource type (not customizable in Map Settings).
    // Keyed by the synthetic type path used in the backend.
    var RESOURCE_TYPE_COLORS = {
        '__resource_iron_ore__':      '#b0b0b0',
        '__resource_stone__':         '#c8b8a0',
        '__resource_coal__':          '#404040',
        '__resource_water__':         '#4488ff',
        '__resource_copper_ore__':    '#ff8844',
        '__resource_crude_oil__':     '#556622',
        '__resource_nitrogen_gas__':  '#44ddaa',
        '__resource_geyser__':        '#ff4444',
        '__resource_sam__':           '#aa44ff',
        '__resource_caterium_ore__':  '#ffdd44',
        '__resource_quartz__':        '#ddaaff',
        '__resource_bauxite__':       '#cc6644',
        '__resource_sulfur__':        '#ddff44',
        '__resource_uranium_ore__':   '#44ff44',
        '__resource_unknown__':       '#888888'
    };

    // i18n keys for resource type display names.
    var RESOURCE_TYPE_NAME_KEYS = {
        '__resource_iron_ore__':      'map.resource.iron_ore',
        '__resource_stone__':         'map.resource.stone',
        '__resource_coal__':          'map.resource.coal',
        '__resource_water__':         'map.resource.water',
        '__resource_copper_ore__':    'map.resource.copper_ore',
        '__resource_crude_oil__':     'map.resource.crude_oil',
        '__resource_nitrogen_gas__':  'map.resource.nitrogen_gas',
        '__resource_geyser__':        'map.resource.geyser',
        '__resource_sam__':           'map.resource.sam',
        '__resource_caterium_ore__':  'map.resource.caterium_ore',
        '__resource_quartz__':        'map.resource.quartz',
        '__resource_bauxite__':       'map.resource.bauxite',
        '__resource_sulfur__':        'map.resource.sulfur',
        '__resource_uranium_ore__':   'map.resource.uranium_ore',
        '__resource_unknown__':       'map.unknown'
    };

    // Purity encoded in yaw: 0=impure, 1=normal, 2=pure
    var PURITY_KEYS = ['map.purity.impure', 'map.purity.normal', 'map.purity.pure'];

    function resourceTypeColor(typePath) {
        return RESOURCE_TYPE_COLORS[typePath] || '#888888';
    }

    function resourceTypeName(typePath) {
        var key = RESOURCE_TYPE_NAME_KEYS[typePath];
        return key ? t(key) : t('map.unknown');
    }

    function purityName(yaw) {
        var idx = Math.round(yaw);
        if (idx < 0 || idx > 2) return t('map.purity.impure');
        return t(PURITY_KEYS[idx]);
    }

    // Build flat [r,g,b, r,g,b, ...] array for u_catColors uniform.
    // Merges DATA.mapColors (hex strings) over CAT_COLORS_GL defaults.
    // DATA.mapColors is baked into the report and never changes at runtime, so
    // the resolved array is built once and reused.
    var _catColorsCache = null;
    function _getCatColorsArray() {
        if (_catColorsCache) return _catColorsCache;
        var arr = new Float32Array(NUM_CATS * 3);
        var mapColors = (typeof DATA !== 'undefined' && DATA.mapColors) ? DATA.mapColors : null;
        for (let i = 0; i < NUM_CATS; i++) {
            var glCol = CAT_COLORS_GL[i] || [1, 1, 1];
            if (mapColors) {
                var key = CAT_COLOR_KEYS[i];
                if (key && mapColors[key]) {
                    var hex = mapColors[key];
                    if (hex && hex[0] === '#') {
                        glCol = [
                            parseInt(hex.substr(1, 2), 16) / 255,
                            parseInt(hex.substr(3, 2), 16) / 255,
                            parseInt(hex.substr(5, 2), 16) / 255
                        ];
                    }
                }
            }
            arr[i * 3] = glCol[0];
            arr[i * 3 + 1] = glCol[1];
            arr[i * 3 + 2] = glCol[2];
        }
        _catColorsCache = arr;
        return arr;
    }

    // Get hex color for a category (for overlay/controls), merging DATA.mapColors with defaults
    function _getCatColorHex(cat) {
        var mapColors = (typeof DATA !== 'undefined' && DATA.mapColors) ? DATA.mapColors : null;
        if (mapColors) {
            var key = CAT_COLOR_KEYS[cat];
            if (key && mapColors[key]) return mapColors[key];
        }
        return CAT_COLORS[cat] || '#ffffff';
    }

    // Line width/extend are compile-time constants, so both arrays are built
    // once on first use.
    var LINE_CATS = [CAT.BELT, CAT.PIPE, CAT.RAIL, CAT.HYPERTUBE, CAT.POWER_POLE];

    var _catWidthCache = null;
    function _getCatWidthArray() {
        if (_catWidthCache) return _catWidthCache;
        var arr = new Float32Array(NUM_CATS);
        for (let i = 0; i < LINE_CATS.length; i++) {
            arr[LINE_CATS[i]] = CAT_LINE_WIDTH[LINE_CATS[i]] || 1.0;
        }
        _catWidthCache = arr;
        return arr;
    }

    var _catExtendCache = null;
    function _getCatExtendArray() {
        if (_catExtendCache) return _catExtendCache;
        var arr = new Float32Array(NUM_CATS);
        for (let i = 0; i < LINE_CATS.length; i++) {
            arr[LINE_CATS[i]] = CAT_LINE_EXTEND[LINE_CATS[i]] || 0.15;
        }
        _catExtendCache = arr;
        return arr;
    }

    function renderWebGL(state) {
        var gl = state.gl;
        if (!gl || !state.glObjProgram) return;

        var dpr = getCanvasDPR(state);
        var vpW = state.canvas.width;
        var vpH = state.canvas.height;

        gl.viewport(0, 0, vpW, vpH);
        gl.clearColor(0.1, 0.1, 0.1, 1.0);
        gl.clear(gl.COLOR_BUFFER_BIT);
        gl.enable(gl.BLEND);
        gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);

        var catVisible = _getCatVisibleArray(state);
        var catColors = _getCatColorsArray();
        var typeVisible = state.typeVisible || new Int32Array(512);
        // camCenter/viewport uniforms set inline via gl.uniform2f calls below

        // --- Draw map image ---
        if (state.mapTexture) {
            gl.useProgram(state.glMapProgram);
            gl.uniform2f(state.glMapUniforms.mapWorldTL, state.glMapWorldTL[0], state.glMapWorldTL[1]);
            gl.uniform2f(state.glMapUniforms.mapWorldBR, state.glMapWorldBR[0], state.glMapWorldBR[1]);
            gl.uniform2f(state.glMapUniforms.camCenter, state.cam.cx, state.cam.cy);
            gl.uniform1f(state.glMapUniforms.zoom, state.cam.zoom * dpr);
            gl.uniform2f(state.glMapUniforms.viewport, vpW, vpH);
            gl.uniform1i(state.glMapUniforms.mapTex, 0);
            gl.activeTexture(gl.TEXTURE0);
            gl.bindTexture(gl.TEXTURE_2D, state.mapTexture);

            gl.bindBuffer(gl.ARRAY_BUFFER, state.glMapQuadVBO);
            gl.enableVertexAttribArray(state.glMapAttrs.pos);
            gl.vertexAttribPointer(state.glMapAttrs.pos, 2, gl.FLOAT, false, 0, 0);
            gl.drawArrays(gl.TRIANGLES, 0, 6);
        }

        // --- Draw polylines (thick lines) ---
        if (state.glLineSegCount > 0) {
            gl.useProgram(state.glLineProgram);
            gl.uniform2f(state.glLineUniforms.camCenter, state.cam.cx, state.cam.cy);
            gl.uniform1f(state.glLineUniforms.zoom, state.cam.zoom * dpr);
            gl.uniform2f(state.glLineUniforms.viewport, vpW, vpH);
            gl.uniform1fv(state.glLineUniforms.catWidth, _getCatWidthArray());
            gl.uniform1fv(state.glLineUniforms.catExtend, _getCatExtendArray());
            gl.uniform1iv(state.glLineUniforms.catVisible, catVisible);
            gl.uniform1iv(state.glLineUniforms.typeVisible, typeVisible);
            gl.uniform3fv(state.glLineUniforms.catColors, catColors);
            gl.uniform1i(state.glLineUniforms.zEnabled, state.zEnabled ? 1 : 0);
            gl.uniform1f(state.glLineUniforms.zMin, state.zFilterMin);
            gl.uniform1f(state.glLineUniforms.zMax, state.zFilterMax);
            gl.uniform1i(state.glLineUniforms.radiusActive, state.radius.active ? 1 : 0);
            gl.uniform2f(state.glLineUniforms.radiusCenter, state.radius.cx, state.radius.cy);
            gl.uniform1f(state.glLineUniforms.radiusMeters, state.radius.meters);

            // Quad VBO
            gl.bindBuffer(gl.ARRAY_BUFFER, state.glLineQuadVBO);
            gl.enableVertexAttribArray(state.glLineAttrs.quadPos);
            gl.vertexAttribPointer(state.glLineAttrs.quadPos, 2, gl.FLOAT, false, 0, 0);

            // Instance attributes
            gl.bindBuffer(gl.ARRAY_BUFFER, state.glLineStartVBO);
            gl.enableVertexAttribArray(state.glLineAttrs.instStart);
            gl.vertexAttribPointer(state.glLineAttrs.instStart, 2, gl.FLOAT, false, 0, 0);
            gl.vertexAttribDivisor(state.glLineAttrs.instStart, 1);

            gl.bindBuffer(gl.ARRAY_BUFFER, state.glLineEndVBO);
            gl.enableVertexAttribArray(state.glLineAttrs.instEnd);
            gl.vertexAttribPointer(state.glLineAttrs.instEnd, 2, gl.FLOAT, false, 0, 0);
            gl.vertexAttribDivisor(state.glLineAttrs.instEnd, 1);

            gl.bindBuffer(gl.ARRAY_BUFFER, state.glLineCatVBO);
            gl.enableVertexAttribArray(state.glLineAttrs.instCat);
            gl.vertexAttribPointer(state.glLineAttrs.instCat, 1, gl.FLOAT, false, 0, 0);
            gl.vertexAttribDivisor(state.glLineAttrs.instCat, 1);

            gl.bindBuffer(gl.ARRAY_BUFFER, state.glLineTypeVBO);
            gl.enableVertexAttribArray(state.glLineAttrs.instTypeIdx);
            gl.vertexAttribPointer(state.glLineAttrs.instTypeIdx, 1, gl.FLOAT, false, 0, 0);
            gl.vertexAttribDivisor(state.glLineAttrs.instTypeIdx, 1);

            gl.bindBuffer(gl.ARRAY_BUFFER, state.glLineZVBO);
            gl.enableVertexAttribArray(state.glLineAttrs.instZ);
            gl.vertexAttribPointer(state.glLineAttrs.instZ, 1, gl.FLOAT, false, 0, 0);
            gl.vertexAttribDivisor(state.glLineAttrs.instZ, 1);

            gl.drawArraysInstanced(gl.TRIANGLES, 0, 6, state.glLineSegCount);

            // Reset divisors
            gl.vertexAttribDivisor(state.glLineAttrs.instStart, 0);
            gl.vertexAttribDivisor(state.glLineAttrs.instEnd, 0);
            gl.vertexAttribDivisor(state.glLineAttrs.instCat, 0);
            gl.vertexAttribDivisor(state.glLineAttrs.instTypeIdx, 0);
            gl.vertexAttribDivisor(state.glLineAttrs.instZ, 0);
        }

        // --- Draw objects (instanced quads) ---
        if (state.glObjCount > 0) {
            gl.useProgram(state.glObjProgram);
            gl.uniform2f(state.glObjUniforms.camCenter, state.cam.cx, state.cam.cy);
            gl.uniform1f(state.glObjUniforms.zoom, state.cam.zoom * dpr);
            gl.uniform2f(state.glObjUniforms.viewport, vpW, vpH);
            gl.uniform1iv(state.glObjUniforms.catVisible, catVisible);
            gl.uniform1iv(state.glObjUniforms.typeVisible, typeVisible);
            gl.uniform1iv(state.glObjUniforms.purityVisible, _getPurityVisibleArray(state));
            gl.uniform3fv(state.glObjUniforms.catColors, catColors);
            gl.uniform1i(state.glObjUniforms.zEnabled, state.zEnabled ? 1 : 0);
            gl.uniform1f(state.glObjUniforms.zMin, state.zFilterMin);
            gl.uniform1f(state.glObjUniforms.zMax, state.zFilterMax);
            gl.uniform1i(state.glObjUniforms.radiusActive, state.radius.active ? 1 : 0);
            gl.uniform2f(state.glObjUniforms.radiusCenter, state.radius.cx, state.radius.cy);
            gl.uniform1f(state.glObjUniforms.radiusMeters, state.radius.meters);
            gl.uniform1f(state.glObjUniforms.foundationAlpha, state.foundationAlpha != null ? state.foundationAlpha : 0.5);

            // Quad VBO
            gl.bindBuffer(gl.ARRAY_BUFFER, state.glQuadVBO);
            gl.enableVertexAttribArray(state.glObjAttrs.quadPos);
            gl.vertexAttribPointer(state.glObjAttrs.quadPos, 2, gl.FLOAT, false, 0, 0);

            // Instance data: 11 floats per instance [posX, posY, posZ, cat, typeIdx, yaw, halfW, halfD, r, g, b]
            gl.bindBuffer(gl.ARRAY_BUFFER, state.glObjInstVBO);
            var stride = 11 * 4; // 11 floats
            gl.enableVertexAttribArray(state.glObjAttrs.instPos);
            gl.vertexAttribPointer(state.glObjAttrs.instPos, 2, gl.FLOAT, false, stride, 0);
            gl.vertexAttribDivisor(state.glObjAttrs.instPos, 1);

            gl.enableVertexAttribArray(state.glObjAttrs.instZ);
            gl.vertexAttribPointer(state.glObjAttrs.instZ, 1, gl.FLOAT, false, stride, 8);
            gl.vertexAttribDivisor(state.glObjAttrs.instZ, 1);

            gl.enableVertexAttribArray(state.glObjAttrs.instCat);
            gl.vertexAttribPointer(state.glObjAttrs.instCat, 1, gl.FLOAT, false, stride, 12);
            gl.vertexAttribDivisor(state.glObjAttrs.instCat, 1);

            gl.enableVertexAttribArray(state.glObjAttrs.instTypeIdx);
            gl.vertexAttribPointer(state.glObjAttrs.instTypeIdx, 1, gl.FLOAT, false, stride, 16);
            gl.vertexAttribDivisor(state.glObjAttrs.instTypeIdx, 1);

            gl.enableVertexAttribArray(state.glObjAttrs.instYaw);
            gl.vertexAttribPointer(state.glObjAttrs.instYaw, 1, gl.FLOAT, false, stride, 20);
            gl.vertexAttribDivisor(state.glObjAttrs.instYaw, 1);

            gl.enableVertexAttribArray(state.glObjAttrs.instHalfDim);
            gl.vertexAttribPointer(state.glObjAttrs.instHalfDim, 2, gl.FLOAT, false, stride, 24);
            gl.vertexAttribDivisor(state.glObjAttrs.instHalfDim, 1);

            gl.enableVertexAttribArray(state.glObjAttrs.instColor);
            gl.vertexAttribPointer(state.glObjAttrs.instColor, 3, gl.FLOAT, false, stride, 32);
            gl.vertexAttribDivisor(state.glObjAttrs.instColor, 1);

            gl.drawArraysInstanced(gl.TRIANGLES, 0, 6, state.glObjCount);

            // Reset divisors
            gl.vertexAttribDivisor(state.glObjAttrs.instPos, 0);
            gl.vertexAttribDivisor(state.glObjAttrs.instZ, 0);
            gl.vertexAttribDivisor(state.glObjAttrs.instCat, 0);
            gl.vertexAttribDivisor(state.glObjAttrs.instTypeIdx, 0);
            gl.vertexAttribDivisor(state.glObjAttrs.instYaw, 0);
            gl.vertexAttribDivisor(state.glObjAttrs.instHalfDim, 0);
            gl.vertexAttribDivisor(state.glObjAttrs.instColor, 0);
        }

    }

    var _objCountTimer = null;
    function updateObjCount(state) {
        if (_objCountTimer) return;
        _objCountTimer = setTimeout(function() {
            _objCountTimer = null;
            _updateObjCountImpl(state);
        }, 200);
    }

    function _updateObjCountImpl(state) {
        var el = document.getElementById('mapObjCount');
        if (!el) return;

        // Grid path counts only what is in the viewport, so it reports
        // "visible"; the fallback walks everything and reports the plain total.
        // Apart from the candidate source and the label the two are identical.
        var indices = state.grid ? getVisibleObjectIndices(state) : null;
        var n = indices ? indices.length : state.count;
        var visible = 0;
        for (let ci = 0; ci < n; ci++) {
            let i = indices ? indices[ci] : ci;
            if (isCatVisible(state, state.cats[i]) && isTypeVisible(state, state.types[i]) &&
                !(state.cats[i] === CAT.RESOURCE_NODE && !isPurityVisible(state, state.yaws[i])) &&
                !isZFiltered(state, state.zValues[i]) &&
                !isRadiusFiltered(state, state.positions[i * 2], state.positions[i * 2 + 1])) {
                visible++;
            }
        }
        var key = indices ? 'map.objects_visible' : 'map.objects';
        el.textContent = t(key, visible.toLocaleString(), state.count.toLocaleString());
    }
