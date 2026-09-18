    function setupMapEvents(state) {
        var canvas = state.canvas;

        // Remove old window-level listeners to prevent accumulation on re-render
        if (state._winListeners) {
            window.removeEventListener('mouseup', state._winListeners.mouseup);
            window.removeEventListener('mousemove', state._winListeners.mousemove);
            window.removeEventListener('keydown', state._winListeners.keydown);
            window.removeEventListener('mouseup', state._winListeners.mouseBack, true);
        }
        if (state._resizeObserver) {
            state._resizeObserver.disconnect();
        }

        // Pan
        canvas.addEventListener('mousedown', function(e) {
            state.dragging = true;
            state.lastX = e.clientX;
            state.lastY = e.clientY;
            state.dragStartX = e.clientX;
            state.dragStartY = e.clientY;
            canvas.style.cursor = 'grabbing';
        });

        // Mouse back button — listen on window with capture phase so we intercept
        // before the browser handles it. Firefox swallows mousedown for buttons 3/4
        // but mouseup with capture works across browsers.
        var onMouseBack = function(e) {
            if (e.button === 3) {
                e.preventDefault();
                e.stopPropagation();
                if (typeof previousTab !== 'undefined' && previousTab && previousTab !== 'map') {
                    switchTab(previousTab);
                }
            } else if (e.button === 4) {
                e.preventDefault();
                e.stopPropagation();
            }
        };
        window.addEventListener('mouseup', onMouseBack, true);

        var onMouseUp = function() {
            state.dragging = false;
            canvas.style.cursor = 'grab';
        };
        var onMouseMove = function(e) {
            var dpr = getCanvasDPR(state);
            if (state.dragging) {
                var dx = e.clientX - state.lastX;
                var dy = e.clientY - state.lastY;
                // Convert screen pixels to world meters
                var worldDx = dx / dpr / state.cam.zoom;
                var worldDy = dy / dpr / state.cam.zoom;
                state.cam.cx -= worldDx;
                state.cam.cy -= worldDy;
                state.lastX = e.clientX;
                state.lastY = e.clientY;
                renderMapFrame(state);
            } else {
                // Skip hover detection when the mouse is over the controls sidebar
                var target = e.target;
                if (target && target.closest && target.closest('.map-controls')) {
                    if (state.hoveredIdx >= 0 || state.hoveredPoly >= 0) {
                        state.hoveredIdx = -1;
                        state.hoveredPoly = -1;
                        drawOverlay(state);
                        var tooltip = document.getElementById('mapTooltip');
                        if (tooltip && state.pinnedIdx < 0 && state.pinnedPoly < 0) {
                            tooltip.style.display = 'none';
                        }
                    }
                    return;
                }
                // Skip hover detection when mouse is outside the canvas (e.g. over tab bar)
                var rect = canvas.getBoundingClientRect();
                if (e.clientX < rect.left || e.clientX > rect.right ||
                    e.clientY < rect.top || e.clientY > rect.bottom) {
                    if (state.hoveredIdx >= 0 || state.hoveredPoly >= 0) {
                        state.hoveredIdx = -1;
                        state.hoveredPoly = -1;
                        drawOverlay(state);
                        tooltip = document.getElementById('mapTooltip');
                        if (tooltip && state.pinnedIdx < 0 && state.pinnedPoly < 0) {
                            tooltip.style.display = 'none';
                        }
                    }
                    return;
                }
                // Hover detection
                var sx = (e.clientX - rect.left) * dpr;
                var sy = (e.clientY - rect.top) * dpr;
                var prevHover = state.hoveredIdx;
                var prevPoly = state.hoveredPoly;
                updateHover(state, sx, sy);
                // Redraw overlay + tooltip if hover target changed
                if (prevHover !== state.hoveredIdx || prevPoly !== state.hoveredPoly) {
                    drawOverlay(state);
                }
                // Track mouse world position for measure rubber-band line
                if (state.measure.active && state.measure.activePoint >= 0) {
                    var mw = screenToWorld(state, sx, sy);
                    state.measure.hoverX = mw[0];
                    state.measure.hoverY = mw[1];
                    drawOverlay(state);
                }
            }
        };

        window.addEventListener('mouseup', onMouseUp);
        window.addEventListener('mousemove', onMouseMove);

        // Zoom
        canvas.addEventListener('wheel', function(e) {
            e.preventDefault();
            var rect = canvas.getBoundingClientRect();
            var dpr = getCanvasDPR(state);
            var sx = (e.clientX - rect.left) * dpr;
            var sy = (e.clientY - rect.top) * dpr;

            // World point under mouse before zoom
            var before = screenToWorld(state, sx, sy);
            var factor = e.deltaY < 0 ? 1.15 : 1 / 1.15;
            state.cam.zoom *= factor;
            state.cam.zoom = Math.max(0.01, Math.min(50, state.cam.zoom));

            // Keep mouse point stable
            var after = screenToWorld(state, sx, sy);
            state.cam.cx += before[0] - after[0];
            state.cam.cy += before[1] - after[1];

            renderMapFrame(state);
        }, { passive: false });

        // Touch support: single-finger pan, two-finger pinch-zoom
        var touchState = { dragging: false, lastX: 0, lastY: 0, pinchDist: 0, pinchZoom: 0, pinchMidX: 0, pinchMidY: 0 };
        canvas.addEventListener('touchstart', function(e) {
            e.preventDefault();
            if (e.touches.length === 1) {
                touchState.dragging = true;
                touchState.lastX = e.touches[0].clientX;
                touchState.lastY = e.touches[0].clientY;
            } else if (e.touches.length === 2) {
                touchState.dragging = false;
                let dx = e.touches[0].clientX - e.touches[1].clientX;
                let dy = e.touches[0].clientY - e.touches[1].clientY;
                touchState.pinchDist = Math.hypot(dx, dy);
                touchState.pinchZoom = state.cam.zoom;
                // Store midpoint in screen coords for anchoring
                var rect = canvas.getBoundingClientRect();
                touchState.pinchMidX = ((e.touches[0].clientX + e.touches[1].clientX) / 2 - rect.left);
                touchState.pinchMidY = ((e.touches[0].clientY + e.touches[1].clientY) / 2 - rect.top);
            }
        }, { passive: false });
        canvas.addEventListener('touchmove', function(e) {
            e.preventDefault();
            if (e.touches.length === 1 && touchState.dragging) {
                let dpr = getCanvasDPR(state);
                let dx = e.touches[0].clientX - touchState.lastX;
                let dy = e.touches[0].clientY - touchState.lastY;
                state.cam.cx -= dx / dpr / state.cam.zoom;
                state.cam.cy -= dy / dpr / state.cam.zoom;
                touchState.lastX = e.touches[0].clientX;
                touchState.lastY = e.touches[0].clientY;
                renderMapFrame(state);
            } else if (e.touches.length === 2) {
                let dx = e.touches[0].clientX - e.touches[1].clientX;
                let dy = e.touches[0].clientY - e.touches[1].clientY;
                let dist = Math.hypot(dx, dy);
                if (touchState.pinchDist > 0) {
                    var dpr = getCanvasDPR(state);
                    var sx = touchState.pinchMidX * dpr;
                    var sy = touchState.pinchMidY * dpr;
                    // World point under pinch midpoint before zoom
                    var before = screenToWorld(state, sx, sy);
                    state.cam.zoom = Math.max(0.01, Math.min(50,
                        touchState.pinchZoom * (dist / touchState.pinchDist)));
                    // Keep pinch midpoint stable
                    var after = screenToWorld(state, sx, sy);
                    state.cam.cx += before[0] - after[0];
                    state.cam.cy += before[1] - after[1];
                    renderMapFrame(state);
                }
            }
        }, { passive: false });
        canvas.addEventListener('touchend', function(e) {
            if (e.touches.length === 0) {
                touchState.dragging = false;
                touchState.pinchDist = 0;
            }
        }, { passive: false });

        // Category toggles
        var parent = canvas.parentElement;

        // Group expand/collapse arrows
        parent.querySelectorAll('.map-group-arrow').forEach(function(arrow) {
            arrow.addEventListener('click', function() {
                var gi = arrow.dataset.group;
                var children = parent.querySelector('.map-group-children[data-group="' + gi + '"]');
                if (children) {
                    if (children.style.display === 'none') {
                        children.style.display = 'block';
                        arrow.textContent = '▼';
                    } else {
                        children.style.display = 'none';
                        arrow.textContent = '▶';
                    }
                }
            });
        });

        // "All" checkbox — toggle all children in group (including per-type checkboxes)
        parent.querySelectorAll('.map-group-all').forEach(function(allCb) {
            allCb.addEventListener('change', function() {
                var gi = allCb.dataset.group;
                var children = parent.querySelectorAll('.map-group-children[data-group="' + gi + '"] > .map-cat-row > div > input[data-cat]');
                children.forEach(function(childCb) {
                    var c = parseInt(childCb.dataset.cat);
                    state.catVisible[c] = allCb.checked;
                    state.catTouched[c] = true;
                    childCb.checked = allCb.checked;
                    // Toggle all types in this category (including single-type
                    // categories that have no rendered per-type checkboxes)
                    var catTypes = state.typesByCat[c];
                    if (catTypes) {
                        for (var ti = 0; ti < catTypes.length; ti++) {
                            state.typeVisible[catTypes[ti].typeIdx] = allCb.checked ? 1 : 0;
                        }
                    }
                    // Sync per-type checkbox UI
                    var typeCbs = parent.querySelectorAll('.map-type-cb[data-cat="' + c + '"]');
                    typeCbs.forEach(function(typeCb) {
                        typeCb.checked = allCb.checked;
                    });
                });
                renderMapFrame(state);
            });
        });

        // Individual category toggles — toggle all types in category + update parent "All" state
        // Use :not(.map-type-cb) to exclude per-type checkboxes which also have data-cat
        parent.querySelectorAll('input[data-cat]:not(.map-type-cb)').forEach(function(cb) {
            cb.addEventListener('change', function() {
                var c = parseInt(cb.dataset.cat);
                state.catVisible[c] = cb.checked;
                state.catTouched[c] = true;
                // Toggle all types in this category (including single-type
                // categories that have no rendered per-type checkboxes)
                var catTypes = state.typesByCat[c];
                if (catTypes) {
                    for (var ti = 0; ti < catTypes.length; ti++) {
                        state.typeVisible[catTypes[ti].typeIdx] = cb.checked ? 1 : 0;
                    }
                }
                // Sync per-type checkbox UI
                var typeCbs = parent.querySelectorAll('.map-type-cb[data-cat="' + c + '"]');
                typeCbs.forEach(function(typeCb) {
                    typeCb.checked = cb.checked;
                });
                // Update parent "All" checkbox
                var groupContainer = cb.closest('.map-cat-group');
                if (groupContainer) {
                    var allCb = groupContainer.querySelector('.map-group-all');
                    var childCbs = groupContainer.querySelectorAll('input[data-cat]:not(.map-type-cb)');
                    var allChecked = true;
                    for (let i = 0; i < childCbs.length; i++) {
                        if (!childCbs[i].checked) { allChecked = false; break; }
                    }
                    if (allCb) allCb.checked = allChecked;
                }
                renderMapFrame(state);
            });
        });

        // Per-type expand/collapse arrows
        parent.querySelectorAll('.map-type-arrow').forEach(function(arrow) {
            arrow.addEventListener('click', function(e) {
                e.preventDefault();
                e.stopPropagation();
                var c = arrow.dataset.cat;
                var children = parent.querySelector('.map-type-children[data-cat="' + c + '"]');
                if (children) {
                    if (children.style.display === 'none') {
                        children.style.display = 'block';
                        arrow.textContent = '▼';
                    } else {
                        children.style.display = 'none';
                        arrow.textContent = '▶';
                    }
                }
            });
        });

        // Individual type checkboxes — toggle single type
        parent.querySelectorAll('.map-type-cb').forEach(function(typeCb) {
            typeCb.addEventListener('change', function() {
                var ti = parseInt(typeCb.dataset.type);
                state.typeVisible[ti] = typeCb.checked ? 1 : 0;
                // If turning a type on, auto-enable the parent category
                if (typeCb.checked) {
                    var c = parseInt(typeCb.dataset.cat);
                    if (!state.catVisible[c]) {
                        state.catVisible[c] = true;
                        state.catTouched[c] = true;
                        var catCb = parent.querySelector('input[data-cat="' + c + '"]:not(.map-type-cb)');
                        if (catCb) catCb.checked = true;
                        // Update parent group "All" checkbox
                        var groupContainer = catCb ? catCb.closest('.map-cat-group') : null;
                        if (groupContainer) {
                            var allCb = groupContainer.querySelector('.map-group-all');
                            var childCbs = groupContainer.querySelectorAll('input[data-cat]:not(.map-type-cb)');
                            var allChecked = true;
                            for (let i = 0; i < childCbs.length; i++) {
                                if (!childCbs[i].checked) { allChecked = false; break; }
                            }
                            if (allCb) allCb.checked = allChecked;
                        }
                    }
                }
                renderMapFrame(state);
            });
        });

        // Purity expand/collapse arrows (resource node types)
        parent.querySelectorAll('.map-purity-arrow').forEach(function(arrow) {
            arrow.addEventListener('click', function(e) {
                e.preventDefault();
                e.stopPropagation();
                var ti = arrow.dataset.type;
                var children = parent.querySelector('.map-purity-children[data-type="' + ti + '"]');
                if (children) {
                    if (children.style.display === 'none') {
                        children.style.display = 'block';
                        arrow.textContent = '▼';
                    } else {
                        children.style.display = 'none';
                        arrow.textContent = '▶';
                    }
                }
            });
        });

        // Purity checkboxes — toggle visibility of a purity level (applies to all resource types)
        parent.querySelectorAll('.map-purity-cb').forEach(function(purCb) {
            purCb.addEventListener('change', function() {
                var pi = parseInt(purCb.dataset.purity);
                state.purityVisible[pi] = purCb.checked;
                // Update all other purity checkboxes with the same purity index
                parent.querySelectorAll('.map-purity-cb[data-purity="' + pi + '"]').forEach(function(cb) {
                    cb.checked = purCb.checked;
                });
                renderMapFrame(state);
            });
        });

        // Z-range sliders
        var zEnabled = parent.querySelector('#mapZEnabled');
        var zMinSlider = parent.querySelector('#mapZMin');
        var zMaxSlider = parent.querySelector('#mapZMax');
        var zMinVal = parent.querySelector('#mapZMinVal');
        var zMaxVal = parent.querySelector('#mapZMaxVal');

        if (zEnabled) {
            zEnabled.addEventListener('change', function() {
                state.zEnabled = zEnabled.checked;
                renderMapFrame(state);
            });
        }
        if (zMinSlider) {
            zMinSlider.addEventListener('input', function() {
                var v = parseInt(zMinSlider.value);
                // Clamping only the state left the thumb sitting past the max thumb.
                if (v > state.zFilterMax) { v = state.zFilterMax; zMinSlider.value = v; }
                state.zFilterMin = v;
                if (zMinVal) zMinVal.textContent = v;
                renderMapFrame(state);
            });
        }
        if (zMaxSlider) {
            zMaxSlider.addEventListener('input', function() {
                var v = parseInt(zMaxSlider.value);
                if (v < state.zFilterMin) { v = state.zFilterMin; zMaxSlider.value = v; }
                state.zFilterMax = v;
                if (zMaxVal) zMaxVal.textContent = v;
                renderMapFrame(state);
            });
        }

        // Z-range +/- buttons (press-and-hold to continuously step)
        function zStep(btnId, sliderId, valId, delta) {
            var btn = parent.querySelector('#' + btnId);
            var slider = parent.querySelector('#' + sliderId);
            var valEl = parent.querySelector('#' + valId);
            if (!btn || !slider) return;

            function applyStep() {
                var v = parseInt(slider.value) + delta;
                var min = parseInt(slider.min);
                var max = parseInt(slider.max);
                if (v < min) v = min;
                if (v > max) v = max;
                if (sliderId === 'mapZMin') {
                    if (v > state.zFilterMax) v = state.zFilterMax;
                    state.zFilterMin = v;
                } else {
                    if (v < state.zFilterMin) v = state.zFilterMin;
                    state.zFilterMax = v;
                }
                // Assign after clamping, otherwise the thumb can drift past the
                // opposite handle while the applied filter stays put.
                slider.value = v;
                if (valEl) valEl.textContent = v;
                renderMapFrame(state);
            }

            var timer = null;
            function start(e) {
                e.preventDefault();
                applyStep();
                timer = setInterval(applyStep, 80);
            }
            function stop() {
                if (timer) { clearInterval(timer); timer = null; }
            }
            btn.addEventListener('mousedown', start);
            btn.addEventListener('mouseup', stop);
            btn.addEventListener('mouseleave', stop);
            btn.addEventListener('touchstart', start, { passive: false });
            btn.addEventListener('touchend', stop);
        }
        zStep('mapZMinDown', 'mapZMin', 'mapZMinVal', -1);
        zStep('mapZMinUp', 'mapZMin', 'mapZMinVal', 1);
        zStep('mapZMaxDown', 'mapZMax', 'mapZMaxVal', -1);
        zStep('mapZMaxUp', 'mapZMax', 'mapZMaxVal', 1);

        // Radius filter
        var rEnabled = parent.querySelector('#mapRadiusEnabled');
        var rSlider = parent.querySelector('#mapRadius');
        var rVal = parent.querySelector('#mapRadiusVal');

        if (rEnabled) {
            rEnabled.addEventListener('change', function() {
                state.radius.active = rEnabled.checked;
                renderMapFrame(state);
            });
        }
        if (rSlider) {
            rSlider.addEventListener('input', function() {
                state.radius.meters = parseInt(rSlider.value);
                if (rVal) rVal.textContent = state.radius.meters + 'm';
                renderMapFrame(state);
            });
        }

        // Click to pin tooltip, Shift+click to set radius center
        canvas.addEventListener('click', function(e) {
            // Skip if this was a drag, not a click
            var dragDist = Math.hypot(e.clientX - state.dragStartX, e.clientY - state.dragStartY);
            if (dragDist > 5) return;

            // Measure mode: click to place points or activate existing points
            if (state.measure.active) {
                var mrect = canvas.getBoundingClientRect();
                var mdpr = getCanvasDPR(state);
                var msx = (e.clientX - mrect.left) * mdpr;
                var msy = (e.clientY - mrect.top) * mdpr;
                var mworld = screenToWorld(state, msx, msy);

                // Check if click hits an existing point (within 10px screen radius)
                var hitIdx = -1;
                var hitTolPx = 10;
                for (var pi = 0; pi < state.measure.points.length; pi++) {
                    var pScreen = worldToScreen(state, state.measure.points[pi][0], state.measure.points[pi][1]);
                    var pdx = pScreen[0] / mdpr - msx / mdpr;
                    var pdy = pScreen[1] / mdpr - msy / mdpr;
                    if (pdx * pdx + pdy * pdy <= hitTolPx * hitTolPx) {
                        hitIdx = pi;
                        break;
                    }
                }

                if (hitIdx >= 0) {
                    if (state.measure.activePoint >= 0 && state.measure.activePoint !== hitIdx) {
                        // Active point exists and clicked a different point: connect them
                        state.measure.segments.push([state.measure.activePoint, hitIdx]);
                        state.measure.activePoint = -1;
                    } else {
                        // No active point (or clicked same point): activate it
                        state.measure.activePoint = hitIdx;
                    }
                } else {
                    // Click on empty space
                    var newIdx = state.measure.points.length;
                    state.measure.points.push([mworld[0], mworld[1]]);
                    if (state.measure.activePoint >= 0) {
                        // Create segment from active point to new point
                        state.measure.segments.push([state.measure.activePoint, newIdx]);
                        state.measure.activePoint = -1; // deactivate
                    } else {
                        // No active point: activate this new point
                        state.measure.activePoint = newIdx;
                    }
                }
                drawOverlay(state);
                _updateMeasureTotal(state);
                return;
            }

            if (!e.shiftKey) {
                // Click to pin/unpin tooltip
                if (state.hoveredIdx >= 0) {
                    state.pinnedIdx = state.hoveredIdx;
                    state.pinnedPoly = -1;
                } else if (state.hoveredPoly >= 0) {
                    state.pinnedPoly = state.hoveredPoly;
                    state.pinnedIdx = -1;
                } else {
                    // Click on empty space: unpin
                    state.pinnedIdx = -1;
                    state.pinnedPoly = -1;
                    var tooltip = document.getElementById('mapTooltip');
                    if (tooltip) tooltip.style.display = 'none';
                }
                if (state.pinnedIdx >= 0 || state.pinnedPoly >= 0) {
                    updateMapTooltip(state);
                }
                drawOverlay(state);
                return;
            }

            // Shift+click: set radius center
            if (!state.radius.active) {
                state.radius.active = true;
                if (rEnabled) rEnabled.checked = true;
            }
            var rect = canvas.getBoundingClientRect();
            var dpr = getCanvasDPR(state);
            var sx = (e.clientX - rect.left) * dpr;
            var sy = (e.clientY - rect.top) * dpr;
            var world = screenToWorld(state, sx, sy);
            state.radius.cx = world[0];
            state.radius.cy = world[1];
            renderMapFrame(state);
        });

        // Right-click: undo last measure point (when measure mode is active)
        canvas.addEventListener('contextmenu', function(e) {
            if (!state.measure.active) return;
            e.preventDefault();
            var m = state.measure;
            if (m.points.length === 0) return;
            // Remove last point
            var removedIdx = m.points.length - 1;
            m.points.pop();
            // Remove segments referencing the removed point
            m.segments = m.segments.filter(function(seg) {
                return seg[0] !== removedIdx && seg[1] !== removedIdx;
            });
            // Fix segment indices that were > removedIdx (shift down by 1)
            for (var si = 0; si < m.segments.length; si++) {
                if (m.segments[si][0] > removedIdx) m.segments[si][0]--;
                if (m.segments[si][1] > removedIdx) m.segments[si][1]--;
            }
            // Deactivate if the active point was removed
            if (m.activePoint === removedIdx) m.activePoint = -1;
            else if (m.activePoint > removedIdx) m.activePoint--;
            drawOverlay(state);
            _updateMeasureTotal(state);
        });

        // Reset button
        var resetBtn = parent.querySelector('#mapReset');
        if (resetBtn) {
            resetBtn.addEventListener('click', function() {
                state.cam.cx = 0;
                state.cam.cy = 0;
                state.cam.zoom = 0.1;
                // Clear pinned tooltip
                state.pinnedIdx = -1;
                state.pinnedPoly = -1;
                var tooltip = document.getElementById('mapTooltip');
                if (tooltip) { tooltip.style.display = 'none'; tooltip.classList.remove('mt-pinned'); }
                // Clear Z-range filter
                state.zEnabled = false;
                state.zFilterMin = state.zRangeMin;
                state.zFilterMax = state.zRangeMax;
                var zEnabledCb = parent.querySelector('#mapZEnabled');
                if (zEnabledCb) zEnabledCb.checked = false;
                var zMinS = parent.querySelector('#mapZMin');
                if (zMinS) zMinS.value = state.zRangeMin;
                var zMaxS = parent.querySelector('#mapZMax');
                if (zMaxS) zMaxS.value = state.zRangeMax;
                var zMinV = parent.querySelector('#mapZMinVal');
                if (zMinV) zMinV.textContent = state.zRangeMin;
                var zMaxV = parent.querySelector('#mapZMaxVal');
                if (zMaxV) zMaxV.textContent = state.zRangeMax;
                // Clear radius filter
                state.radius.active = false;
                state.radius.cx = 0;
                state.radius.cy = 0;
                state.radius.meters = 500;
                var rEnabledCb = parent.querySelector('#mapRadiusEnabled');
                if (rEnabledCb) rEnabledCb.checked = false;
                var rSld = parent.querySelector('#mapRadius');
                if (rSld) rSld.value = 500;
                var rValEl = parent.querySelector('#mapRadiusVal');
                if (rValEl) rValEl.textContent = '500m';
                // Reset category visibility to defaults
                for (var ci = 0; ci < NUM_CATS; ci++) {
                    state.catVisible[ci] = !!DEFAULT_CAT_VISIBLE[ci];
                    state.catTouched[ci] = false;
                    var catCb = parent.querySelector('input[data-cat="' + ci + '"]:not(.map-type-cb)');
                    if (catCb) catCb.checked = !!DEFAULT_CAT_VISIBLE[ci];
                }
                // Reset per-type visibility to all visible
                for (var ti = 0; ti < state.typeVisible.length; ti++) {
                    state.typeVisible[ti] = 1;
                }
                parent.querySelectorAll('.map-type-cb').forEach(function(typeCb) {
                    typeCb.checked = true;
                });
                // Update group "All" checkboxes
                parent.querySelectorAll('.map-group-all').forEach(function(allCb) {
                    var gi = allCb.dataset.group;
                    var childCbs = parent.querySelectorAll('.map-group-children[data-group="' + gi + '"] input[data-cat]:not(.map-type-cb)');
                    var allChecked = true;
                    for (var i = 0; i < childCbs.length; i++) {
                        if (!childCbs[i].checked) { allChecked = false; break; }
                    }
                    allCb.checked = allChecked;
                });
                // Reset group expand/collapse to initial defaults:
                // resource_nodes (gi=6) and collectibles (gi=7) collapsed, rest expanded
                state._groupExpandState = null;
                parent.querySelectorAll('.map-group-arrow').forEach(function(arrow) {
                    var gi = arrow.dataset.group;
                    var children = parent.querySelector('.map-group-children[data-group="' + gi + '"]');
                    var collapsed = (gi === '6' || gi === '7');
                    if (children) children.style.display = collapsed ? 'none' : 'block';
                    arrow.textContent = collapsed ? '▶' : '▼';
                });
                // Collapse all per-type dropdowns
                state._typeExpandState = null;
                parent.querySelectorAll('.map-type-arrow').forEach(function(arrow) {
                    arrow.textContent = '▶';
                });
                parent.querySelectorAll('.map-type-children').forEach(function(el) {
                    el.style.display = 'none';
                });
                // Reset foundation opacity
                state.foundationAlpha = 0.5;
                var faSld = parent.querySelector('#mapFoundationAlpha');
                if (faSld) faSld.value = 50;
                var faV = parent.querySelector('#mapFoundationAlphaVal');
                if (faV) faV.textContent = '50%';
                // Reset measure tool
                state.measure.active = false;
                state.measure.points = [];
                state.measure.segments = [];
                state.measure.activePoint = -1;
                var measureCb = parent.querySelector('#mapMeasure');
                if (measureCb) measureCb.checked = false;
                var measureTotal = parent.querySelector('#mapMeasureTotal');
                if (measureTotal) measureTotal.textContent = '';
                canvas.style.cursor = 'grab';
                renderMapFrame(state);
            });
        }

        // Tooltip toggle
        var tooltipToggle = parent.querySelector('#mapTooltips');
        if (tooltipToggle) {
            tooltipToggle.addEventListener('change', function() {
                state.tooltipsEnabled = tooltipToggle.checked;
                if (!state.tooltipsEnabled) {
                    state.pinnedIdx = -1;
                    state.pinnedPoly = -1;
                    var tooltip = document.getElementById('mapTooltip');
                    if (tooltip) tooltip.style.display = 'none';
                }
            });
        }

        // Measure tool toggle
        var measureToggle = parent.querySelector('#mapMeasure');
        if (measureToggle) {
            measureToggle.addEventListener('change', function() {
                state.measure.active = measureToggle.checked;
                if (!state.measure.active) {
                    state.measure.points = [];
                    state.measure.segments = [];
                    state.measure.activePoint = -1;
                }
                canvas.style.cursor = state.measure.active ? 'crosshair' : 'grab';
                drawOverlay(state);
                _updateMeasureTotal(state);
            });
        }

        // Foundation opacity slider
        var faSlider = parent.querySelector('#mapFoundationAlpha');
        var faVal = parent.querySelector('#mapFoundationAlphaVal');
        if (faSlider) {
            faSlider.addEventListener('input', function() {
                var pct = parseInt(faSlider.value);
                state.foundationAlpha = pct / 100;
                if (faVal) faVal.textContent = pct + '%';
                renderMapFrame(state);
            });
        }

        // Hide tooltip when mouse leaves the canvas (unless pinned)
        canvas.addEventListener('mouseleave', function() {
            state.hoveredIdx = -1;
            state.hoveredPoly = -1;
            if (state.pinnedIdx < 0 && state.pinnedPoly < 0) {
                var tooltip = document.getElementById('mapTooltip');
                if (tooltip) tooltip.style.display = 'none';
            }
            var info = document.getElementById('mapInfo');
            if (info) info.textContent = '';
            drawOverlay(state);
        });

        // Escape key to unpin / clear measure
        var onKeyDown = function(e) {
            if (e.key === 'Escape') {
                if (state.measure.active && state.measure.points.length > 0) {
                    state.measure.points = [];
                    state.measure.segments = [];
                    state.measure.activePoint = -1;
                    drawOverlay(state);
                    _updateMeasureTotal(state);
                } else if (state.pinnedIdx >= 0 || state.pinnedPoly >= 0) {
                    state.pinnedIdx = -1;
                    state.pinnedPoly = -1;
                    var tooltip = document.getElementById('mapTooltip');
                    if (tooltip) tooltip.style.display = 'none';
                    drawOverlay(state);
                }
            }
        };
        window.addEventListener('keydown', onKeyDown);

        // Resize observer
        if (window.ResizeObserver) {
            var ro = new ResizeObserver(function() {
                resizeMapCanvas(state);
                renderMapFrame(state);
            });
            ro.observe(canvas);
            state._resizeObserver = ro;
        }

        // Store window-level listeners for cleanup on re-render
        state._winListeners = {
            mouseup: onMouseUp,
            mousemove: onMouseMove,
            keydown: onKeyDown,
            mouseBack: onMouseBack
        };
    }

    function screenToWorld(state, sx, sy) {
        var dpr = getCanvasDPR(state);
        var w = state.canvas.clientWidth;
        var h = state.canvas.clientHeight;
        var wx = state.cam.cx + (sx / dpr - w / 2) / state.cam.zoom;
        var wy = state.cam.cy + (sy / dpr - h / 2) / state.cam.zoom;
        return [wx, wy];
    }

    function worldToScreen(state, wx, wy) {
        var dpr = getCanvasDPR(state);
        var w = state.canvas.clientWidth;
        var h = state.canvas.clientHeight;
        var sx = (wx - state.cam.cx) * state.cam.zoom + w / 2;
        var sy = (wy - state.cam.cy) * state.cam.zoom + h / 2;
        return [sx * dpr, sy * dpr];
    }

    function isCatVisible(state, cat) {
        return state.catVisible[cat];
    }

    function isTypeVisible(state, typeIdx) {
        if (!state.typeVisible) return true;
        return state.typeVisible[typeIdx] !== 0;
    }

    // Purity is encoded in yaw for resource nodes: 0=impure, 1=normal, 2=pure
    function isPurityVisible(state, yaw) {
        if (!state.purityVisible) return true;
        var p = Math.round(yaw);
        if (p < 0 || p > 2) p = 0;
        return state.purityVisible[p];
    }

    function isZFiltered(state, z) {
        if (!state.zEnabled) return false;
        return z < state.zFilterMin || z > state.zFilterMax;
    }

    function isRadiusFiltered(state, wx, wy) {
        if (!state.radius.active) return false;
        var dx = wx - state.radius.cx;
        var dy = wy - state.radius.cy;
        return (dx * dx + dy * dy) > (state.radius.meters * state.radius.meters);
    }

    // Point-in-rotated-rectangle test. Returns true if (wx,wy) is inside the rect
    // centered at (cx,cy) with half-width hw, half-depth hd, rotated by yawRad.
    function pointInRotatedRect(wx, wy, cx, cy, hw, hd, yawRad) {
        var c = Math.cos(-yawRad), s = Math.sin(-yawRad);
        var lx = (wx - cx) * c - (wy - cy) * s;
        var ly = (wx - cx) * s + (wy - cy) * c;
        return Math.abs(lx) <= hw && Math.abs(ly) <= hd;
    }

    // Distance from point (px,py) to line segment (ax,ay)-(bx,by)
    function pointToSegmentDist2(px, py, ax, ay, bx, by) {
        var dx = bx - ax, dy = by - ay;
        var len2 = dx * dx + dy * dy;
        if (len2 < 1e-9) {
            var ddx = px - ax, ddy = py - ay;
            return ddx * ddx + ddy * ddy;
        }
        var t = ((px - ax) * dx + (py - ay) * dy) / len2;
        if (t < 0) t = 0; else if (t > 1) t = 1;
        var cx = ax + t * dx, cy = ay + t * dy;
        var ex = px - cx, ey = py - cy;
        return ex * ex + ey * ey;
    }

    function updateHover(state, sx, sy) {
        var dpr = getCanvasDPR(state);
        var world = screenToWorld(state, sx, sy);
        var wx = world[0], wy = world[1];
        var pxTol = 12; // 12px tolerance
        var tolerance = pxTol / state.cam.zoom; // in world meters

        var bestIdx = -1;
        var bestDist = Infinity; // track closest hit (rect hits use distance 0)
        var pointTol2 = tolerance * tolerance; // tolerance for point-only objects

        // Use the spatial grid when available (huge perf win on large saves),
        // otherwise fall back to scanning every object. Both paths run the same
        // hit-test; only the source of the candidate index differs, so the loop
        // body lives in one place.
        var candidates = state.grid ? getVisibleObjectIndices(state) : null;
        var nCandidates = candidates ? candidates.length : state.count;
        var arrowTol = 15 / state.cam.zoom;
        var arrowTol2 = arrowTol * arrowTol;
        let i, ox, oy, effHw, effHd, yawRad, dx, dy, d2;
        for (let ci = 0; ci < nCandidates; ci++) {
            i = candidates ? candidates[ci] : ci;
            if (!isCatVisible(state, state.cats[i])) continue;
            if (!isTypeVisible(state, state.types[i])) continue;
            if (state.cats[i] === CAT.RESOURCE_NODE && !isPurityVisible(state, state.yaws[i])) continue;
            if (isZFiltered(state, state.zValues[i])) continue;
            if (isRadiusFiltered(state, state.positions[i * 2], state.positions[i * 2 + 1])) continue;

            ox = state.positions[i * 2];
            oy = state.positions[i * 2 + 1];
            dx = ox - wx; dy = oy - wy;
            d2 = dx * dx + dy * dy;

            // Arrow markers (resource nodes, collectibles) are drawn at a fixed
            // pixel size regardless of zoom, so they get a screen-space tolerance
            // rather than their footprint.
            if (state.cats[i] === CAT.RESOURCE_NODE || (state.cats[i] >= 12 && state.cats[i] <= 17)) {
                if (d2 < arrowTol2 && d2 < bestDist) {
                    bestDist = d2;
                    bestIdx = i;
                }
                continue;
            }

            effHw = getEffHw(state, i);
            effHd = getEffHd(state, i);
            if (effHw > 0 && effHd > 0) {
                // Has a footprint: rotated-rect hit-test, distance to centre
                // used only to break ties between overlapping rects.
                yawRad = state.yaws[i] * 0.1 * Math.PI / 180.0;
                if (pointInRotatedRect(wx, wy, ox, oy, effHw, effHd, yawRad) && d2 < bestDist) {
                    bestDist = d2;
                    bestIdx = i;
                }
            } else if (d2 < pointTol2 && d2 < bestDist) {
                // Point object: plain radius tolerance.
                bestDist = d2;
                bestIdx = i;
            }
        }

        // --- Polyline hover detection (spatially indexed) ---
        var bestPolyIdx = -1;
        var bestPolyDist = tolerance * tolerance;
        if (state.polylines && state.polylines.length > 0) {
            var polyTol = tolerance;
            var g = state.grid;
            if (g && g.polyCells) {
                // Use spatial grid: find cell at mouse position, check those polylines
                var mcx = Math.floor((wx - g.minX) / GRID_CELL_SIZE);
                var mcy = Math.floor((wy - g.minY) / GRID_CELL_SIZE);
                // Also check neighboring cells (polyline may be in adjacent cell)
                var visited = {};
                for (let row = mcy - 1; row <= mcy + 1; row++) {
                    for (let col = mcx - 1; col <= mcx + 1; col++) {
                        if (row < 0 || row >= g.rows || col < 0 || col >= g.cols) continue;
                        let cellPolys = g.polyCells[row * g.cols + col];
                        if (!cellPolys) continue;
                        for (let pi = 0; pi < cellPolys.length; pi++) {
                            let p = cellPolys[pi];
                            if (visited[p]) continue;
                            visited[p] = true;
                            let pl = state.polylines[p];
                            if (!isCatVisible(state, pl.cat)) continue;
                            if (!isTypeVisible(state, pl.typeIdx)) continue;
                            if (isZFiltered(state, pl.z || 0)) continue;
                            for (let j = 0; j < pl.pointCount - 1; j++) {
                                let segMidX = (pl.points[j * 2] + pl.points[(j + 1) * 2]) * 0.5;
                                let segMidY = (pl.points[j * 2 + 1] + pl.points[(j + 1) * 2 + 1]) * 0.5;
                                if (isRadiusFiltered(state, segMidX, segMidY)) continue;
                                d2 = pointToSegmentDist2(wx, wy,
                                    pl.points[j * 2], pl.points[j * 2 + 1],
                                    pl.points[(j + 1) * 2], pl.points[(j + 1) * 2 + 1]);
                                if (d2 < bestPolyDist) {
                                    bestPolyDist = d2;
                                    bestPolyIdx = p;
                                }
                            }
                        }
                    }
                }
            } else {
                // Fallback: linear scan with bbox culling
                for (let p = 0; p < state.polylines.length; p++) {
                    let pl = state.polylines[p];
                    if (!isCatVisible(state, pl.cat)) continue;
                    if (!isTypeVisible(state, pl.typeIdx)) continue;
                    if (isZFiltered(state, pl.z || 0)) continue;
                    let bb = g && g.polyBBoxes ? g.polyBBoxes[p] : null;
                    if (bb) {
                        if (wx < bb.minX - polyTol || wx > bb.maxX + polyTol ||
                            wy < bb.minY - polyTol || wy > bb.maxY + polyTol) continue;
                    }
                    for (let j = 0; j < pl.pointCount - 1; j++) {
                        let segMidX = (pl.points[j * 2] + pl.points[(j + 1) * 2]) * 0.5;
                        let segMidY = (pl.points[j * 2 + 1] + pl.points[(j + 1) * 2 + 1]) * 0.5;
                        if (isRadiusFiltered(state, segMidX, segMidY)) continue;
                        d2 = pointToSegmentDist2(wx, wy,
                            pl.points[j * 2], pl.points[j * 2 + 1],
                            pl.points[(j + 1) * 2], pl.points[(j + 1) * 2 + 1]);
                        if (d2 < bestPolyDist) {
                            bestPolyDist = d2;
                            bestPolyIdx = p;
                        }
                    }
                }
            }
        }

        // Prefer the closer of object vs polyline
        // Rect hits (bestDist < pointTol2) always beat polyline hits
        if (bestPolyIdx >= 0 && bestPolyDist < bestDist) {
            bestIdx = -1; // polyline wins
        } else {
            bestPolyIdx = -1; // object wins
        }

        state.hoveredIdx = bestIdx;
        state.hoveredPoly = bestPolyIdx;
        state.mouseX = sx / dpr; // CSS pixels for tooltip positioning
        state.mouseY = sy / dpr;

        // Update bottom info bar
        var info = document.getElementById('mapInfo');
        if (info) {
            if (bestIdx >= 0) {
                var info_text = mapTypeName(state, state.types[bestIdx]);
                info_text += ' [' + catName(state.cats[bestIdx]) + ']';
                // Show purity for resource nodes
                if (state.cats[bestIdx] === CAT.RESOURCE_NODE) {
                    info_text += ' - ' + purityName(state.yaws[bestIdx]);
                }
                info_text += ' (' + Math.round(state.positions[bestIdx * 2]) + ', ' + Math.round(state.positions[bestIdx * 2 + 1]) + ', ' + Math.round(state.zValues[bestIdx]) + 'm)';
                info.textContent = info_text;
            } else if (bestPolyIdx >= 0) {
                var pl2 = state.polylines[bestPolyIdx];
                var pname = mapTypeName(state, pl2.typeIdx);
                info.textContent = pname + ' [' + catName(pl2.cat) + ']';
            } else {
                info.textContent = 'X: ' + Math.round(wx) + 'm, Y: ' + Math.round(wy) + 'm';
            }
        }

        // Update floating tooltip (if enabled and not pinned — pinned tooltip
        // content is fixed; only repositioned by renderMapFrame on pan/zoom)
        if (state.pinnedIdx >= 0 || state.pinnedPoly >= 0) {
            // Pinned: don't rebuild tooltip, just update info bar
        } else if (state.tooltipsEnabled) {
            updateMapTooltip(state);
        } else {
            var tooltip = document.getElementById('mapTooltip');
            if (tooltip) tooltip.style.display = 'none';
        }
    }

    function updateMapTooltip(state) {
        var tooltip = document.getElementById('mapTooltip');
        if (!tooltip) return;

        // Use pinned object if set, otherwise hovered
        var idx = state.pinnedIdx >= 0 ? state.pinnedIdx : state.hoveredIdx;
        var poly = state.pinnedPoly >= 0 ? state.pinnedPoly : state.hoveredPoly;
        var isPinned = state.pinnedIdx >= 0 || state.pinnedPoly >= 0;

        var html = '';
        if (isPinned) {
            html += '<div class="mt-close" title="Close">&times;</div>';
        }
        if (idx >= 0) {
            var i = idx;
            var ti = state.types[i];
            var cat = state.cats[i];
            // Collectibles (cats 12-17) use synthetic type names that duplicate
            // the category label in English — skip the header, show only the
            // translated category line.
            // Resource nodes: show type name in uppercase, skip redundant category line.
            if (cat === CAT.RESOURCE_NODE) {
                var rnName = mapTypeName(state, ti);
                var rnInfo = state.typeInfo[ti];
                var rnPath = rnInfo ? rnInfo.path : '';
                var rnColor = resourceTypeColor(rnPath);
                html += '<div class="mt-name" style="text-transform:uppercase;color:' + rnColor + ';">' + esc(rnName) + '</div>';
            } else if (cat < 12 || cat > 17) {
                // Custom names (train stations) replace the generic type name
                var customName = state.buildingInfo[i] && state.buildingInfo[i].name;
                var name = customName || mapTypeName(state, ti);
                html += '<div class="mt-name">' + esc(name) + '</div>';
                html += '<div class="mt-cat" style="color:' + _getCatColorHex(cat) + '">' + catName(cat) + '</div>';
            } else {
                html += '<div class="mt-cat" style="color:' + _getCatColorHex(cat) + '">' + catName(cat) + '</div>';
            }
            // Show purity for resource nodes
            if (cat === CAT.RESOURCE_NODE) {
                html += '<div class="mt-row"><span>' + t('map.purity') + '</span><span>' + purityName(state.yaws[i]) + '</span></div>';
            }
            // Show extracted resource for miners
            if (cat === CAT.MINER && state.minerResources[i]) {
                var resName = displayName(state.minerResources[i]);
                html += '<div class="mt-row"><span>' + t('map.resource') + '</span><span style="color:#ffd700;font-weight:bold;">' + esc(resName) + '</span></div>';
            }
            // Show building info (recipe, status, efficiency, fuel) for production buildings, miners, and generators
            if (state.buildingInfo[i]) {
                var bi = state.buildingInfo[i];
                if (bi.recipe) {
                    html += '<div class="mt-row"><span>' + t('map.recipe') + '</span><span>' + esc(recipeName(bi.recipe)) + '</span></div>';
                }
                if (bi.fuel) {
                    html += '<div class="mt-row"><span>' + t('map.fuel') + '</span><span>' + esc(displayName(bi.fuel)) + '</span></div>';
                }
                if (bi.status) {
                    var statusColor = bi.status === 'Running' ? '#4caf50' : (bi.status === 'Paused' ? '#f44336' : '#ff9800');
                    html += '<div class="mt-row"><span>' + t('map.status') + '</span><span style="color:' + statusColor + ';font-weight:bold;">' + t('map.status.' + bi.status.toLowerCase()) + '</span></div>';
                }
                if (bi.hasEff) {
                    var eff = bi.efficiency || 0;
                    var effColor = eff >= 95 ? '#4caf50' : (eff >= 50 ? '#ff9800' : '#f44336');
                    html += '<div class="mt-row"><span>' + t('map.efficiency') + '</span><span style="color:' + effColor + ';font-weight:bold;">' + eff.toFixed(1) + '%</span></div>';
                }
            }
            html += '<div class="mt-row"><span>' + t('map.position') + '</span><span>' +
                'X: ' + Math.round(state.positions[i * 2]) + '  Y: ' + Math.round(state.positions[i * 2 + 1]) + '  Z: ' + Math.round(state.zValues[i]) + 'm \u2191</span></div>';
            if (cat < 12 || cat > 17) {
                var effHw = getEffHw(state, i);
                var effHd = getEffHd(state, i);
                if (effHw > 0 && effHd > 0 && cat !== CAT.RESOURCE_NODE) {
                    html += '<div class="mt-row"><span>' + t('map.size') + '</span><span>' +
                        (effHw * 2).toFixed(1) + 'm × ' + (effHd * 2).toFixed(1) + 'm</span></div>';
                }
                // Skip heading for resource nodes (yaw encodes purity, not rotation)
                if (cat !== CAT.RESOURCE_NODE) {
                    var yawDeg = state.yaws[i] * 0.1;
                    html += '<div class="mt-row"><span>' + t('map.heading') + '</span><span>' + yawDeg.toFixed(0) + '°</span></div>';
                }
            }
        } else if (poly >= 0) {
            var pl = state.polylines[poly];
            var pname = mapTypeName(state, pl.typeIdx);
            html += '<div class="mt-name">' + esc(pname) + '</div>';
            html += '<div class="mt-cat" style="color:' + _getCatColorHex(pl.cat) + '">' + catName(pl.cat) + '</div>';
            html += '<div class="mt-row"><span>' + t('map.points') + '</span><span>' + pl.pointCount + '</span></div>';
            // Start and end coords
            var sx0 = pl.points[0], sy0 = pl.points[1];
            var ex0 = pl.points[(pl.pointCount - 1) * 2], ey0 = pl.points[(pl.pointCount - 1) * 2 + 1];
            html += '<div class="mt-row"><span>' + t('map.from') + '</span><span>' +
                Math.round(sx0) + ', ' + Math.round(sy0) + 'm</span></div>';
            html += '<div class="mt-row"><span>' + t('map.to') + '</span><span>' +
                Math.round(ex0) + ', ' + Math.round(ey0) + 'm</span></div>';
        }

        if (html) {
            tooltip.innerHTML = html;
            tooltip.style.display = 'block';

            // Wire up close button on pinned tooltip
            if (isPinned) {
                var closeBtn = tooltip.querySelector('.mt-close');
                if (closeBtn) {
                    closeBtn.addEventListener('click', function() {
                        state.pinnedIdx = -1;
                        state.pinnedPoly = -1;
                        tooltip.style.display = 'none';
                        tooltip.classList.remove('mt-pinned');
                        drawOverlay(state);
                    });
                }
            }

            var tx, ty, canvasW, canvasH, tw, th;
            if (isPinned) {
                tooltip.classList.add('mt-pinned');
                // Position at the pinned object's screen location
                var dpr = getCanvasDPR(state);
                var anchorX, anchorY;
                if (idx >= 0) {
                    var screen = worldToScreen(state, state.positions[idx * 2], state.positions[idx * 2 + 1]);
                    anchorX = screen[0] / dpr;
                    anchorY = screen[1] / dpr;
                } else {
                    // Use polyline midpoint
                    var ppl = state.polylines[poly];
                    var midI = Math.floor(ppl.pointCount / 2);
                    var pscreen = worldToScreen(state, ppl.points[midI * 2], ppl.points[midI * 2 + 1]);
                    anchorX = pscreen[0] / dpr;
                    anchorY = pscreen[1] / dpr;
                }
                tx = anchorX + 14;
                ty = anchorY + 14;
                canvasW = state.canvas.clientWidth;
                canvasH = state.canvas.clientHeight;
                tw = tooltip.offsetWidth || 200;
                th = tooltip.offsetHeight || 100;
                if (tx + tw > canvasW - 4) tx = anchorX - tw - 14;
                if (ty + th > canvasH - 4) ty = anchorY - th - 14;
                tx = Math.max(4, Math.min(tx, canvasW - tw - 4));
                ty = Math.max(4, Math.min(ty, canvasH - th - 4));
                tooltip.style.left = tx + 'px';
                tooltip.style.top = ty + 'px';
            } else {
                tooltip.classList.remove('mt-pinned');
                // Position tooltip near mouse, offset by 14px
                tx = state.mouseX + 14;
                ty = state.mouseY + 14;
                // Keep tooltip within canvas bounds
                canvasW = state.canvas.clientWidth;
                canvasH = state.canvas.clientHeight;
                tw = tooltip.offsetWidth || 200;
                th = tooltip.offsetHeight || 100;
                if (tx + tw > canvasW - 4) tx = state.mouseX - tw - 14;
                if (ty + th > canvasH - 4) ty = state.mouseY - th - 14;
                tooltip.style.left = tx + 'px';
                tooltip.style.top = ty + 'px';
            }
        } else {
            tooltip.style.display = 'none';
            tooltip.classList.remove('mt-pinned');
        }
    }

    // ============================================================
    // Render debouncing + frame orchestration
    // ============================================================

    var renderPending = false;

    function renderMapFrame(state) {
        if (renderPending) return;
        renderPending = true;
        requestAnimationFrame(function() {
            renderPending = false;
            renderWebGL(state);
            drawOverlay(state);
            if (state.pinnedIdx >= 0 || state.pinnedPoly >= 0) {
                updateMapTooltip(state);
            }
            updateObjCount(state);
        });
    }

    // ============================================================
    // Overlay renderer — draws hover highlight, radius circle, and
    // selection markers on a 2D canvas layered on top of the main
    // WebGL2 canvas.
    // ============================================================

    function drawOverlay(state) {
        if (!state.overlay) return;
        var ctx = state.overlay.getContext('2d');
        if (!ctx) return;

        var dpr = getCanvasDPR(state);
        var w = state.overlay.clientWidth;
        var h = state.overlay.clientHeight;

        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        ctx.clearRect(0, 0, w, h);

        // Collectible arrow markers — always visible regardless of zoom.
        //
        // These iterate the precomputed per-category index lists directly. The
        // old code asked the spatial grid for every object in the viewport and
        // then filtered that down to the handful of collectibles, which cost far
        // more than scanning the short list — and the off-screen reject below
        // already does the culling.
        if (state.collectibleIndices && state.collectibleIndices.length > 0) {
            var arrowSize = 7; // half-width of arrow base in px
            var arrowHeight = 10; // arrow height in px
            var cIndices = state.collectibleIndices;
            for (let ci = 0; ci < cIndices.length; ci++) {
                let i = cIndices[ci];
                let cat = state.cats[i];
                if (!isCatVisible(state, cat)) continue;
                if (!isTypeVisible(state, state.types[i])) continue;
                if (isZFiltered(state, state.zValues[i])) continue;
                if (isRadiusFiltered(state, state.positions[i * 2], state.positions[i * 2 + 1])) continue;
                let screen = worldToScreen(state, state.positions[i * 2], state.positions[i * 2 + 1]);
                let sx = screen[0] / dpr;
                let sy = screen[1] / dpr;
                // Skip off-screen
                if (sx < -20 || sx > w + 20 || sy < -20 || sy > h + 20) continue;
                // Draw downward-pointing arrow with tip at (sx, sy)
                ctx.fillStyle = _getCatColorHex(cat);
                ctx.beginPath();
                ctx.moveTo(sx, sy); // tip
                ctx.lineTo(sx - arrowSize, sy - arrowHeight);
                ctx.lineTo(sx + arrowSize, sy - arrowHeight);
                ctx.closePath();
                ctx.fill();
                // Dark outline for contrast
                ctx.strokeStyle = 'rgba(0,0,0,0.5)';
                ctx.lineWidth = 1;
                ctx.stroke();
            }
        }

        // Resource node arrow markers — per-type colors + purity-based sizing
        if (state.resourceNodeIndices && state.resourceNodeIndices.length > 0) {
            var rnIndices = state.resourceNodeIndices;
            for (let ri = 0; ri < rnIndices.length; ri++) {
                let i = rnIndices[ri];
                if (!isCatVisible(state, state.cats[i])) continue;
                if (!isTypeVisible(state, state.types[i])) continue;
                if (!isPurityVisible(state, state.yaws[i])) continue;
                if (isZFiltered(state, state.zValues[i])) continue;
                if (isRadiusFiltered(state, state.positions[i * 2], state.positions[i * 2 + 1])) continue;
                let screen = worldToScreen(state, state.positions[i * 2], state.positions[i * 2 + 1]);
                let sx = screen[0] / dpr;
                let sy = screen[1] / dpr;
                if (sx < -20 || sx > w + 20 || sy < -20 || sy > h + 20) continue;
                // Per-type color
                let info = state.typeInfo[state.types[i]];
                let path = info ? info.path : '';
                let rnColor = resourceTypeColor(path);
                // Purity-based sizing: 0=impure(7px), 1=normal(9px), 2=pure(11px)
                var pur = Math.round(state.yaws[i]);
                if (pur < 0 || pur > 2) pur = 0;
                var rnArrowSize = 5 + pur * 2; // 5, 7, 9 half-width
                var rnArrowHeight = 8 + pur * 2; // 8, 10, 12 height
                ctx.fillStyle = rnColor;
                ctx.beginPath();
                ctx.moveTo(sx, sy); // tip
                ctx.lineTo(sx - rnArrowSize, sy - rnArrowHeight);
                ctx.lineTo(sx + rnArrowSize, sy - rnArrowHeight);
                ctx.closePath();
                ctx.fill();
                ctx.strokeStyle = 'rgba(0,0,0,0.5)';
                ctx.lineWidth = 1;
                ctx.stroke();
            }
        }

        // Hover highlight for objects — draw rect outline + ring marker
        if (state.hoveredIdx >= 0 && state.hoveredIdx < state.count) {
            let i = state.hoveredIdx;
            let screen = worldToScreen(state, state.positions[i * 2], state.positions[i * 2 + 1]);
            let sx = screen[0] / dpr;
            let sy = screen[1] / dpr;

            // If the object has dimensions, draw a rotated rectangle outline
            var effHw = getRenderHw(state, i);
            var effHd = getRenderHd(state, i);
            if (effHw > 0 && effHd > 0) {
                var yawRad = state.yaws[i] * 0.1 * Math.PI / 180.0;
                var halfWPx = effHw * state.cam.zoom;
                var halfDPx = effHd * state.cam.zoom;
                ctx.save();
                ctx.translate(sx, sy);
                ctx.rotate(yawRad);
                ctx.strokeStyle = '#ffffff';
                ctx.lineWidth = 2;
                ctx.strokeRect(-halfWPx, -halfDPx, halfWPx * 2, halfDPx * 2);
                ctx.restore();
            } else {
                // For small objects, draw a filled circle for visibility
                ctx.fillStyle = 'rgba(255,255,255,0.3)';
                ctx.beginPath();
                ctx.arc(sx, sy, 5, 0, Math.PI * 2);
                ctx.fill();
            }

            // Ring marker on top
            ctx.strokeStyle = '#ffffff';
            ctx.lineWidth = 2;
            ctx.beginPath();
            ctx.arc(sx, sy, 8, 0, Math.PI * 2);
            ctx.stroke();
        }

        // Hover highlight for polylines — draw the line thicker in white
        if (state.hoveredPoly >= 0 && state.hoveredPoly < state.polylines.length) {
            var pl = state.polylines[state.hoveredPoly];
            ctx.strokeStyle = '#ffffff';
            ctx.lineWidth = 4;
            ctx.lineCap = 'round';
            ctx.lineJoin = 'round';
            ctx.beginPath();
            for (let j = 0; j < pl.pointCount; j++) {
                let pscreen = worldToScreen(state, pl.points[j * 2], pl.points[j * 2 + 1]);
                let px = pscreen[0] / dpr;
                let py = pscreen[1] / dpr;
                if (j === 0) ctx.moveTo(px, py);
                else ctx.lineTo(px, py);
            }
            ctx.stroke();
            ctx.lineWidth = 1;
            ctx.lineCap = 'butt';
            ctx.lineJoin = 'miter';
        }

        // Pinned object highlight (gold ring + rect outline)
        if (state.pinnedIdx >= 0 && state.pinnedIdx < state.count) {
            let pi = state.pinnedIdx;
            let pscreen = worldToScreen(state, state.positions[pi * 2], state.positions[pi * 2 + 1]);
            let psx = pscreen[0] / dpr;
            let psy = pscreen[1] / dpr;
            var peffHw = getRenderHw(state, pi);
            var peffHd = getRenderHd(state, pi);
            if (peffHw > 0 && peffHd > 0) {
                var pyawRad = state.yaws[pi] * 0.1 * Math.PI / 180.0;
                var phalfWPx = peffHw * state.cam.zoom;
                var phalfDPx = peffHd * state.cam.zoom;
                ctx.save();
                ctx.translate(psx, psy);
                ctx.rotate(pyawRad);
                ctx.strokeStyle = '#ffcc00';
                ctx.lineWidth = 2;
                ctx.strokeRect(-phalfWPx, -phalfDPx, phalfWPx * 2, phalfDPx * 2);
                ctx.restore();
            }
            ctx.strokeStyle = '#ffcc00';
            ctx.lineWidth = 2;
            ctx.beginPath();
            ctx.arc(psx, psy, 10, 0, Math.PI * 2);
            ctx.stroke();
            ctx.lineWidth = 1;
        }

        // Pinned polyline highlight
        if (state.pinnedPoly >= 0 && state.pinnedPoly < state.polylines.length) {
            var ppl = state.polylines[state.pinnedPoly];
            ctx.strokeStyle = '#ffcc00';
            ctx.lineWidth = 5;
            ctx.lineCap = 'round';
            ctx.lineJoin = 'round';
            ctx.beginPath();
            for (let pj = 0; pj < ppl.pointCount; pj++) {
                var ppscreen = worldToScreen(state, ppl.points[pj * 2], ppl.points[pj * 2 + 1]);
                var ppx = ppscreen[0] / dpr;
                var ppy = ppscreen[1] / dpr;
                if (pj === 0) ctx.moveTo(ppx, ppy);
                else ctx.lineTo(ppx, ppy);
            }
            ctx.stroke();
            ctx.lineWidth = 1;
            ctx.lineCap = 'butt';
            ctx.lineJoin = 'miter';
        }

        // Measure tool — draw segments, points, labels, and rubber-band line
        if (state.measure.active && state.measure.points.length > 0) {
            var mPts = state.measure.points;
            var mSegs = state.measure.segments;
            var mColor = '#4a9eff';
            var mActiveColor = '#ffcc00';
            var mLabelBg = 'rgba(15, 15, 15, 0.9)';

            // Solid segments
            ctx.strokeStyle = mColor;
            ctx.lineWidth = 2;
            ctx.lineCap = 'round';
            ctx.lineJoin = 'round';
            for (let si = 0; si < mSegs.length; si++) {
                var seg = mSegs[si];
                var pa = worldToScreen(state, mPts[seg[0]][0], mPts[seg[0]][1]);
                var pb = worldToScreen(state, mPts[seg[1]][0], mPts[seg[1]][1]);
                ctx.beginPath();
                ctx.moveTo(pa[0] / dpr, pa[1] / dpr);
                ctx.lineTo(pb[0] / dpr, pb[1] / dpr);
                ctx.stroke();

                // Per-segment distance label at midpoint
                var segDist = Math.hypot(
                    mPts[seg[1]][0] - mPts[seg[0]][0],
                    mPts[seg[1]][1] - mPts[seg[0]][1]
                );
                var labelX = (pa[0] / dpr + pb[0] / dpr) / 2;
                var labelY = (pa[1] / dpr + pb[1] / dpr) / 2;
                _drawMeasureLabel(ctx, labelX, labelY, _formatDistance(segDist), mColor, mLabelBg);
            }

            // Rubber-band dashed line from active point to cursor
            if (state.measure.activePoint >= 0) {
                var apt = mPts[state.measure.activePoint];
                var aptScreen = worldToScreen(state, apt[0], apt[1]);
                var cursorPt = worldToScreen(state, state.measure.hoverX, state.measure.hoverY);
                ctx.setLineDash([6, 4]);
                ctx.strokeStyle = mActiveColor;
                ctx.lineWidth = 2;
                ctx.beginPath();
                ctx.moveTo(aptScreen[0] / dpr, aptScreen[1] / dpr);
                ctx.lineTo(cursorPt[0] / dpr, cursorPt[1] / dpr);
                ctx.stroke();
                ctx.setLineDash([]);

                // Rubber-band distance label
                var rbDist = Math.hypot(state.measure.hoverX - apt[0], state.measure.hoverY - apt[1]);
                var rbLabelX = (aptScreen[0] / dpr + cursorPt[0] / dpr) / 2;
                var rbLabelY = (aptScreen[1] / dpr + cursorPt[1] / dpr) / 2;
                _drawMeasureLabel(ctx, rbLabelX, rbLabelY, _formatDistance(rbDist), mActiveColor, mLabelBg);
            }

            // Point markers
            for (let mi = 0; mi < mPts.length; mi++) {
                var mps2 = worldToScreen(state, mPts[mi][0], mPts[mi][1]);
                var mx = mps2[0] / dpr;
                var my = mps2[1] / dpr;
                var isActive = (mi === state.measure.activePoint);

                // Outer glow for active point
                if (isActive) {
                    ctx.fillStyle = 'rgba(255, 204, 0, 0.3)';
                    ctx.beginPath();
                    ctx.arc(mx, my, 8, 0, Math.PI * 2);
                    ctx.fill();
                }

                // Filled circle with outline
                ctx.fillStyle = isActive ? mActiveColor : mColor;
                ctx.beginPath();
                ctx.arc(mx, my, 4, 0, Math.PI * 2);
                ctx.fill();
                ctx.strokeStyle = '#fff';
                ctx.lineWidth = 1.5;
                ctx.stroke();
            }

            ctx.lineWidth = 1;
            ctx.lineCap = 'butt';
            ctx.lineJoin = 'miter';
        }

        // Radius filter circle
        if (state.radius.active) {
            var rc = worldToScreen(state, state.radius.cx, state.radius.cy);
            var rsx = rc[0] / dpr;
            var rsy = rc[1] / dpr;
            var rPix = state.radius.meters * state.cam.zoom;
            ctx.strokeStyle = '#ff8800';
            ctx.lineWidth = 2;
            ctx.setLineDash([6, 4]);
            ctx.beginPath();
            ctx.arc(rsx, rsy, rPix, 0, Math.PI * 2);
            ctx.stroke();
            ctx.setLineDash([]);
            // Center marker
            ctx.fillStyle = '#ff8800';
            ctx.beginPath();
            ctx.arc(rsx, rsy, 3, 0, Math.PI * 2);
            ctx.fill();
        }

        // Highlight marker from showOnMap() — crosshair + ripple + building outline
        if (state.highlightTime > 0) {
            var elapsed = (performance.now() - state.highlightTime) / 1000;
            if (elapsed < 6) {
                var alpha = 1 - elapsed / 6;

                if (state.highlightIdx >= 0 && state.highlightIdx < state.count) {
                    var hi = state.highlightIdx;
                    var hscreen = worldToScreen(state, state.positions[hi * 2], state.positions[hi * 2 + 1]);
                    var hsx = hscreen[0] / dpr;
                    var hsy = hscreen[1] / dpr;

                    // Crosshair lines (limited to 150px around target)
                    ctx.strokeStyle = 'rgba(255, 215, 0, ' + (alpha * 0.7).toFixed(2) + ')';
                    ctx.lineWidth = 2;
                    ctx.setLineDash([6, 4]);
                    ctx.beginPath();
                    ctx.moveTo(hsx - 150, hsy);
                    ctx.lineTo(hsx + 150, hsy);
                    ctx.moveTo(hsx, hsy - 150);
                    ctx.lineTo(hsx, hsy + 150);
                    ctx.stroke();
                    ctx.setLineDash([]);

                    // Expanding ripple rings (3 staggered)
                    for (let r = 0; r < 3; r++) {
                        var phase = (elapsed + r * 0.5) % 1.5;
                        if (phase < 1.0) {
                            var ringAlpha = (1 - phase) * alpha;
                            var ringR = 10 + phase * 60;
                            ctx.strokeStyle = 'rgba(255, 215, 0, ' + ringAlpha.toFixed(2) + ')';
                            ctx.lineWidth = 2;
                            ctx.beginPath();
                            ctx.arc(hsx, hsy, ringR, 0, Math.PI * 2);
                            ctx.stroke();
                        }
                    }

                    // Building footprint outline
                    var heffHw = getRenderHw(state, hi);
                    var heffHd = getRenderHd(state, hi);
                    if (heffHw > 0 && heffHd > 0) {
                        var hyawRad = state.yaws[hi] * 0.1 * Math.PI / 180.0;
                        var hhalfWPx = heffHw * state.cam.zoom;
                        var hhalfDPx = heffHd * state.cam.zoom;
                        ctx.save();
                        ctx.translate(hsx, hsy);
                        ctx.rotate(hyawRad);
                        // Bright fill
                        ctx.fillStyle = 'rgba(255, 215, 0, ' + (alpha * 0.25).toFixed(2) + ')';
                        ctx.fillRect(-hhalfWPx, -hhalfDPx, hhalfWPx * 2, hhalfDPx * 2);
                        // Thick outline
                        ctx.strokeStyle = 'rgba(255, 215, 0, ' + alpha.toFixed(2) + ')';
                        ctx.lineWidth = 3;
                        ctx.strokeRect(-hhalfWPx, -hhalfDPx, hhalfWPx * 2, hhalfDPx * 2);
                        ctx.restore();
                    }

                    // Center dot
                    ctx.fillStyle = 'rgba(255, 215, 0, ' + alpha.toFixed(2) + ')';
                    ctx.beginPath();
                    ctx.arc(hsx, hsy, 3, 0, Math.PI * 2);
                    ctx.fill();
                } else {
                    // No object found: fall back to crosshair + ripple at clicked position
                    var hc2 = worldToScreen(state, state.highlightX, state.highlightY);
                    var hsx2 = hc2[0] / dpr;
                    var hsy2 = hc2[1] / dpr;

                    // Crosshair
                    ctx.strokeStyle = 'rgba(255, 215, 0, ' + (alpha * 0.7).toFixed(2) + ')';
                    ctx.lineWidth = 2;
                    ctx.setLineDash([6, 4]);
                    ctx.beginPath();
                    ctx.moveTo(hsx2 - 150, hsy2);
                    ctx.lineTo(hsx2 + 150, hsy2);
                    ctx.moveTo(hsx2, hsy2 - 150);
                    ctx.lineTo(hsx2, hsy2 + 150);
                    ctx.stroke();
                    ctx.setLineDash([]);

                    // Ripple rings
                    for (let r2 = 0; r2 < 3; r2++) {
                        var phase2 = (elapsed + r2 * 0.5) % 1.5;
                        if (phase2 < 1.0) {
                            var ringAlpha2 = (1 - phase2) * alpha;
                            var ringR2 = 10 + phase2 * 60;
                            ctx.strokeStyle = 'rgba(255, 215, 0, ' + ringAlpha2.toFixed(2) + ')';
                            ctx.lineWidth = 2;
                            ctx.beginPath();
                            ctx.arc(hsx2, hsy2, ringR2, 0, Math.PI * 2);
                            ctx.stroke();
                        }
                    }

                    // Center dot
                    ctx.fillStyle = 'rgba(255, 215, 0, ' + alpha.toFixed(2) + ')';
                    ctx.beginPath();
                    ctx.arc(hsx2, hsy2, 3, 0, Math.PI * 2);
                    ctx.fill();
                }
            }
        }
    }

    // ============================================================
    // Measure tool helpers
    // ============================================================

    function _formatDistance(meters) {
        if (meters >= 1000) {
            return (meters / 1000).toFixed(2) + ' km';
        }
        return Math.round(meters) + ' m';
    }

    function _drawMeasureLabel(ctx, x, y, text, color, bgColor) {
        ctx.font = 'bold 11px sans-serif';
        var metrics = ctx.measureText(text);
        var padX = 4, padY = 2;
        var w = metrics.width + padX * 2;
        var h = 11 + padY * 2;
        // Background pill
        ctx.fillStyle = bgColor;
        ctx.beginPath();
        if (ctx.roundRect) {
            ctx.roundRect(x - w / 2, y - h / 2, w, h, 3);
        } else {
            ctx.rect(x - w / 2, y - h / 2, w, h);
        }
        ctx.fill();
        // Border
        ctx.strokeStyle = color;
        ctx.lineWidth = 1;
        ctx.stroke();
        // Text
        ctx.fillStyle = color;
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillText(text, x, y);
        // Reset
        ctx.textAlign = 'left';
        ctx.textBaseline = 'alphabetic';
    }

    function _updateMeasureTotal(state) {
        var el = document.getElementById('mapMeasureTotal');
        if (!el) return;
        var m = state.measure;
        if (!m.active || m.segments.length === 0) {
            el.textContent = '';
            return;
        }
        var total = 0;
        for (var i = 0; i < m.segments.length; i++) {
            var seg = m.segments[i];
            total += Math.hypot(
                m.points[seg[1]][0] - m.points[seg[0]][0],
                m.points[seg[1]][1] - m.points[seg[0]][1]
            );
        }
        el.textContent = t('map.measure_total') + ': ' + _formatDistance(total);
    }

    // ============================================================
    // Spatial Grid (for viewport culling + hover detection)
    // ============================================================

    var GRID_CELL_SIZE = 200; // meters per cell

    function buildSpatialGrid(state) {
        var n = state.count;
        // Compute world bounds
        var minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
        for (let i = 0; i < n; i++) {
            var x = state.positions[i * 2];
            var y = state.positions[i * 2 + 1];
            if (x < minX) minX = x;
            if (y < minY) minY = y;
            if (x > maxX) maxX = x;
            if (y > maxY) maxY = y;
        }
        var cols = Math.max(1, Math.ceil((maxX - minX) / GRID_CELL_SIZE));
        var rows = Math.max(1, Math.ceil((maxY - minY) / GRID_CELL_SIZE));
        var cells = new Array(cols * rows);
        for (let i = 0; i < cells.length; i++) cells[i] = null;

        for (let i = 0; i < n; i++) {
            var cx = Math.floor((state.positions[i * 2] - minX) / GRID_CELL_SIZE);
            var cy = Math.floor((state.positions[i * 2 + 1] - minY) / GRID_CELL_SIZE);
            if (cx < 0) cx = 0; if (cx >= cols) cx = cols - 1;
            if (cy < 0) cy = 0; if (cy >= rows) cy = rows - 1;
            var idx = cy * cols + cx;
            if (!cells[idx]) cells[idx] = [];
            cells[idx].push(i);
        }

        // Build polyline bounding boxes for culling + spatial index
        var polyBBoxes = null;
        var polyCells = null; // maps grid cell index -> array of polyline indices
        if (state.polylines && state.polylines.length > 0) {
            polyBBoxes = [];
            polyCells = new Array(cols * rows);
            for (let i = 0; i < polyCells.length; i++) polyCells[i] = null;
            for (let p = 0; p < state.polylines.length; p++) {
                var pl = state.polylines[p];
                var pminX = Infinity, pminY = Infinity, pmaxX = -Infinity, pmaxY = -Infinity;
                for (let j = 0; j < pl.pointCount; j++) {
                    var px = pl.points[j * 2];
                    var py = pl.points[j * 2 + 1];
                    if (px < pminX) pminX = px;
                    if (py < pminY) pminY = py;
                    if (px > pmaxX) pmaxX = px;
                    if (py > pmaxY) pmaxY = py;
                }
                polyBBoxes.push({ minX: pminX, minY: pminY, maxX: pmaxX, maxY: pmaxY });
                // Register polyline in all grid cells its bbox overlaps
                var pcMinCol = Math.max(0, Math.floor((pminX - minX) / GRID_CELL_SIZE));
                var pcMaxCol = Math.min(cols - 1, Math.floor((pmaxX - minX) / GRID_CELL_SIZE));
                var pcMinRow = Math.max(0, Math.floor((pminY - minY) / GRID_CELL_SIZE));
                var pcMaxRow = Math.min(rows - 1, Math.floor((pmaxY - minY) / GRID_CELL_SIZE));
                for (let prow = pcMinRow; prow <= pcMaxRow; prow++) {
                    for (let pcol = pcMinCol; pcol <= pcMaxCol; pcol++) {
                        var pidx = prow * cols + pcol;
                        if (!polyCells[pidx]) polyCells[pidx] = [];
                        polyCells[pidx].push(p);
                    }
                }
            }
        }

        state.grid = {
            cells: cells, cols: cols, rows: rows,
            minX: minX, minY: minY,
            polyBBoxes: polyBBoxes,
            polyCells: polyCells
        };
        // The memoised viewport index list is keyed on the camera only, so a
        // rebuilt grid has to invalidate it explicitly.
        state._visIdxCache = null;
    }

    // Returns the indices of every object whose grid cell overlaps the viewport.
    //
    // The result depends only on the camera and the canvas size, but the callers
    // (hover on every mousemove, the object counter, and previously the overlay
    // twice per frame) each asked for it independently. Zoomed out on a large
    // save that meant rebuilding a several-hundred-thousand-entry plain array
    // three or four times per frame, all of it immediate garbage. It is now
    // memoised against the camera and canvas size, so a frame builds it at most
    // once and a stationary camera never rebuilds it at all.
    function getVisibleObjectIndices(state) {
        var g = state.grid;
        if (!g) return null;

        var dpr = getCanvasDPR(state);
        var w = state.canvas.clientWidth;
        var h = state.canvas.clientHeight;

        var c = state._visIdxCache;
        if (c && c.cx === state.cam.cx && c.cy === state.cam.cy &&
            c.zoom === state.cam.zoom && c.w === w && c.h === h && c.dpr === dpr) {
            return c.indices;
        }

        var tl = screenToWorld(state, 0, 0);
        var br = screenToWorld(state, w * dpr, h * dpr);
        var viewMinX = Math.min(tl[0], br[0]) - 50;
        var viewMaxX = Math.max(tl[0], br[0]) + 50;
        var viewMinY = Math.min(tl[1], br[1]) - 50;
        var viewMaxY = Math.max(tl[1], br[1]) + 50;

        var minCol = Math.max(0, Math.floor((viewMinX - g.minX) / GRID_CELL_SIZE));
        var maxCol = Math.min(g.cols - 1, Math.floor((viewMaxX - g.minX) / GRID_CELL_SIZE));
        var minRow = Math.max(0, Math.floor((viewMinY - g.minY) / GRID_CELL_SIZE));
        var maxRow = Math.min(g.rows - 1, Math.floor((viewMaxY - g.minY) / GRID_CELL_SIZE));

        // Size the buffer up front so the fill never has to grow it.
        var total = 0;
        for (let row = minRow; row <= maxRow; row++) {
            for (let col = minCol; col <= maxCol; col++) {
                var cnt = g.cells[row * g.cols + col];
                if (cnt) total += cnt.length;
            }
        }
        var indices = new Int32Array(total);
        var at = 0;
        for (let row = minRow; row <= maxRow; row++) {
            for (let col = minCol; col <= maxCol; col++) {
                var cell = g.cells[row * g.cols + col];
                if (cell) {
                    for (let k = 0; k < cell.length; k++) indices[at++] = cell[k];
                }
            }
        }

        state._visIdxCache = {
            cx: state.cam.cx, cy: state.cam.cy, zoom: state.cam.zoom,
            w: w, h: h, dpr: dpr, indices: indices
        };
        return indices;
    }

