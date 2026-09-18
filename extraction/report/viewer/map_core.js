    // ============================================================
    // MAP TAB — WebGL2 instanced renderer
    // Map Developed by Paul and Physox
    // ============================================================

    // Calibration constants
    var MAP_SIZE = 8192.0;
    var WORLD_TO_PIXEL_SCALE = 22.887;
    var WORLD_OFFSET_X = 18282.5;
    var WORLD_OFFSET_Y = 20480.0;
    var OLD_MAP_DESCALE = 20.0;
    var CROP_LO = 204.8;
    var CROP_SPAN = 1638.4;
    var SCALE_TO_HIGHRES = MAP_SIZE / CROP_SPAN;

    // Category constants (must match Go-side MapCat* constants)
    var NUM_CATS = 22;

    var CAT = {
        BUILDING: 0,
        MINER: 1,
        STATION: 2,
        POWER_POLE: 3,
        BELT: 4,
        PIPE: 5,
        RAIL: 6,
        HYPERTUBE: 7,
        RESOURCE_NODE: 9,
        STRUCTURE: 10,
        FOUNDATION: 11,
        BLUE_SLUG: 12,
        YELLOW_SLUG: 13,
        PURPLE_SLUG: 14,
        SOMERSLOOP: 15,
        MERCER_SPHERE: 16,
        CRASH_SITE: 17,
        POWER_GEN: 18,
        POWER_STORAGE: 19,
        SIGNAL: 20,
        SPECIAL: 21
    };

    var CAT_NAMES = {
        0: 'map.cat.production', 1: 'map.cat.miners', 2: 'map.cat.stations', 3: 'map.cat.power_poles',
        4: 'map.cat.belts', 5: 'map.cat.pipes', 6: 'map.cat.rails', 7: 'map.cat.hypertubes',
        9: 'map.cat.resource_nodes', 10: 'map.cat.structures', 11: 'map.cat.foundations',
        12: 'map.cat.blue_slugs', 13: 'map.cat.yellow_slugs', 14: 'map.cat.purple_slugs',
        15: 'map.cat.somersloops', 16: 'map.cat.mercer_spheres', 17: 'map.cat.crash_sites',
        18: 'map.cat.power_gen', 19: 'map.cat.power_storage', 20: 'map.cat.signal', 21: 'map.cat.special'
    };

    function catName(cat) {
        var key = CAT_NAMES[cat];
        return key ? t(key) : t('map.unknown');
    }

    // Length of the per-type visibility array (and of the matching shader uniform).
    // Minimum 512 for backwards compatibility, rounded up to a multiple of 64.
    function typeVisibleSize(typeCount) {
        var n = Math.max(512, typeCount);
        return Math.ceil(n / 64) * 64;
    }

    // Resolve a type index to a language-aware display name via the frontend's
    // displayName() lookup (uses itemDisplayNames for the current language).
    function mapTypeName(state, ti) {
        var info = state.typeInfo[ti];
        if (!info) return t('map.unknown');
        if (info.path && info.path.indexOf('__resource_') === 0) {
            return resourceTypeName(info.path);
        }
        return displayName(info.path);
    }

    var CAT_COLORS = {
        0: '#4a9eff', 1: '#ffaa44', 2: '#ff66aa', 3: '#ddaa44',
        4: '#88dd88', 5: '#ff8800', 6: '#aa88ff', 7: '#ff8866',
        9: '#88ff88', 10: '#888888', 11: '#cccccc',
        12: '#4a9eff', 13: '#f5a623', 14: '#9b59b6',
        15: '#e91e63', 16: '#a855f7', 17: '#e67e22',
        18: '#ff6600', 19: '#ffdd44', 20: '#ff4444', 21: '#fbbf24'
    };

    // Default category visibility
    var DEFAULT_CAT_VISIBLE = {};
    DEFAULT_CAT_VISIBLE[CAT.BUILDING] = true;
    DEFAULT_CAT_VISIBLE[CAT.MINER] = true;
    DEFAULT_CAT_VISIBLE[CAT.STATION] = true;
    DEFAULT_CAT_VISIBLE[CAT.POWER_POLE] = true;
    DEFAULT_CAT_VISIBLE[CAT.BELT] = true;
    DEFAULT_CAT_VISIBLE[CAT.PIPE] = true;
    DEFAULT_CAT_VISIBLE[CAT.RAIL] = true;
    DEFAULT_CAT_VISIBLE[CAT.HYPERTUBE] = true;
    DEFAULT_CAT_VISIBLE[CAT.RESOURCE_NODE] = false;
    DEFAULT_CAT_VISIBLE[CAT.STRUCTURE] = false;
    DEFAULT_CAT_VISIBLE[CAT.FOUNDATION] = false;
    DEFAULT_CAT_VISIBLE[CAT.BLUE_SLUG] = false;
    DEFAULT_CAT_VISIBLE[CAT.YELLOW_SLUG] = false;
    DEFAULT_CAT_VISIBLE[CAT.PURPLE_SLUG] = false;
    DEFAULT_CAT_VISIBLE[CAT.SOMERSLOOP] = false;
    DEFAULT_CAT_VISIBLE[CAT.MERCER_SPHERE] = false;
    DEFAULT_CAT_VISIBLE[CAT.CRASH_SITE] = false;
    DEFAULT_CAT_VISIBLE[CAT.POWER_GEN] = true;
    DEFAULT_CAT_VISIBLE[CAT.POWER_STORAGE] = true;
    DEFAULT_CAT_VISIBLE[CAT.SIGNAL] = true;
    DEFAULT_CAT_VISIBLE[CAT.SPECIAL] = true;

    // Read IEEE 754 half-precision (float16) from a DataView at given offset.
    function readFloat16(view, offset) {
        var h = view.getUint16(offset, true);
        var sign = (h >> 15) & 1;
        var exp = (h >> 10) & 0x1F;
        var mant = h & 0x3FF;
        var val;
        if (exp === 0) {
            if (mant === 0) {
                val = 0;
            } else {
                // Subnormal
                val = mant / 1024 * Math.pow(2, -14);
            }
        } else if (exp === 0x1F) {
            val = mant !== 0 ? NaN : Infinity;
        } else {
            val = (1 + mant / 1024) * Math.pow(2, exp - 15);
        }
        return sign ? -val : val;
    }

    // Maximum perpendicular deviation (meters) at which a Bezier span is treated
    // as a straight line and emitted as a single segment. 5cm is far below the
    // rendered width of a belt, so collapsing these spans is visually lossless.
    // float16 tangent quantisation contributes at most ~2cm of error, so
    // genuinely straight spans always fall inside this tolerance.
    var SPAN_FLAT_TOL = 0.05;

    // Below this chord length a span is considered degenerate in XY — vertical
    // conveyor lifts and vertical pipe runs collapse to a single point on a 2D
    // map, and emitting them produces zero-length line quads.
    var SPAN_DEGENERATE_CHORD = 0.001;

    // Subdivide Hermite spline segments into line segments for rendering.
    // Each control point has: x, y (position), ltx, lty (leave tangent),
    // atx, aty (arrive tangent). Tangents are in world meters.
    // Hermite -> Bezier: CP1 = P0 + LeaveTan/3, CP2 = P1 - ArriveTan/3
    // Straight spans collapse to one segment; curved spans subdivide adaptively.
    function subdivideHermite(cps) {
        if (cps.length < 2) return new Float32Array(0);
        var pts = [cps[0].x, cps[0].y];
        for (var i = 0; i < cps.length - 1; i++) {
            var p0 = cps[i], p1 = cps[i + 1];
            // Bezier control points from Hermite tangents
            var cp1x = p0.x + p0.ltx / 3;
            var cp1y = p0.y + p0.lty / 3;
            var cp2x = p1.x - p1.atx / 3;
            var cp2y = p1.y - p1.aty / 3;

            var dx = p1.x - p0.x, dy = p1.y - p0.y;
            var chord = Math.hypot(dx, dy);

            if (chord < SPAN_DEGENERATE_CHORD) {
                // Both endpoints coincide in XY. If the control points stay put
                // too there is nothing to draw, so drop the span entirely.
                if (Math.hypot(cp1x - p0.x, cp1y - p0.y) < SPAN_FLAT_TOL &&
                    Math.hypot(cp2x - p1.x, cp2y - p1.y) < SPAN_FLAT_TOL) {
                    continue;
                }
            } else {
                // Flatness test: perpendicular deviation of both Bezier control
                // points from the chord. Also require them to project inside the
                // chord — a cusp can be perpendicular-flat yet still bulge along
                // the chord direction and would be visibly wrong as one segment.
                var ux = dx / chord, uy = dy / chord;
                var perp1 = Math.abs(-uy * (cp1x - p0.x) + ux * (cp1y - p0.y));
                var perp2 = Math.abs(-uy * (cp2x - p0.x) + ux * (cp2y - p0.y));
                if (perp1 < SPAN_FLAT_TOL && perp2 < SPAN_FLAT_TOL) {
                    var along1 = ux * (cp1x - p0.x) + uy * (cp1y - p0.y);
                    var along2 = ux * (cp2x - p0.x) + uy * (cp2y - p0.y);
                    if (along1 >= -SPAN_FLAT_TOL &&
                        along2 <= chord + SPAN_FLAT_TOL &&
                        along1 <= along2 + SPAN_FLAT_TOL) {
                        pts.push(p1.x, p1.y);
                        continue;
                    }
                }
            }

            // Segment length estimate (chord + control polygon)
            var ctrlLen = Math.hypot(cp1x - p0.x, cp1y - p0.y) +
                          Math.hypot(cp2x - cp1x, cp2y - cp1y) +
                          Math.hypot(p1.x - cp2x, p1.y - cp2y);
            var estLen = (chord + ctrlLen) / 2;

            // Adaptive subdivision count: ~1 segment per 2m, min 2, max 24
            var subdivs = Math.max(2, Math.min(24, Math.ceil(estLen / 2)));

            for (var s = 1; s <= subdivs; s++) {
                var t = s / subdivs;
                var mt = 1 - t;
                var x = mt*mt*mt*p0.x + 3*mt*mt*t*cp1x + 3*mt*t*t*cp2x + t*t*t*p1.x;
                var y = mt*mt*mt*p0.y + 3*mt*mt*t*cp1y + 3*mt*t*t*cp2y + t*t*t*p1.y;
                pts.push(x, y);
            }
        }
        // A polyline whose every span was degenerate leaves a single point,
        // which yields zero segments downstream.
        if (pts.length < 4) return new Float32Array(0);
        return new Float32Array(pts);
    }

    // Map state
    var mapState = null;
    var pendingFocus = null; // {x, y, zoom} set by showOnMap before map is ready

    // Called from other tabs via mapLink() to focus the map on a world position.
    // Switches to the map tab, centers the camera, and draws a highlight marker.
    function showOnMap(x, y, zoom) {
        switchTab('map');
        var z = zoom || 2.0; // ~400m visible on typical canvas
        if (!mapState) {
            pendingFocus = { x: x, y: y, zoom: z };
        } else {
            applyMapFocus(mapState, x, y, z);
        }
    }

    // Find the nearest map object to (x, y) in world meters. Searches a 50m radius
    // using the spatial grid. Returns object index or -1 if none found.
    function findNearestObject(state, x, y) {
        var g = state.grid;
        if (!g) return -1;
        var searchR = 50; // meters
        var minDist2 = searchR * searchR;
        var bestIdx = -1;
        var cx0 = Math.floor((x - searchR - g.minX) / GRID_CELL_SIZE);
        var cy0 = Math.floor((y - searchR - g.minY) / GRID_CELL_SIZE);
        var cx1 = Math.floor((x + searchR - g.minX) / GRID_CELL_SIZE);
        var cy1 = Math.floor((y + searchR - g.minY) / GRID_CELL_SIZE);
        cx0 = Math.max(0, cx0); cy0 = Math.max(0, cy0);
        cx1 = Math.min(g.cols - 1, cx1); cy1 = Math.min(g.rows - 1, cy1);
        for (let cy = cy0; cy <= cy1; cy++) {
            for (let cx = cx0; cx <= cx1; cx++) {
                var cell = g.cells[cy * g.cols + cx];
                if (!cell) continue;
                for (let ci = 0; ci < cell.length; ci++) {
                    var i = cell[ci];
                    var dx = state.positions[i * 2] - x;
                    var dy = state.positions[i * 2 + 1] - y;
                    var d2 = dx * dx + dy * dy;
                    if (d2 < minDist2) {
                        minDist2 = d2;
                        bestIdx = i;
                    }
                }
            }
        }
        return bestIdx;
    }

    function applyMapFocus(state, x, y, zoom) {
        state.cam.cx = x;
        state.cam.cy = y;
        state.cam.zoom = zoom;
        state.highlightX = x;
        state.highlightY = y;
        state.highlightTime = performance.now();
        state.highlightIdx = findNearestObject(state, x, y);
        resizeMapCanvas(state);
        renderMapFrame(state);
        drawOverlay(state);
        // Animate the pulsing highlight
        if (state._highlightAnim) cancelAnimationFrame(state._highlightAnim);
        function animateHighlight() {
            var elapsed = (performance.now() - state.highlightTime) / 1000;
            if (elapsed >= 6) {
                state.highlightTime = 0;
                state.highlightIdx = -1;
                drawOverlay(state);
                state._highlightAnim = null;
                return;
            }
            drawOverlay(state);
            state._highlightAnim = requestAnimationFrame(animateHighlight);
        }
        state._highlightAnim = requestAnimationFrame(animateHighlight);
        // Safety: clear after 6.5s
        if (state._highlightTimer) clearTimeout(state._highlightTimer);
        state._highlightTimer = setTimeout(function() {
            state.highlightTime = 0;
            state.highlightIdx = -1;
            if (state._highlightAnim) { cancelAnimationFrame(state._highlightAnim); state._highlightAnim = null; }
            drawOverlay(state);
        }, 6500);
    }

    async function renderMap() {
        var container = document.getElementById('tab-map');
        if (!container) return;

        var mapObjEl = document.getElementById('map-objects');
        if (!DATA.mapData || !mapObjEl || !mapObjEl.textContent.trim()) {
            container.innerHTML = '<div class="empty-state">' + t('map.no_data') + '</div>';
            return;
        }

        // Load map image from the script tag
        var mapImgEl = document.getElementById('map-image');
        var mapImgSrc = mapImgEl ? mapImgEl.textContent : '';

        if (!mapState) {
            mapState = await createMapState(mapImgSrc);
            buildSpatialGrid(mapState);
        } else {
            // Re-render (language switch) replaces the canvas below, which orphans
            // the current WebGL context. Release it first.
            disposeWebGL(mapState);
            // Save group/type expand state before DOM is rebuilt.
            _saveExpandState(mapState);
        }

        // Build the map UI
        var html = '<div class="map-container">';
        html += '<canvas id="mapCanvas" style="width:100%;height:100%;display:block;background:#1a1a1a;cursor:grab;"></canvas>';
        html += '<canvas id="mapOverlay" style="position:absolute;top:0;left:0;width:100%;height:100%;pointer-events:none;z-index:5;"></canvas>';
        html += '<div class="map-controls">';
        html += buildMapControls(mapState);
        html += '</div>';
        html += '<div class="map-info" id="mapInfo"></div>';
        html += '<button class="map-back-btn" id="mapBackBtn" style="display:none;" title="' + esc(t('btn.back_to_tab')) + '">\u2190 ' + esc(t('btn.back')) + '</button>';
        html += '<div class="map-tooltip" id="mapTooltip" style="display:none;"></div>';
        html += '</div>';
        container.innerHTML = html;

        // Initialize canvas — WebGL2 required
        var canvas = document.getElementById('mapCanvas');
        mapState.canvas = canvas;
        mapState.overlay = document.getElementById('mapOverlay');

        mapState.gl = canvas.getContext('webgl2', { antialias: true, premultipliedAlpha: false });
        if (!mapState.gl) {
            container.innerHTML = '<div class="empty-state" style="padding:40px;text-align:center;">' +
                '<div style="font-size:18px;margin-bottom:8px;">' + t('map.no_webgl2') + '</div>' +
                '<div style="color:#888;font-size:13px;">WebGL2 is required for the interactive map. ' +
                'Please use a modern browser (Chrome, Firefox, Edge).</div></div>';
            return;
        }
        initWebGL(mapState);

        // Setup event handlers
        setupMapEvents(mapState);

        // Resize canvas to display size
        resizeMapCanvas(mapState);
        renderMapFrame(mapState);

        // Apply pending focus from showOnMap() if set
        if (pendingFocus) {
            var pf = pendingFocus;
            pendingFocus = null;
            applyMapFocus(mapState, pf.x, pf.y, pf.zoom);
        }
    }

    // Decompress gzip base64 data from a script tag into Uint8Array
    async function decompressScriptTag(id) {
        var el = document.getElementById(id);
        if (!el) return new Uint8Array(0);
        var b64 = el.textContent.trim();
        if (!b64) return new Uint8Array(0);
        var binary = atob(b64);
        var bytes = new Uint8Array(binary.length);
        for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
        var ds = new DecompressionStream('gzip');
        var blob = new Blob([bytes]);
        var stream = blob.stream().pipeThrough(ds);
        var buf = await new Response(stream).arrayBuffer();
        return new Uint8Array(buf);
    }

    async function createMapState(mapImgSrc) {
        var md = DATA.mapData;

        // Decompress binary object data from separate script tag
        // Format: 19 bytes per object [int32 x, int32 y, int16 z, uint8 cat, uint16 typeIdx, int16 yaw, uint16 halfLenOvr, uint16 halfDepthOvr]
        // X/Y in 1cm units (multiply by 0.01 for meters). Z in 10cm units. All little-endian.
        // halfLenOvr: per-instance half-width override in 1cm units (0 = use type default).
        // halfDepthOvr: per-instance half-depth override in 1cm units (0 = use type default).
        var objBytes = await decompressScriptTag('map-objects');
        var objView = new DataView(objBytes.buffer);
        var n = Math.floor(objBytes.length / 19);

        var positions = new Float32Array(n * 2); // x, y in meters
        var zValues = new Float32Array(n);
        var cats = new Uint8Array(n);
        var types = new Uint16Array(n);
        var yaws = new Int16Array(n);
        var halfLenOvr = new Uint16Array(n); // 0 = use type default, else 1cm units
        var halfDepthOvr = new Uint16Array(n); // 0 = use type default, else 1cm units

        for (let i = 0; i < n; i++) {
            var off = i * 19;
            positions[i * 2]     = objView.getInt32(off, true) * 0.01;      // x (1cm units -> meters)
            positions[i * 2 + 1] = objView.getInt32(off + 4, true) * 0.01;  // y (1cm units -> meters)
            zValues[i]           = objView.getInt16(off + 8, true) * 0.1;   // z
            cats[i]              = objView.getUint8(off + 10);              // cat
            types[i]             = objView.getUint16(off + 11, true);       // typeIdx
            yaws[i]              = objView.getInt16(off + 13, true);        // yaw
            halfLenOvr[i]        = objView.getUint16(off + 15, true);       // halfLenOvr
            halfDepthOvr[i]      = objView.getUint16(off + 17, true);       // halfDepthOvr
        }

        // Build type lookup
        var typeInfo = md.types || [];

        // Decompress and decode polyline data
        // Two formats per polyline, selected by high bit of pointCount:
        //
        // Tangent mode (ptCount & 0x8000): raw control points + tangents
        //   Header: uint8 cat, uint16 typeIdx, uint16 ptCount|0x8000, int16 z
        //   Per point: int32 x, int32 y, 4×float16 (leaveTanX, leaveTanY, arriveTanX, arriveTanY)
        //   -> frontend subdivides Hermite segments into line segments
        //
        // Legacy mode: delta-encoded points (power lines)
        //   Header: uint8 cat, uint16 typeIdx, uint16 ptCount, int16 z
        //   First point (int32 x, int32 y) absolute,
        //   Subsequent: (int8 dx, int8 dy) or sentinel 0x80,0x80 + (int16 dx, int16 dy)
        var polylines = [];
        var polyBytes = await decompressScriptTag('map-polylines');
        if (polyBytes.length > 0) {
            var polyView = new DataView(polyBytes.buffer);
            off = 0;
            while (off + 7 <= polyBytes.length) {
                var cat = polyView.getUint8(off);
                let ti = polyView.getUint16(off + 1, true);
                var rawPtCount = polyView.getUint16(off + 3, true);
                var hasTan = (rawPtCount & 0x8000) !== 0;
                var ptCount = rawPtCount & 0x7FFF;
                var polyZ = polyView.getInt16(off + 5, true) * 0.1; // 10cm units -> meters
                off += 7;
                if (ptCount === 0) break;

                if (hasTan) {
                    // Tangent mode: each point is 16 bytes
                    // int32 x, int32 y, 4×float16 tangents
                    if (off + ptCount * 16 > polyBytes.length) break;
                    var cps = [];
                    for (let j = 0; j < ptCount; j++) {
                        var px = polyView.getInt32(off, true) * 0.1;       // 10cm -> meters
                        var py = polyView.getInt32(off + 4, true) * 0.1;
                        var ltx = readFloat16(polyView, off + 8);
                        var lty = readFloat16(polyView, off + 10);
                        var atx = readFloat16(polyView, off + 12);
                        var aty = readFloat16(polyView, off + 14);
                        off += 16;
                        cps.push({ x: px, y: py, ltx: ltx, lty: lty, atx: atx, aty: aty });
                    }
                    // Subdivide Hermite segments into line segments
                    var subPts = subdivideHermite(cps);
                    polylines.push({
                        cat: cat,
                        typeIdx: ti,
                        points: subPts,
                        pointCount: subPts.length / 2,
                        z: polyZ
                    });
                } else {
                    // Legacy delta-encoded mode
                    if (off + 8 > polyBytes.length) break;
                    var curX = polyView.getInt32(off, true);
                    var curY = polyView.getInt32(off + 4, true);
                    off += 8;
                    var pts = new Float32Array(ptCount * 2);
                    pts[0] = curX * 0.1;
                    pts[1] = curY * 0.1;
                    var actualPts = 1;
                    for (let j = 1; j < ptCount; j++) {
                        if (off + 2 > polyBytes.length) break;
                        var b0 = polyBytes[off];
                        var b1 = polyBytes[off + 1];
                        if (b0 === 0x80 && b1 === 0x80) {
                            // 16-bit delta
                            off += 2;
                            if (off + 4 > polyBytes.length) break;
                            curX += polyView.getInt16(off, true);
                            curY += polyView.getInt16(off + 2, true);
                            off += 4;
                        } else if (b0 === 0x80 && b1 === 0x81) {
                            // 32-bit delta
                            off += 2;
                            if (off + 8 > polyBytes.length) break;
                            curX += polyView.getInt32(off, true);
                            curY += polyView.getInt32(off + 4, true);
                            off += 8;
                        } else {
                            // int8 delta
                            curX += (b0 << 24) >> 24; // sign-extend int8
                            curY += (b1 << 24) >> 24;
                            off += 2;
                        }
                        pts[j * 2] = curX * 0.1;
                        pts[j * 2 + 1] = curY * 0.1;
                        actualPts++;
                    }
                    polylines.push({
                        cat: cat,
                        typeIdx: ti,
                        points: pts,
                        pointCount: actualPts,
                        z: polyZ
                    });
                }
            }
        }

        // Z-range filter (dual slider)
        var zMin = -500, zMax = 500;
        var hasZ = false;
        if (zValues.length > 0) {
            zMin = Infinity; zMax = -Infinity;
            for (let i = 0; i < zValues.length; i++) {
                if (zValues[i] < zMin) zMin = zValues[i];
                if (zValues[i] > zMax) zMax = zValues[i];
            }
            hasZ = true;
        }
        // Also include polyline Z values
        for (let p = 0; p < polylines.length; p++) {
            var pz = polylines[p].z || 0;
            if (!hasZ) { zMin = pz; zMax = pz; hasZ = true; }
            if (pz < zMin) zMin = pz;
            if (pz > zMax) zMax = pz;
        }
        if (hasZ) {
            // Round to nice values
            zMin = Math.floor(zMin / 5) * 5;
            zMax = Math.ceil(zMax / 5) * 5;
        }

        // Radius filter
        var radius = {
            active: false,
            cx: 0, cy: 0,  // center in world meters
            meters: 500     // radius in meters
        };

        // Camera state: world meters -> screen pixels
        // World bounds: approximately -375000 to 375000 cm = -3750 to 3750 m
        var cam = {
            cx: 0,      // center x in world meters
            cy: 0,      // center y in world meters
            zoom: 0.1   // pixels per meter (start zoomed out)
        };

        // Build collectible index list (cats 12-17) for arrow marker overlay
        var collectibleIndices = [];
        for (let i = 0; i < n; i++) {
            if (cats[i] >= 12 && cats[i] <= 17) {
                collectibleIndices.push(i);
            }
        }

        // Build resource node index list (cat 9) for arrow marker overlay
        var resourceNodeIndices = [];
        // Build purity counts per resource type: typeIdx -> [impure, normal, pure]
        var resourcePurityCounts = {}; // typeIdx -> [count0, count1, count2]
        for (let i = 0; i < n; i++) {
            if (cats[i] === CAT.RESOURCE_NODE) {
                resourceNodeIndices.push(i);
                let pti = types[i];
                if (!resourcePurityCounts[pti]) resourcePurityCounts[pti] = [0, 0, 0];
                var pur = Math.round(yaws[i]);
                if (pur < 0 || pur > 2) pur = 0;
                resourcePurityCounts[pti][pur]++;
            }
        }

        // Build per-category type lists for per-type dropdown UI
        var typesByCat = {}; // cat -> {typeIdx -> count}
        for (let i = 0; i < n; i++) {
            var c = cats[i];
            if (!typesByCat[c]) typesByCat[c] = {};
            let ti = types[i];
            typesByCat[c][ti] = (typesByCat[c][ti] || 0) + 1;
        }
        // Include polyline types
        for (let p = 0; p < polylines.length; p++) {
            var pc = polylines[p].cat;
            if (!typesByCat[pc]) typesByCat[pc] = {};
            let pti = polylines[p].typeIdx;
            typesByCat[pc][pti] = (typesByCat[pc][pti] || 0) + 1;
        }
        // Convert to sorted arrays of {typeIdx, count}
        var typesByCatList = {}; // cat -> [{typeIdx, count}]
        for (var c2 in typesByCat) {
            var arr = [];
            for (var ti2 in typesByCat[c2]) {
                arr.push({ typeIdx: parseInt(ti2), count: typesByCat[c2][ti2] });
            }
            arr.sort(function(a, b) { return b.count - a.count; });
            typesByCatList[c2] = arr;
        }

        // Per-type visibility (all visible by default). Must be Int32Array to match
        // shader uniform u_typeVisible and gl.uniform1iv requirements.
        // Sized to the actual type count (heavily modded saves exceed the old fixed
        // 512); the shader's array length is derived from this at program build time.
        var typeVisible = new Int32Array(typeVisibleSize(typeInfo.length));
        for (let i = 0; i < typeVisible.length; i++) typeVisible[i] = 1;
        // For categories hidden by default, start with all types hidden too.
        // Skip collectibles (12-17) — they don't have per-type dropdowns, so their
        // types must stay visible and rely on the category checkbox alone.
        for (var hc in typesByCat) {
            var hcNum = parseInt(hc);
            if (DEFAULT_CAT_VISIBLE[hcNum] === false && (hcNum < 12 || hcNum > 17)) {
                for (var ht in typesByCat[hc]) {
                    typeVisible[parseInt(ht)] = 0;
                }
            }
        }

        // Purity visibility for resource nodes (0=impure, 1=normal, 2=pure). All visible by default.
        var purityVisible = [true, true, true];

        // Category visibility (copy defaults, track if user-touched)
        var catVisible = {};
        var catTouched = {};
        for (let k in DEFAULT_CAT_VISIBLE) {
            catVisible[k] = DEFAULT_CAT_VISIBLE[k];
            catTouched[k] = false;
        }

        return {
            mapImgSrc: mapImgSrc,
            mapImg: null,
            positions: positions,
            zValues: zValues,
            cats: cats,
            types: types,
            yaws: yaws,
            halfLenOvr: halfLenOvr,
            halfDepthOvr: halfDepthOvr,
            typeInfo: typeInfo,
            polylines: polylines,
            count: n,
            minerResources: md.minerResources || {},
            buildingInfo: md.buildingInfo || {},
            cam: cam,
            catVisible: catVisible,
            catTouched: catTouched,
            collectibleIndices: collectibleIndices,
            resourceNodeIndices: resourceNodeIndices,
            resourcePurityCounts: resourcePurityCounts,
            typesByCat: typesByCatList,
            typeVisible: typeVisible,
            purityVisible: purityVisible,
            zRangeMin: zMin,
            zRangeMax: zMax,
            zFilterMin: zMin,
            zFilterMax: zMax,
            zEnabled: false,
            radius: radius,
            canvas: null,
            gl: null,
            glObjProgram: null,
            glBuffers: null,
            mapTexture: null,
            dragging: false,
            lastX: 0,
            lastY: 0,
            mouseWorld: null,
            hoveredIdx: -1,
            hoveredPoly: -1,
            mouseX: 0,
            mouseY: 0,
            tooltipsEnabled: true,
            foundationAlpha: 0.5,
            highlightX: 0,
            highlightY: 0,
            highlightTime: 0,
            highlightIdx: -1,
            pinnedIdx: -1,
            pinnedPoly: -1,
            dragStartX: 0,
            dragStartY: 0,
            measure: {
                active: false,
                points: [],      // array of [wx, wy] in world meters
                segments: [],    // array of [pointIdxA, pointIdxB]
                activePoint: -1, // index of point to continue from (-1 = none)
                hoverX: 0,       // current mouse world x for rubber-band
                hoverY: 0        // current mouse world y for rubber-band
            }
        };
    }

    // Save group and per-type expand/collapse state from the current DOM so it
    // can be restored after a language switch rebuilds the controls HTML.
    function _saveExpandState(state) {
        var parent = state.canvas ? state.canvas.parentElement : null;
        if (!parent) return;
        // Group expand state
        state._groupExpandState = {};
        parent.querySelectorAll('.map-group-children').forEach(function(el) {
            var gi = el.dataset.group;
            state._groupExpandState[gi] = (el.style.display !== 'none');
        });
        // Per-type expand state
        state._typeExpandState = {};
        parent.querySelectorAll('.map-type-children').forEach(function(el) {
            var c = el.dataset.cat;
            state._typeExpandState[c] = (el.style.display !== 'none');
        });
    }

    // Renders the controls sidebar for the given map state. Takes the state
    // explicitly rather than reaching for the module-level mapState, matching
    // setupMapEvents and the render functions.
    function buildMapControls(state) {
        var html = '';
        html += '<div style="font-weight:bold;margin-bottom:6px;color:#eee;">' + t('map.title') + '</div>';

        // Category toggles — grouped with expand/collapse + "All" toggle
        var catGroups = [
            { label: 'map.group.production', cats: [CAT.BUILDING, CAT.MINER] },
            { label: 'map.group.power', cats: [CAT.POWER_GEN, CAT.POWER_STORAGE, CAT.POWER_POLE] },
            { label: 'map.group.transport', cats: [CAT.BELT, CAT.PIPE, CAT.RAIL, CAT.HYPERTUBE, CAT.SIGNAL] },
            { label: 'map.group.stations', cats: [CAT.STATION] },
            { label: 'map.group.special', cats: [CAT.SPECIAL] },
            { label: 'map.group.build', cats: [CAT.STRUCTURE, CAT.FOUNDATION] },
            { label: 'map.group.resource_nodes', cats: [CAT.RESOURCE_NODE] },
            { label: 'map.group.collectibles', cats: [CAT.BLUE_SLUG, CAT.YELLOW_SLUG, CAT.PURPLE_SLUG, CAT.SOMERSLOOP, CAT.MERCER_SPHERE, CAT.CRASH_SITE] }
        ];
        for (let gi = 0; gi < catGroups.length; gi++) {
            var grp = catGroups[gi];
            var allChecked = true;
            for (let ci = 0; ci < grp.cats.length; ci++) {
                if (!state.catVisible[grp.cats[ci]]) { allChecked = false; break; }
            }
            // Resource Nodes and Collectibles groups start collapsed by default,
            // but restore saved expand state if available (language switch).
            var collapsed;
            if (state._groupExpandState && state._groupExpandState[String(gi)] !== undefined) {
                collapsed = !state._groupExpandState[String(gi)];
            } else {
                collapsed = (grp.label === 'map.group.resource_nodes' || grp.label === 'map.group.collectibles');
            }
            html += '<div class="map-cat-group" style="margin-bottom:2px;">';
            html += '<div style="display:flex;align-items:center;gap:4px;padding:2px 0;">';
            html += '<span class="map-group-arrow" data-group="' + gi + '" style="cursor:pointer;color:#888;font-size:10px;width:12px;user-select:none;">' + (collapsed ? '▶' : '▼') + '</span>';
            html += '<input type="checkbox" class="map-group-all" data-group="' + gi + '" ' + (allChecked ? 'checked' : '') + '>';
            html += '<span style="color:#ccc;font-size:12px;font-weight:bold;">' + t(grp.label) + '</span>';
            html += '</div>';
            html += '<div class="map-group-children" data-group="' + gi + '" style="display:' + (collapsed ? 'none' : 'block') + ';padding-left:18px;">';
            for (let ci = 0; ci < grp.cats.length; ci++) {
                var c = grp.cats[ci];
                var color = _getCatColorHex(c);
                var checked = state.catVisible[c] ? 'checked' : '';
                var isCollectible = (c >= 12 && c <= 17);
                var catTypes = state.typesByCat[c];
                var hasTypes = catTypes && catTypes.length > 1 && !isCollectible;

                html += '<div class="map-cat-row" data-cat="' + c + '" style="margin-bottom:2px;">';
                // Category row: arrow on left (if has types) + checkbox + swatch + name
                html += '<div style="display:flex;align-items:center;gap:4px;padding:1px 0;">';
                if (hasTypes) {
                    var typeArrowChar = (state._typeExpandState && state._typeExpandState[String(c)]) ? '▼' : '▶';
                    html += '<span class="map-type-arrow" data-cat="' + c + '" style="cursor:pointer;color:#888;font-size:10px;width:12px;user-select:none;">' + typeArrowChar + '</span>';
                } else {
                    html += '<span style="width:12px;flex-shrink:0;"></span>';
                }
                html += '<input type="checkbox" data-cat="' + c + '" ' + checked + '>';
                html += '<span style="display:inline-block;width:10px;height:10px;background:' + color + ';border-radius:2px;"></span>';
                html += '<span style="color:#aaa;font-size:11px;">' + catName(c) + '</span>';
                if (c === CAT.FOUNDATION) {
                    var faPct = Math.round((state.foundationAlpha != null ? state.foundationAlpha : 0.5) * 100);
                    html += '<input type="range" id="mapFoundationAlpha" min="0" max="100" value="' + faPct + '" step="5" style="flex:1;min-width:30px;margin-left:6px;" title="' + t('map.foundation_opacity') + '">';
                    html += '<span id="mapFoundationAlphaVal" style="color:#888;font-size:10px;min-width:30px;text-align:right;">' + faPct + '%</span>';
                }
                html += '</div>';

                // Per-type dropdown (not for collectibles — they already have individual categories)
                if (hasTypes) {
                    var typeExpanded = state._typeExpandState && state._typeExpandState[String(c)];
                    html += '<div class="map-type-children" data-cat="' + c + '" style="display:' + (typeExpanded ? 'block' : 'none') + ';padding-left:18px;padding-top:1px;">';
                    // Per-type checkboxes
                    for (let ti = 0; ti < catTypes.length; ti++) {
                        var entry = catTypes[ti];
                        var typeChecked = isTypeVisible(state, entry.typeIdx) ? 'checked' : '';
                        var typeColor, typeLabel;
                        var isResourceNode = (c === CAT.RESOURCE_NODE);
                        if (isResourceNode) {
                            var info = state.typeInfo[entry.typeIdx];
                            var path = info ? info.path : '';
                            typeColor = resourceTypeColor(path);
                            typeLabel = resourceTypeName(path);
                        } else {
                            typeColor = color; // same category color for all types
                            typeLabel = mapTypeName(state, entry.typeIdx);
                        }
                        // Truncate long labels
                        if (typeLabel.length > 28) typeLabel = typeLabel.substring(0, 25) + '...';
                        // Resource node types get an expandable arrow for purity sub-checkboxes
                        if (isResourceNode) {
                            html += '<div class="map-rn-type-row" data-type="' + entry.typeIdx + '" style="margin-bottom:1px;">';
                            html += '<div style="display:flex;align-items:center;gap:3px;padding:1px 0;">';
                            html += '<span class="map-purity-arrow" data-type="' + entry.typeIdx + '" style="cursor:pointer;color:#888;font-size:9px;width:10px;user-select:none;">▶</span>';
                            html += '<input type="checkbox" class="map-type-cb" data-cat="' + c + '" data-type="' + entry.typeIdx + '" ' + typeChecked + '>';
                            html += '<span style="display:inline-block;width:8px;height:8px;background:' + typeColor + ';border-radius:2px;flex-shrink:0;"></span>';
                            html += '<span style="color:#999;font-size:10px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;">' + typeLabel + '</span>';
                            html += '<span style="color:#666;font-size:9px;">(' + entry.count + ')</span>';
                            html += '</div>';
                            // Purity sub-checkboxes
                            var purCounts = state.resourcePurityCounts[entry.typeIdx] || [0, 0, 0];
                            html += '<div class="map-purity-children" data-type="' + entry.typeIdx + '" style="display:none;padding-left:22px;">';
                            for (var pi = 0; pi < 3; pi++) {
                                var purChecked = state.purityVisible[pi] ? 'checked' : '';
                                html += '<label style="display:flex;align-items:center;gap:3px;padding:1px 0;">';
                                html += '<input type="checkbox" class="map-purity-cb" data-purity="' + pi + '" ' + purChecked + '>';
                                html += '<span style="color:#777;font-size:9px;">' + t(PURITY_KEYS[pi]) + '</span>';
                                html += '<span style="color:#555;font-size:8px;">(' + purCounts[pi] + ')</span>';
                                html += '</label>';
                            }
                            html += '</div>';
                            html += '</div>';
                        } else {
                            html += '<label style="display:flex;align-items:center;gap:3px;padding:1px 0;">';
                            html += '<span style="width:10px;flex-shrink:0;"></span>';
                            html += '<input type="checkbox" class="map-type-cb" data-cat="' + c + '" data-type="' + entry.typeIdx + '" ' + typeChecked + '>';
                            html += '<span style="display:inline-block;width:8px;height:8px;background:' + typeColor + ';border-radius:2px;flex-shrink:0;"></span>';
                            html += '<span style="color:#999;font-size:10px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;">' + typeLabel + '</span>';
                            html += '<span style="color:#666;font-size:9px;">(' + entry.count + ')</span>';
                            html += '</label>';
                        }
                    }
                    html += '</div>';
                }

                html += '</div>';
            }
            html += '</div>';
            html += '</div>';
        }

        // Z-range slider
        html += '<div style="margin-top:8px;border-top:1px solid #333;padding-top:6px;">';
        html += '<div style="font-weight:bold;margin-bottom:4px;color:#eee;">' + t('map.z_range') + '</div>';
        html += '<label style="display:flex;align-items:center;gap:4px;color:#ccc;font-size:12px;">';
        html += '<input type="checkbox" id="mapZEnabled" ' + (state.zEnabled ? 'checked' : '') + '> ' + t('map.enable');
        html += '</label>';
        html += '<div style="display:flex;align-items:center;gap:2px;margin-top:4px;">';
        html += '<span style="color:#888;font-size:11px;min-width:24px;">' + t('map.min') + '</span>';
        html += '<button id="mapZMinDown" style="width:auto;padding:0 3px;font-size:10px;min-width:16px;cursor:pointer;line-height:1.2;">&minus;</button>';
        html += '<input type="range" id="mapZMin" min="' + state.zRangeMin + '" max="' + state.zRangeMax + '" value="' + state.zFilterMin + '" step="1" style="flex:1;min-width:40px;">';
        html += '<button id="mapZMinUp" style="width:auto;padding:0 3px;font-size:10px;min-width:16px;cursor:pointer;line-height:1.2;">+</button>';
        html += '<span id="mapZMinVal" style="color:#aaa;font-size:11px;min-width:32px;text-align:right;">' + state.zFilterMin + '</span>';
        html += '</div>';
        html += '<div style="display:flex;align-items:center;gap:2px;margin-top:2px;">';
        html += '<span style="color:#888;font-size:11px;min-width:24px;">' + t('map.max') + '</span>';
        html += '<button id="mapZMaxDown" style="width:auto;padding:0 3px;font-size:10px;min-width:16px;cursor:pointer;line-height:1.2;">&minus;</button>';
        html += '<input type="range" id="mapZMax" min="' + state.zRangeMin + '" max="' + state.zRangeMax + '" value="' + state.zFilterMax + '" step="1" style="flex:1;min-width:40px;">';
        html += '<button id="mapZMaxUp" style="width:auto;padding:0 3px;font-size:10px;min-width:16px;cursor:pointer;line-height:1.2;">+</button>';
        html += '<span id="mapZMaxVal" style="color:#aaa;font-size:11px;min-width:32px;text-align:right;">' + state.zFilterMax + '</span>';
        html += '</div>';
        html += '</div>';

        // Radius filter
        html += '<div style="margin-top:8px;border-top:1px solid #333;padding-top:6px;">';
        html += '<div style="font-weight:bold;margin-bottom:4px;color:#eee;">' + t('map.radius_filter') + '</div>';
        html += '<label style="display:flex;align-items:center;gap:4px;color:#ccc;font-size:12px;">';
        html += '<input type="checkbox" id="mapRadiusEnabled" ' + (state.radius.active ? 'checked' : '') + '> ' + t('map.enable');
        html += '</label>';
        html += '<div style="display:flex;align-items:center;gap:4px;margin-top:4px;">';
        html += '<span style="color:#888;font-size:11px;min-width:24px;">R</span>';
        html += '<input type="range" id="mapRadius" min="50" max="5000" value="' + state.radius.meters + '" step="50" style="flex:1;min-width:60px;">';
        html += '<span id="mapRadiusVal" style="color:#aaa;font-size:11px;min-width:48px;text-align:right;">' + state.radius.meters + 'm</span>';
        html += '</div>';
        html += '<div style="color:#888;font-size:10px;margin-top:2px;">' + t('map.shift_click') + '</div>';
        html += '</div>';

        // Object count
        html += '<div id="mapObjCount" style="margin-top:6px;color:#888;font-size:11px;"></div>';

        // Reset button
        html += '<button id="mapReset">' + t('map.reset_view') + '</button>';

        // Tooltip toggle
        html += '<label style="display:flex;align-items:center;gap:4px;color:#ccc;font-size:12px;margin-top:6px;border-top:1px solid #333;padding-top:6px;">';
        html += '<input type="checkbox" id="mapTooltips" ' + (state.tooltipsEnabled ? 'checked' : '') + '>';
        html += t('map.tooltips');
        html += '</label>';

        // Measure tool
        html += '<div style="margin-top:6px;border-top:1px solid #333;padding-top:6px;">';
        html += '<label style="display:flex;align-items:center;gap:4px;color:#ccc;font-size:12px;">';
        html += '<input type="checkbox" id="mapMeasure" ' + (state.measure && state.measure.active ? 'checked' : '') + '>';
        html += t('map.measure');
        html += '</label>';
        html += '<div style="color:#888;font-size:10px;margin-top:2px;">' + t('map.measure_hint') + '</div>';
        html += '<div id="mapMeasureTotal" style="color:#4a9eff;font-size:11px;font-weight:bold;margin-top:2px;"></div>';
        html += '</div>';

        return html;
    }

    // Map pixel -> world meters (inverse)
    function mapPixelToWorld(px, py) {
        var adjX = px / SCALE_TO_HIGHRES + CROP_LO;
        var adjY = py / SCALE_TO_HIGHRES + CROP_LO;
        var x = (adjX * OLD_MAP_DESCALE - WORLD_OFFSET_X) * WORLD_TO_PIXEL_SCALE / 100;
        var y = (adjY * OLD_MAP_DESCALE - WORLD_OFFSET_Y) * WORLD_TO_PIXEL_SCALE / 100;
        return [x, y];
    }

    // Returns effective half-width (meters) for an object, using per-instance override if present.
    // For beams, the override replaces halfWidth (the beam's long axis).
    function getEffHw(state, i) {
        var ovr = state.halfLenOvr[i];
        if (ovr > 0) return ovr * 0.01; // 1cm units -> meters
        var info = state.typeInfo[state.types[i]];
        return (info && info.hw > 0) ? info.hw : 0;
    }

    // Returns effective half-depth (meters) for an object, using per-instance override if present.
    function getEffHd(state, i) {
        var ovr = state.halfDepthOvr[i];
        if (ovr > 0) return ovr * 0.01; // 1cm units -> meters
        var info = state.typeInfo[state.types[i]];
        return (info && info.hd > 0) ? info.hd : 0;
    }

    var MAX_DPR = 1.0;

    function getCanvasDPR(state) {
        var dpr = state.canvas.width / state.canvas.clientWidth;
        return isFinite(dpr) && dpr > 0 ? dpr : 1;
    }

    function resizeMapCanvas(state) {
        var canvas = state.canvas;
        var dpr = Math.min(window.devicePixelRatio || 1, MAX_DPR);
        var w = canvas.clientWidth;
        var h = canvas.clientHeight;
        canvas.width = Math.round(w * dpr);
        canvas.height = Math.round(h * dpr);
        if (state.gl) {
            state.gl.viewport(0, 0, canvas.width, canvas.height);
        }
        // Resize overlay canvas to match
        if (state.overlay) {
            state.overlay.width = Math.round(w * dpr);
            state.overlay.height = Math.round(h * dpr);
        }
    }
