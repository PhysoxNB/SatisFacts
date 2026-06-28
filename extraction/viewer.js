(async function() {
    'use strict';

    let DATA = null;
    let currentTab = 'overview';

    // ===== Utilities =====

    function fmt(n) {
        if (n === null || n === undefined) return '0';
        return Number(n).toLocaleString('en-US');
    }

    function fmtKm(m) {
        if (!m) return '0 km';
        return (m / 1000).toFixed(2) + ' km';
    }

    function fmtMw(mw) {
        if (mw === null || mw === undefined) return '0 MW';
        if (mw === -1) return 'Unknown';
        return fmt(Math.round(mw * 10) / 10) + ' MW';
    }

    function fmtPct(p) {
        if (p === null || p === undefined) return '0%';
        return Number(p).toFixed(1) + '%';
    }

    function fmtTime(seconds) {
        if (!seconds) return 'Unknown';
        const s = parseInt(seconds);
        const d = Math.floor(s / 86400);
        const h = Math.floor((s % 86400) / 3600);
        const m = Math.floor((s % 3600) / 60);
        if (d > 0) return d + 'd ' + h + 'h ' + m + 'm';
        if (h > 0) return h + 'h ' + m + 'm';
        return m + 'm';
    }

    function fmtDate(ts) {
        if (!ts) return 'Unknown';
        var d = new Date(typeof ts === 'string' ? parseInt(ts) : ts);
        if (isNaN(d.getTime())) return 'Unknown';
        var yyyy = d.getFullYear();
        var mm = String(d.getMonth() + 1).padStart(2, '0');
        var dd = String(d.getDate()).padStart(2, '0');
        var hh = String(d.getHours()).padStart(2, '0');
        var mi = String(d.getMinutes()).padStart(2, '0');
        var ss = String(d.getSeconds()).padStart(2, '0');
        return yyyy + '-' + mm + '-' + dd + ' ' + hh + ':' + mi + ':' + ss;
    }

    function displayName(typePath) {
        if (!typePath) return 'Unknown';
        // Check sign display names map first
        if (DATA && DATA.signDisplayNames) {
            var shortName = typePath.split('/').pop();
            if (shortName.includes('.')) shortName = shortName.split('.')[0];
            if (DATA.signDisplayNames[shortName]) return DATA.signDisplayNames[shortName];
            if (DATA.signDisplayNames[shortName + '_C']) return DATA.signDisplayNames[shortName + '_C'];
            if (DATA.signDisplayNames[typePath]) return DATA.signDisplayNames[typePath];
        }
        let name = typePath.split('/').pop();
        if (name.includes('.')) name = name.split('.')[0];
        name = name.replace(/^Build_/, '').replace(/_C$/, '');
        name = name.replace(/([a-z0-9])([A-Z])/g, '$1 $2');
        name = name.replace(/_/g, ' ');
        name = name.replace(/Mk\s*(\d+)/gi, ' Mk$1');
        return name.replace(/\s+/g, ' ').trim();
    }

    function recipeName(path) {
        if (!path) return 'Unknown';
        let name = path.split('/').pop();
        name = name.replace(/^Recipe_/, '').replace(/_C$/, '');
        if (name.startsWith('Alternate_')) name = 'Alt: ' + name.replace(/^Alternate_/, '');
        name = name.replace(/([a-z0-9])([A-Z])/g, '$1 $2');
        name = name.replace(/_/g, ' ');
        return name.replace(/\s+/g, ' ').trim();
    }

    function safe(v, fallback) {
        return (v !== null && v !== undefined) ? v : (fallback || 0);
    }

    function esc(s) {
        if (s === null || s === undefined) return '';
        return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    }

    // ===== DOM Helpers =====

    function h(tag, attrs) {
        var el = document.createElement(tag);
        if (attrs) {
            for (var key in attrs) {
                if (key === 'class') el.className = attrs[key];
                else if (key === 'text') el.textContent = attrs[key];
                else if (key === 'html') el.innerHTML = attrs[key];
                else if (key === 'style') el.setAttribute('style', attrs[key]);
                else if (attrs[key] === false || attrs[key] == null) continue;
                else if (attrs[key] === true) el.setAttribute(key, key);
                else el.setAttribute(key, attrs[key]);
            }
        }
        for (var i = 2; i < arguments.length; i++) {
            var child = arguments[i];
            if (child == null) continue;
            if (typeof child === 'string') el.appendChild(document.createTextNode(child));
            else el.appendChild(child);
        }
        return el;
    }

    function card(label, value, desc, color) {
        var style = color ? 'border-left-color: ' + color + '; background: ' + color + '22;' : '';
        var descEl = null;
        if (desc) {
            if (typeof desc === 'object' && desc.__html !== undefined) {
                descEl = h('div', { class: 'desc', html: desc.__html });
            } else {
                descEl = h('div', { class: 'desc', text: desc });
            }
        }
        return h('div', { class: 'card', style: style },
            h('div', { class: 'label', text: label }),
            h('div', { class: 'value', text: String(value) }),
            descEl
        );
    }

    function progressBar(value, max, color) {
        var pct = max > 0 ? Math.min(100, (value / max) * 100) : 0;
        var c = color || '#00d4ff';
        return h('div', { class: 'bar-container' },
            h('div', { class: 'bar-fill', style: 'width: ' + pct + '%; background-color: ' + c + ';' }),
            h('span', { class: 'bar-label', text: String(value) })
        );
    }

    function progressCard(label, collected, total, desc, color) {
        var c = color || '#00d4ff';
        var style = 'border-left-color: ' + c + '; background: ' + c + '22;';
        var pct = total > 0 ? Math.min(100, (collected / total) * 100) : 0;
        var valueText = collected + ' / ' + total;
        var descEl = desc ? h('div', { class: 'desc', text: desc }) : null;
        return h('div', { class: 'card', style: style },
            h('div', { class: 'label', text: label }),
            h('div', { class: 'value', text: valueText }),
            h('div', { class: 'bar-container', style: 'margin-top: 6px;' },
                h('div', { class: 'bar-fill', style: 'width: ' + pct + '%; background-color: ' + c + ';' }),
                h('span', { class: 'bar-label', text: String(collected) })
            ),
            descEl
        );
    }

    function statRow(label, value, opts) {
        opts = opts || {};
        var cls = 'stat-row' + (opts.overclocked ? ' overclocked' : '') + (opts.underclocked ? ' underclocked' : '');
        return h('div', { class: cls },
            h('span', { class: 'label', text: label }),
            h('span', { class: 'value', text: String(value) })
        );
    }

    function badge(text, type) {
        return h('span', { class: 'badge ' + (type || 'green'), text: text });
    }

    function section(title) {
        var s = h('div', { class: 'section' }, h('h2', { text: title }));
        return s;
    }

    function tabDescription(text) {
        var box = h('div', {
            style: 'background: var(--surface); border-left: 3px solid var(--accent); border-radius: 6px; padding: 10px 14px; margin-bottom: 16px; font-size: 0.95em; color: var(--text); display: flex; align-items: flex-start; gap: 8px;',
        });
        var icon = h('span', { style: 'color: var(--accent); font-size: 1.1em; flex-shrink: 0;', text: '\u24D8' });
        var msg = h('span', { text: text });
        box.appendChild(icon);
        box.appendChild(msg);
        return box;
    }

    function sortableTable(headers, rows, opts) {
        opts = opts || {};
        var table = document.createElement('table');
        table.setAttribute('data-sortable', 'true');

        var thead = document.createElement('thead');
        var tr = document.createElement('tr');
        headers.forEach(function(hdr, i) {
            var th = document.createElement('th');
            th.textContent = hdr.label || hdr;
            th.dataset.col = i;
            th.dataset.type = hdr.type || 'string';
            if (hdr.cls) th.className = hdr.cls;
            if (hdr.width) th.style.width = hdr.width;
            th.addEventListener('click', function() {
                sortTable(table, i, hdr.type || 'string', th);
            });
            tr.appendChild(th);
        });
        thead.appendChild(tr);
        table.appendChild(thead);

        var tbody = document.createElement('tbody');
        rows.forEach(function(row) {
            var tr = document.createElement('tr');
            row.forEach(function(cell, ci) {
                var td = document.createElement('td');
                if (headers[ci] && headers[ci].cls) td.className = headers[ci].cls;
                if (cell && cell.__html) td.innerHTML = cell.__html;
                else if (cell && cell.__el) td.appendChild(cell.__el);
                else td.textContent = (cell === null || cell === undefined) ? '' : String(cell);
                tr.appendChild(td);
            });
            tbody.appendChild(tr);
        });
        table.appendChild(tbody);

        if (opts.scrollable) {
            var wrap = h('div', { class: 'scrollable' });
            wrap.appendChild(table);
            return wrap;
        }
        return table;
    }

    function sortTable(table, colIdx, type, th) {
        var tbody = table.querySelector('tbody');
        var rows = Array.from(tbody.querySelectorAll('tr'));
        var ascending = th.classList.contains('sort-asc') ? false : true;

        table.querySelectorAll('th').forEach(function(t) { t.classList.remove('sort-asc', 'sort-desc'); });
        th.classList.add(ascending ? 'sort-asc' : 'sort-desc');

        rows.sort(function(a, b) {
            var va = a.children[colIdx].textContent;
            var vb = b.children[colIdx].textContent;
            if (type === 'number') {
                va = parseFloat(va.replace(/[^0-9.-]/g, '')) || 0;
                vb = parseFloat(vb.replace(/[^0-9.-]/g, '')) || 0;
            }
            if (va < vb) return ascending ? -1 : 1;
            if (va > vb) return ascending ? 1 : -1;
            return 0;
        });

        rows.forEach(function(r) { tbody.appendChild(r); });
    }

    function html(s) {
        return { __html: s };
    }

    // ===== Data Loading =====

    async function loadData() {
        var el = document.getElementById('embedded-data');
        var b64 = el.textContent.trim();
        var binary = atob(b64);
        var bytes = new Uint8Array(binary.length);
        for (var i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);

        if (typeof DecompressionStream === 'undefined') {
            throw new Error('Your browser does not support DecompressionStream. Please use Chrome 80+, Firefox 113+, or Safari 16.4+.');
        }

        var ds = new DecompressionStream('gzip');
        var blob = new Blob([bytes]);
        var stream = blob.stream().pipeThrough(ds);
        var text = await new Response(stream).text();
        return JSON.parse(text);
    }

    // ===== Overview =====

    function renderOverview() {
        var c = document.getElementById('tab-overview');
        c.innerHTML = '';
        var d = DATA;
        var hdr = d.header || {};
        var pb = (d.analytics && d.analytics.power && d.analytics.power.power_balance) || {};
        var tr = (d.analytics && d.analytics.transport) || {};
        var mp = (d.analytics && d.analytics.map) || {};
        var st = (d.structures && d.structures.totals) || {};
        var gp = d.gameProgression || {};

        var s1 = section('Save Information');
        var grid1 = h('div', { class: 'card-grid' });
        grid1.appendChild(card('Save Name', hdr.SaveName || hdr.SessionName || 'Unknown', ''));
        grid1.appendChild(card('Session', hdr.SessionName || 'Unknown', 'Version ' + hdr.SaveVersion));
        grid1.appendChild(card('Play Time', fmtTime(hdr.PlayDurationSeconds), 'Build ' + hdr.BuildVersion));
        grid1.appendChild(card('Save Date', fmtDate(hdr.SaveDateTime), ''));
        if (hdr.IsModdedSave) grid1.appendChild(card('Modded', 'Yes', ''));
        s1.appendChild(grid1);
        c.appendChild(s1);

        var s2 = section('Colony Summary');
        var grid2 = h('div', { class: 'card-grid' });
        grid2.appendChild(card('Total Objects', fmt(d.totalObjects), ''));
        grid2.appendChild(card('Total Buildings', fmt(d.totalBuildings), (d.buildings ? Object.keys(d.buildings).length : 0) + ' unique types'));
        var totalStructures = 0;
        for (var k in st) totalStructures += st[k] || 0;
        grid2.appendChild(card('Total Structures', fmt(totalStructures), 'Foundations, walls, beams, etc.'));
        if (d.analytics) {
            grid2.appendChild(card('Power Capacity', fmtMw(pb.capacity_mw), pb.surplus_mw >= 0 ? 'Surplus: ' + fmtMw(pb.surplus_mw) : 'Deficit: ' + fmtMw(Math.abs(pb.surplus_mw))));
            grid2.appendChild(card('Power Consumption', fmtMw(pb.consumption_mw), ''));
            grid2.appendChild(card('Infrastructure', tr.total_infrastructure_km ? tr.total_infrastructure_km.toFixed(1) + ' km' : '0 km', 'Belts, pipes, power lines, etc.'));
            grid2.appendChild(card('Map Area', mp.total_area_km2 ? mp.total_area_km2.toFixed(2) + ' km\u00b2' : '0', mp.building_density ? Math.round(mp.building_density).toLocaleString('en-US') + ' buildings/km\u00b2' : ''));
        }
        s2.appendChild(grid2);
        c.appendChild(s2);

        if (gp.currentPhase || gp.targetPhase) {
            var s3 = section('Project Assembly');
            var grid3 = h('div', { class: 'card-grid' });
            grid3.appendChild(card('Current Phase', (gp.currentPhase || 'Unknown').replace('Project_Assembly_Phase_', 'Phase '), ''));
            grid3.appendChild(card('Target Phase', (gp.targetPhase || 'Unknown').replace('Project_Assembly_Phase_', 'Phase '), ''));
            if (gp.sinkTotalPoints) grid3.appendChild(card('Sink Points', fmt(gp.sinkTotalPoints), ''));
            if (gp.sinkCoupons) grid3.appendChild(card('Sink Coupons', fmt(gp.sinkCoupons), ''));
            if (gp.purchasedSchematics) grid3.appendChild(card('Schematics', fmt(gp.purchasedSchematics.length), 'purchased'));
            s3.appendChild(grid3);
            c.appendChild(s3);
        }

        // Custom map settings (1.2+)
        if (gp.nodePuritySetting || gp.nodeRandomization || gp.partsCostMultiplier || gp.spacePartsCostMultiplier || gp.powerConsumptionMultiplier) {
            var sMap = section('Custom Map Settings');
            var gridMap = h('div', { class: 'card-grid' });
            if (gp.nodePuritySetting) {
                var purityMap = { 'AllPure': 'All Pure', 'MostlyPure': 'Mostly Pure', 'Average': 'Average', 'MostlyImpure': 'Mostly Impure', 'AllImpure': 'All Impure', 'Random': 'Random', 'Default': 'Default' };
                gridMap.appendChild(card('Resource Node Purity', purityMap[gp.nodePuritySetting] || gp.nodePuritySetting, 'Resource node purity setting'));
            }
            if (gp.nodeRandomization) {
                var randMap = { 'Default': 'Default', 'Random': 'Random', 'BasicResourceRich': 'Basic Resource Rich', 'AdvancedResourceRich': 'Advanced Resource Rich', 'FossilFuelRich': 'Fossil Fuel Rich', 'Strict': 'Default (Strict)' };
                gridMap.appendChild(card('Resource Node Randomization', randMap[gp.nodeRandomization] || gp.nodeRandomization, 'Resource node layout mode'));
            }
            if (gp.nodeRandomizationSeed) {
                gridMap.appendChild(card('World Seed', gp.nodeRandomizationSeed, gp.nodeRandomizationSeed === 0 ? 'Default vanilla seed' : 'Share this seed with friends'));
            }
            if (gp.partsCostMultiplier) {
                gridMap.appendChild(card('Recipe Parts Cost', gp.partsCostMultiplier + 'x', 'Recipe part costs'));
            }
            if (gp.spacePartsCostMultiplier) {
                gridMap.appendChild(card('Space Elevator Cost', gp.spacePartsCostMultiplier + 'x', 'Space elevator deliverable costs'));
            }
            if (gp.powerConsumptionMultiplier) {
                gridMap.appendChild(card('Power Consumption', gp.powerConsumptionMultiplier + 'x', 'Power consumption multiplier'));
            }
            sMap.appendChild(gridMap);
            c.appendChild(sMap);
        }

    }

    // ===== Structures =====

    function renderStructures() {
        var c = document.getElementById('tab-structures');
        c.innerHTML = '';
        var d = DATA;
        var totals = (d.structures && d.structures.totals) || {};
        var details = (d.structures && d.structures.details) || {};

        // Summary cards
        var s1 = section('Structure Summary');
        var grid = h('div', { class: 'card-grid' });
        var cats = Object.keys(totals).sort(function(a, b) { return totals[b] - totals[a]; });
        cats.forEach(function(cat) {
            if (totals[cat] > 0) {
                grid.appendChild(card(cat.charAt(0).toUpperCase() + cat.slice(1), fmt(totals[cat]), ''));
            }
        });
        s1.appendChild(grid);
        c.appendChild(s1);

        // Detailed breakdown per category
        cats.forEach(function(cat) {
            if (!details[cat] || Object.keys(details[cat]).length === 0) return;
            var s = section(cat.charAt(0).toUpperCase() + cat.slice(1) + ' (' + fmt(totals[cat]) + ')');
            var types = details[cat];
            var rows = Object.keys(types).filter(function(type) { return types[type] > 0; }).map(function(type) {
                return [displayName(type), fmt(types[type]), type];
            }).sort(function(a, b) {
                return parseInt(b[1].replace(/,/g, '')) - parseInt(a[1].replace(/,/g, ''));
            });
            if (rows.length === 0) return;
            s.appendChild(sortableTable(
                [{label: 'Type', width: '45%'}, {label: 'Count', type: 'number', width: '10%'}, {label: 'Type Path', cls: 'type-path-col', width: '45%'}],
                rows,
                {scrollable: rows.length > 20}
            ));
            c.appendChild(s);
        });
    }

    // ===== Buildings =====

    function renderBuildings() {
        var c = document.getElementById('tab-buildings');
        c.innerHTML = '';
        var d = DATA;
        var buildings = d.buildings || {};

        var buildingKeys = Object.keys(buildings).filter(function(type) { return buildings[type] > 0; });
        var total = 0;
        buildingKeys.forEach(function(type) { total += buildings[type]; });

        // Categorize buildings
        var categories = {
            'Production': {},
            'Power': {},
            'Mining': {},
            'Storage': {},
            'Transport': {},
            'Other': {}
        };

        buildingKeys.forEach(function(type) {
            var lower = type.toLowerCase();
            var cat = 'Other';

            // Power (check before production — "generator" not "generatorfuel" as production)
            if (lower.includes('generator') || lower.includes('powerstorage')) {
                cat = 'Power';
            }
            // Mining
            else if (lower.includes('miner') || lower.includes('fracking') || lower.includes('oilpump') || lower.includes('resourcecollector')) {
                cat = 'Mining';
            }
            // Storage
            else if (lower.includes('storage') || lower.includes('container') || lower.includes('centralstorage') || lower.includes('industrialtank') || lower.includes('pipestoragetank')) {
                cat = 'Storage';
            }
            // Transport (trains, signals, stations, tracks)
            else if (lower.includes('train') || lower.includes('railroad')) {
                cat = 'Transport';
            }
            // Production
            else if (lower.includes('assembler') || lower.includes('smelter') || lower.includes('foundry') ||
                lower.includes('constructor') || lower.includes('manufacturer') || lower.includes('refinery') ||
                lower.includes('blender') || lower.includes('packager') || lower.includes('converter') ||
                lower.includes('hadron') || lower.includes('collider') || lower.includes('waterpump') ||
                lower.includes('portal') || lower.includes('accelerator') || lower.includes('quantum') ||
                lower.includes('encoder')) {
                cat = 'Production';
            }

            categories[cat][type] = buildings[type];
        });

        var catOrder = ['Production', 'Power', 'Mining', 'Storage', 'Transport', 'Other'];
        catOrder.forEach(function(catName) {
            var catBuildings = categories[catName];
            var catKeys = Object.keys(catBuildings);
            if (catKeys.length === 0) return;

            var catTotal = 0;
            catKeys.forEach(function(type) { catTotal += catBuildings[type]; });

            var s = section(catName + ' (' + fmt(catTotal) + ')');
            var rows = catKeys.map(function(type) {
                return [displayName(type), fmt(catBuildings[type]), type];
            }).sort(function(a, b) {
                return parseInt(b[1].replace(/,/g, '')) - parseInt(a[1].replace(/,/g, ''));
            });
            s.appendChild(sortableTable(
                [{label: 'Building', width: '45%'}, {label: 'Count', type: 'number', width: '10%'}, {label: 'Type Path', cls: 'type-path-col', width: '45%'}],
                rows,
                {scrollable: rows.length > 20}
            ));
            c.appendChild(s);
        });
    }

    function renderClockSpeeds() {
        var c = document.getElementById('tab-clockspeeds');
        c.innerHTML = '';
        var d = DATA;
        var csd = (d.analytics && d.analytics.clockSpeedDistribution) || {};
        var pbd = (d.analytics && d.analytics.productionBoostDistribution) || {};
        var css = (d.analytics && d.analytics.clockSpeedSloopCounts) || {};

        if (Object.keys(csd).length === 0) {
            c.appendChild(h('p', { text: 'No clock speed data available.', style: 'color: #888;' }));
            return;
        }

        // Legend / explanation
        var legend = section('What is Clock Speed & Somersloops?');
        legend.appendChild(h('p', { text: 'Clock speed determines how fast a building operates. 100% is the default. Buildings can be underclocked (slower, uses less power) or overclocked (faster, uses more power) using Power Shards.', style: 'color: #aaa; margin-bottom: 12px;' }));
        legend.appendChild(h('p', { text: 'Somersloops double the output of a production building without increasing input, but at the cost of significantly higher power usage (up to 4x with max sloop slots). Each building type has a different number of sloop slots. A building can be both overclocked AND slooped.', style: 'color: #aaa; margin-bottom: 12px;' }));
        var legendGrid = h('div', { class: 'card-grid' });
        legendGrid.appendChild(card('100% (Default)', '', html('<span style="color:#00ff00;">●</span> Green = default speed, no Power Shards needed')));
        legendGrid.appendChild(card('Below 100%', '', html('<span style="color:#6495ED;">●</span> Blue = underclocked, slower but more power-efficient')));
        legendGrid.appendChild(card('Above 100%', '', html('<span style="color:#ff6464;">●</span> Red = overclocked, faster but requires Power Shards')));
        legendGrid.appendChild(card('Somerslooped', '', html('<span style="color:#a855f7;">●</span> Purple = Somersloop inserted, output doubled, power usage up to 4x')));
        legend.appendChild(legendGrid);
        c.appendChild(legend);

        // Merge building types from both distributions
        var allTypes = {};
        Object.keys(csd).forEach(function(t) { allTypes[t] = true; });
        Object.keys(pbd).forEach(function(t) { allTypes[t] = true; });

        var s1 = section('Building Overclock & Somersloop Distribution');
        Object.keys(allTypes).sort(function(a, b) {
            return d.buildings[b] - d.buildings[a];
        }).forEach(function(type) {
            var csDist = csd[type] || {};
            var pbDist = pbd[type] || {};

            var csTotal = 0;
            for (var speed in csDist) csTotal += csDist[speed];
            var pbTotal = 0;
            for (var key in pbDist) pbTotal += pbDist[key];
            if (csTotal === 0 && pbTotal === 0) return;

            var summaryText = displayName(type) + ' (' + fmt(csTotal || pbTotal) + ')';
            if (pbTotal > 0) summaryText += ' — ' + pbTotal + ' slooped';

            var det = h('details', {});
            det.appendChild(h('summary', { text: summaryText }));

            // Clock speed table
            if (csTotal > 0) {
                det.appendChild(h('h4', { text: 'Clock Speed', style: 'margin: 12px 0 6px; color: #ccc;' }));
                var sloopDist = css[type] || {};
                var csRows = Object.keys(csDist).map(function(speed) {
                    var count = csDist[speed];
                    var pct = (count / csTotal * 100).toFixed(1);
                    var color = speed > 100 ? '#ff6464' : (speed < 100 ? '#6495ED' : '#00ff00');
                    var slooped = sloopDist[speed] || 0;
                    var sloopCell = slooped > 0
                        ? html('<span style="color:#a855f7;font-weight:bold;">' + slooped + ' slooped</span>')
                        : '';
                    return [
                        speed + '%',
                        fmt(count),
                        sloopCell,
                        html('<div style="display:flex;align-items:center;gap:6px;"><div style="flex:1;min-width:60px;">' + progressBar(count, csTotal, color).outerHTML + '</div><b>' + pct + '%</b></div>')
                    ];
                }).sort(function(a, b) {
                    return parseInt(b[0]) - parseInt(a[0]);
                });
                det.appendChild(sortableTable(
                    [{label: 'Clock Speed'}, {label: 'Buildings', type: 'number'}, {label: 'Slooped'}, {label: 'Share of Total'}],
                    csRows
                ));
            }

            s1.appendChild(det);
        });
        c.appendChild(s1);
    }

    // ===== Power =====

    function renderPower() {
        var c = document.getElementById('tab-power');
        c.innerHTML = '';
        var d = DATA;
        c.appendChild(tabDescription('Power generation and consumption across your factory. Location coordinates match the in-game map — you can copy and paste them directly into the game to find any building.'));
        var pw = (d.analytics && d.analytics.power) || {};
        var gen = pw.generators || {};
        var con = pw.consumers || {};
        var bal = pw.power_balance || {};
        var storage = pw.power_storage || {};
        var apa = pw.apa || {};
        var gp = (d.extraction && d.extraction.GeneratorPower) || {};
        var pc = (d.extraction && d.extraction.PowerConsumption) || {};

        // Summary cards
        var s1 = section('Power Balance');
        var grid = h('div', { class: 'card-grid' });
        var surplus = bal.surplus_mw || 0;
        var genCap = bal.generator_capacity_mw || bal.capacity_mw || 0;
        var apaBase = bal.apa_base_power_mw || 0;
        var apaMult = bal.apa_multiplier || 1;
        var totalCap = bal.capacity_mw || genCap;
        var genSubtext = (gen.active_generators || 0) + ' active generators';
        if (apaBase > 0) genSubtext += ' + ' + fmtMw(apaBase) + ' APA base';
        grid.appendChild(card('Generation', fmtMw(totalCap), genSubtext, '#00ff00'));
        var theoMax = bal.theoretical_max_mw || 0;
        if (theoMax > 0) {
            var theoAug = (theoMax + apaBase) * apaMult;
            grid.appendChild(card('Max Theoretical', fmtMw(theoAug), 'All generators running \u00D7' + apaMult.toFixed(1) + ' APA', '#ffa500'));
        }
        grid.appendChild(card('Consumption', fmtMw(bal.consumption_mw), (con.active_buildings || 0) + ' active buildings', '#ff6464'));
        grid.appendChild(card(surplus >= 0 ? 'Surplus' : 'Deficit', (surplus >= 0 ? '+' : '') + fmtMw(surplus), surplus >= 0 ? 'Power available' : 'Not enough power!', surplus >= 0 ? '#00ff00' : '#ff6464'));
        if (storage.capacity_mwh) {
            var storageSubtext = (storage.count || 0) + ' units \u00B7 ' + fmtMw(storage.max_charge_rate_mw || 0) + ' max charge';
            if (storage.stored_mwh !== undefined) {
                storageSubtext = fmt(storage.stored_mwh) + ' / ' + fmt(storage.capacity_mwh) + ' MWh stored \u00B7 ' + (storage.count || 0) + ' units';
            }
            grid.appendChild(card('Power Storage', fmt(storage.capacity_mwh) + ' MWh', storageSubtext, '#9370db'));
        }
        if (apa.building_count) grid.appendChild(card('APA', apa.building_count + ' installed', (apa.fueled_count || 0) + ' fueled', '#ffc800'));
        s1.appendChild(grid);
        c.appendChild(s1);

        // APA detailed info box
        if (apa.building_count && apa.building_count > 0) {
            var apaBox = h('div', {
                style: 'margin: 0 0 16px; padding: 12px 14px; background: rgba(255, 200, 0, 0.12); border-left: 4px solid #ffc800; border-radius: 6px;'
            });
            var apaHeader = h('div', { style: 'display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;' });
            apaHeader.appendChild(h('strong', { style: 'font-size: 1.1em;', text: '\u26A1 APA (Alien Power Augmenter)' }));
            var boostPct = ((apaMult - 1) * 100).toFixed(1);
            apaHeader.appendChild(h('span', { style: 'color: #ffc800; font-size: 1.2em; font-weight: bold;', text: '+' + boostPct + '% Grid Boost' }));
            apaBox.appendChild(apaHeader);

            var fueled = apa.fueled_count || 0;
            var unfueled = apa.unfueled_count || 0;
            var apaDetail = h('div', { style: 'font-size: 0.95em; margin-bottom: 8px;' });
            apaDetail.appendChild(h('strong', { text: apa.building_count + ' APAs' }));
            apaDetail.appendChild(h('span', { text: ' installed: ' }));
            if (fueled > 0) apaDetail.appendChild(h('span', { style: 'color: #00ff00;', text: fueled + ' fueled (+30% each)' }));
            if (fueled > 0 && unfueled > 0) apaDetail.appendChild(h('span', { text: ', ' }));
            if (unfueled > 0) apaDetail.appendChild(h('span', { style: 'color: #ffa500;', text: unfueled + ' unfueled (+10% each)' }));
            apaBox.appendChild(apaDetail);

            var apaStats = h('div', { style: 'font-size: 0.9em; color: #aaa;' });
            apaStats.appendChild(h('div', { text: 'Base APA Power: ' + fmtMw(apaBase) + ' (500 MW per APA)' }));
            apaStats.appendChild(h('div', { text: 'Active Generator Capacity: ' + fmtMw(gen.active_capacity_mw || 0) }));
            apaStats.appendChild(h('div', { style: 'margin-top: 4px;', text: 'Total Augmented Capacity: ' }));
            var augStrong = h('strong', { style: 'color: #ffc800;', text: fmtMw(apa.augmented_capacity_mw || totalCap) + ' (\u00D7' + apaMult.toFixed(1) + ' multiplier)' });
            apaStats.appendChild(augStrong);
            var maxAug = apa.max_augmented_capacity_mw || 0;
            if (maxAug > (apa.augmented_capacity_mw || 0)) {
                apaStats.appendChild(h('div', { style: 'margin-top: 6px; padding-top: 6px; border-top: 1px solid rgba(255, 200, 0, 0.2); color: #888;' }));
                apaStats.appendChild(h('div', { text: 'Max Theoretical (all generators running):' }));
                apaStats.appendChild(h('strong', { style: 'color: #888;', text: fmtMw(maxAug) + ' (\u00D7' + apaMult.toFixed(1) + ' multiplier)' }));
                apaStats.appendChild(h('div', { style: 'font-size: 0.85em; color: #666; margin-top: 2px;', text: 'Based on ' + fmtMw(bal.theoretical_max_mw || 0) + ' theoretical max (base power \u00D7 clock speed)' }));
            }
            apaBox.appendChild(apaStats);

            if (unfueled > 0) {
                var apaTip = h('div', { style: 'margin-top: 8px; padding-top: 8px; border-top: 1px solid rgba(255, 200, 0, 0.3); font-size: 0.85em; color: #aaa;' });
                apaTip.appendChild(h('span', { text: '\u{1F4A1} Tip: Fuel APAs with Alien Power Matrix to increase boost from +10% to +30%!' }));
                apaBox.appendChild(apaTip);
            }
            c.appendChild(apaBox);
        }

        // Generator types breakdown
        if (gen.by_type && Object.keys(gen.by_type).length > 0) {
            var s2 = section('Generator Types');
            var rows = Object.keys(gen.by_type).map(function(type) {
                var g = gen.by_type[type];
                return [displayName(type), g.count, g.active, g.standby, fmtMw(g.total_mw), fmtMw(g.active_mw)];
            }).sort(function(a, b) { return b[4] - a[4]; });
            s2.appendChild(sortableTable(
                [{label: 'Type'}, {label: 'Total', type: 'number'}, {label: 'Active', type: 'number'}, {label: 'Standby', type: 'number'}, {label: 'Capacity', type: 'number'}, {label: 'Active MW', type: 'number'}],
                rows
            ));
            c.appendChild(s2);
        }

        // Individual generators (paginated for performance with large saves)
        if (gp.Generators && gp.Generators.length > 0) {
            var s3 = section('Individual Generators (' + gp.Generators.length + ')');
            var genPageSize = 200;
            var genState = { filtered: gp.Generators, page: 0 };

            // Search/filter bar
            var genFilterBar = h('div', { class: 'filter-bar', style: 'margin-bottom:8px;display:flex;gap:8px;align-items:center;flex-wrap:wrap;' });
            var genSearchInput = h('input', { type: 'text', placeholder: 'Filter by type, status, or instance...', style: 'flex:1;min-width:200px;padding:4px 8px;' });
            var genStatusFilter = h('select', { style: 'padding:4px;' });
            genStatusFilter.appendChild(h('option', { value: '' }, 'All statuses'));
            ['Active', 'Standby'].forEach(function(st) {
                genStatusFilter.appendChild(h('option', { value: st }, st));
            });
            genFilterBar.appendChild(h('span', {}, 'Filter:'));
            genFilterBar.appendChild(genSearchInput);
            genFilterBar.appendChild(genStatusFilter);
            s3.appendChild(genFilterBar);

            var genTableContainer = h('div', {});
            s3.appendChild(genTableContainer);

            var genPagerBar = h('div', { style: 'margin-top:8px;display:flex;gap:8px;align-items:center;justify-content:space-between;' });
            var genPageInfo = h('span', {});
            var genPagerButtons = h('div', { style: 'display:flex;gap:4px;' });
            genPagerBar.appendChild(genPageInfo);
            genPagerBar.appendChild(genPagerButtons);
            s3.appendChild(genPagerBar);

            function renderGeneratorPage() {
                genTableContainer.innerHTML = '';
                var start = genState.page * genPageSize;
                var end = Math.min(start + genPageSize, genState.filtered.length);
                var pageRows = [];
                for (var i = start; i < end; i++) {
                    var g = genState.filtered[i];
                    var statusColor = g.Status === 'Active' ? 'green' : (g.Status === 'Standby' ? 'orange' : 'red');
                    var statusBadge = badge(g.Status || 'Unknown', statusColor);
                    var locStr = '';
                    if (g.Location) {
                        locStr = g.Location.X + ', ' + g.Location.Y + ' (alt: ' + g.Location.Z + 'm)';
                    }
                    pageRows.push([g.Type, {__el: statusBadge}, fmtMw(g.ActualMW), fmtMw(g.MaxMW), g.Reason || '', locStr, g.InstanceName]);
                }
                genTableContainer.appendChild(sortableTable(
                    [{label: 'Type'}, {label: 'Status'}, {label: 'Actual', type: 'number'}, {label: 'Max', type: 'number'}, {label: 'Reason'}, {label: 'Location'}, {label: 'Instance', cls: 'type-path-col'}],
                    pageRows,
                    {scrollable: true}
                ));

                // Update pager
                var totalPages = Math.ceil(genState.filtered.length / genPageSize);
                genPageInfo.textContent = 'Showing ' + (start + 1) + '-' + end + ' of ' + genState.filtered.length + (genState.filtered.length !== gp.Generators.length ? ' (filtered from ' + gp.Generators.length + ')' : '');
                genPagerButtons.innerHTML = '';
                var prevBtn = h('button', { disabled: genState.page === 0 }, '◀ Prev');
                prevBtn.style.padding = '4px 12px';
                prevBtn.addEventListener('click', function() { if (genState.page > 0) { genState.page--; renderGeneratorPage(); } });
                genPagerButtons.appendChild(prevBtn);

                var pageInput = h('span', { style: 'padding:0 8px;' }, (genState.page + 1) + ' / ' + Math.max(1, totalPages));
                genPagerButtons.appendChild(pageInput);

                var nextBtn = h('button', { disabled: genState.page >= totalPages - 1 }, 'Next ▶');
                nextBtn.style.padding = '4px 12px';
                nextBtn.addEventListener('click', function() { if (genState.page < totalPages - 1) { genState.page++; renderGeneratorPage(); } });
                genPagerButtons.appendChild(nextBtn);
            }

            function applyGenFilter() {
                var q = genSearchInput.value.toLowerCase().trim();
                var stFilter = genStatusFilter.value;
                genState.filtered = gp.Generators.filter(function(g) {
                    if (stFilter && g.Status !== stFilter) return false;
                    if (!q) return true;
                    return g.Type.toLowerCase().indexOf(q) >= 0 ||
                           g.InstanceName.toLowerCase().indexOf(q) >= 0 ||
                           (g.Reason || '').toLowerCase().indexOf(q) >= 0 ||
                           (g.Location && ((g.Location.X + ',' + g.Location.Y + ',' + g.Location.Z).indexOf(q) >= 0));
                });
                genState.page = 0;
                renderGeneratorPage();
            }

            genSearchInput.addEventListener('input', applyGenFilter);
            genStatusFilter.addEventListener('change', applyGenFilter);
            renderGeneratorPage();
            c.appendChild(s3);
        }

        // Individual consumers (paginated for performance with large saves)
        if (pc.Consumers && pc.Consumers.length > 0) {
            var s4 = section('Power Consumers (' + pc.Consumers.length + ')');
            var pageSize = 200;
            var consumerState = { filtered: pc.Consumers, page: 0 };

            // Search/filter bar
            var filterBar = h('div', { class: 'filter-bar', style: 'margin-bottom:8px;display:flex;gap:8px;align-items:center;flex-wrap:wrap;' });
            var searchInput = h('input', { type: 'text', placeholder: 'Filter by type, status, or instance...', style: 'flex:1;min-width:200px;padding:4px 8px;' });
            var statusFilter = h('select', { style: 'padding:4px;' });
            statusFilter.appendChild(h('option', { value: '' }, 'All statuses'));
            ['Running', 'Idle', 'Paused', 'Blocked', 'Starving', 'Unknown'].forEach(function(st) {
                statusFilter.appendChild(h('option', { value: st }, st));
            });
            filterBar.appendChild(h('span', {}, 'Filter:'));
            filterBar.appendChild(searchInput);
            filterBar.appendChild(statusFilter);
            s4.appendChild(filterBar);

            var tableContainer = h('div', {});
            s4.appendChild(tableContainer);

            var pagerBar = h('div', { style: 'margin-top:8px;display:flex;gap:8px;align-items:center;justify-content:space-between;' });
            var pageInfo = h('span', {});
            var pagerButtons = h('div', { style: 'display:flex;gap:4px;' });
            pagerBar.appendChild(pageInfo);
            pagerBar.appendChild(pagerButtons);
            s4.appendChild(pagerBar);

            function renderConsumerPage() {
                tableContainer.innerHTML = '';
                var start = consumerState.page * pageSize;
                var end = Math.min(start + pageSize, consumerState.filtered.length);
                var pageRows = [];
                for (var i = start; i < end; i++) {
                    var c = consumerState.filtered[i];
                    var statusColor = ({
                        'Running': 'green',
                        'Idle': 'blue',
                        'Paused': 'orange',
                        'Starving': 'yellow',
                        'Blocked': 'red',
                        'Standby': 'orange'
                    })[c.Status] || 'gray';
                    var statusBadge = badge(c.Status || 'Unknown', statusColor);
                    var locStr = '';
                    if (c.Location) {
                        locStr = c.Location.X + ', ' + c.Location.Y + ' (alt: ' + c.Location.Z + 'm)';
                    }
                    pageRows.push([c.Type, {__el: statusBadge}, fmtMw(c.ActualMW), fmtMw(c.MaxMW), c.Reason || '', locStr, c.InstanceName]);
                }
                tableContainer.appendChild(sortableTable(
                    [{label: 'Type'}, {label: 'Status'}, {label: 'Actual', type: 'number'}, {label: 'Max', type: 'number'}, {label: 'Reason'}, {label: 'Location'}, {label: 'Instance', cls: 'type-path-col'}],
                    pageRows,
                    {scrollable: true}
                ));

                // Update pager
                var totalPages = Math.ceil(consumerState.filtered.length / pageSize);
                pageInfo.textContent = 'Showing ' + (start + 1) + '-' + end + ' of ' + consumerState.filtered.length + (consumerState.filtered.length !== pc.Consumers.length ? ' (filtered from ' + pc.Consumers.length + ')' : '');
                pagerButtons.innerHTML = '';
                var prevBtn = h('button', { disabled: consumerState.page === 0 }, '◀ Prev');
                prevBtn.style.padding = '4px 12px';
                prevBtn.addEventListener('click', function() { if (consumerState.page > 0) { consumerState.page--; renderConsumerPage(); } });
                pagerButtons.appendChild(prevBtn);

                var pageInput = h('span', { style: 'padding:0 8px;' }, (consumerState.page + 1) + ' / ' + Math.max(1, totalPages));
                pagerButtons.appendChild(pageInput);

                var nextBtn = h('button', { disabled: consumerState.page >= totalPages - 1 }, 'Next ▶');
                nextBtn.style.padding = '4px 12px';
                nextBtn.addEventListener('click', function() { if (consumerState.page < totalPages - 1) { consumerState.page++; renderConsumerPage(); } });
                pagerButtons.appendChild(nextBtn);
            }

            function applyFilter() {
                var q = searchInput.value.toLowerCase().trim();
                var stFilter = statusFilter.value;
                consumerState.filtered = pc.Consumers.filter(function(c) {
                    if (stFilter && c.Status !== stFilter) return false;
                    if (!q) return true;
                    return c.Type.toLowerCase().indexOf(q) >= 0 ||
                           c.InstanceName.toLowerCase().indexOf(q) >= 0 ||
                           (c.Reason || '').toLowerCase().indexOf(q) >= 0 ||
                           (c.Location && ((c.Location.X + ',' + c.Location.Y + ',' + c.Location.Z).indexOf(q) >= 0));
                });
                consumerState.page = 0;
                renderConsumerPage();
            }

            searchInput.addEventListener('input', applyFilter);
            statusFilter.addEventListener('change', applyFilter);
            renderConsumerPage();
            c.appendChild(s4);
        }

        // Power Grid Topology
        var pgd = d.powerGridData;
        if (pgd && (pgd.Circuits || pgd.circuits) && (pgd.Circuits || pgd.circuits).length > 0) {
            var circuits = pgd.Circuits || pgd.circuits;
            var s5 = section('Power Grid Topology');

            // Summary stats grid
            var gridStats = h('div', { class: 'card-grid' });
            gridStats.appendChild(card('Circuits', fmt(circuits.length), 'Power networks', '#00d4ff'));
            gridStats.appendChild(card('Components', fmt(pgd.ComponentCount || pgd.componentCount || 0), 'Power connections', '#00d4ff'));
            gridStats.appendChild(card('Power Lines', fmt(pgd.LineCount || pgd.lineCount || 0), 'Cables connecting poles', '#00d4ff'));

            // Count power switches from structures
            var switchCount = 0;
            if (d.structures && d.structures.details && d.structures.details.powerSwitches) {
                var sw = d.structures.details.powerSwitches;
                for (var swType in sw) {
                    if (sw.hasOwnProperty(swType)) switchCount += sw[swType];
                }
            }
            if (switchCount > 0) {
                gridStats.appendChild(card('Switches', fmt(switchCount), 'Power switches', '#ffa500'));
            }
            s5.appendChild(gridStats);

            // Circuit details — sorted by component count descending
            var sorted = circuits.slice().sort(function(a, b) {
                return (b.ComponentCount || b.componentCount || 0) - (a.ComponentCount || a.componentCount || 0);
            });
            var circuitRows = sorted.map(function(circuit, i) {
                var cc = circuit.ComponentCount || circuit.componentCount || 0;
                var color = cc > 500 ? '#ff6464' : (cc > 200 ? '#ffc800' : '#00ff80');
                var badgeClass = cc > 500 ? 'red' : (cc > 200 ? 'yellow' : 'green');
                return ['Circuit #' + (i + 1), {__el: badge(cc + ' components', badgeClass)}, cc];
            });
            s5.appendChild(sortableTable(
                [{label: 'Circuit'}, {label: 'Load'}, {label: 'Components', type: 'number'}],
                circuitRows,
                {scrollable: true}
            ));
            c.appendChild(s5);
        }
    }

    // ==== Production ====
    function renderProduction() {
        var c = document.getElementById('tab-production');
        c.innerHTML = '';
        var d = DATA;
        c.appendChild(tabDescription('Mining and resource extraction overview.'));
        var prod = (d.analytics && d.analytics.production) || {};
        var miners = prod.miners || {};
        var minerDetails = prod.miner_details || {};
        var fluidEx = prod.fluid_extractors || {};
        var prodBuildings = prod.production_buildings || {};

        // Summary cards
        var s1 = section('Mining & Extraction');
        var grid = h('div', { class: 'card-grid' });
        grid.appendChild(card('Total Miner Capacity', fmt(Math.round(prod.total_miner_capacity || 0)) + '/min', ''));
        grid.appendChild(card('Total Fluid Capacity', fmt(Math.round(prod.total_fluid_capacity || 0)) + '/min', ''));
        grid.appendChild(card('Miner Types', Object.keys(miners).length, ''));
        grid.appendChild(card('Miner Instances', Object.keys(minerDetails).length, ''));
        s1.appendChild(grid);
        c.appendChild(s1);

        // Miner summary by type
        if (Object.keys(miners).length > 0) {
            var s2 = section('Miners by Type');
            var rows = Object.keys(miners).map(function(type) {
                var m = miners[type];
                return [type, m.avg_clock_speed ? m.avg_clock_speed.toFixed(1) + '%' : '100%', m.base_rate || 60, ''];
            });
            s2.appendChild(sortableTable(
                [{label: 'Type'}, {label: 'Avg Clock', type: 'number'}, {label: 'Base Rate', type: 'number'}, {label: 'Details'}],
                rows
            ));
            c.appendChild(s2);
        }

        // Miner details (paginated for performance with large saves)
        if (Object.keys(minerDetails).length > 0) {
            var minerList = Object.keys(minerDetails).map(function(inst) {
                var m = minerDetails[inst];
                var locStr = '';
                if (m.location) {
                    locStr = m.location.X + ', ' + m.location.Y + ' (alt: ' + m.location.Z + 'm)';
                }
                return [m.type || 'Unknown', m.clock_speed + '%', m.purity || 'unknown', m.resource || 'Unknown', fmt(m.capacity) + '/min', locStr, (m.input_belts || []).join(', '), (m.output_belts || []).join(', '), inst];
            });
            var s3 = section('Miner Details (' + minerList.length + ')');
            var minerPageSize = 200;
            var minerState = { filtered: minerList, page: 0 };

            var minerFilterBar = h('div', { class: 'filter-bar', style: 'margin-bottom:8px;display:flex;gap:8px;align-items:center;flex-wrap:wrap;' });
            var minerSearchInput = h('input', { type: 'text', placeholder: 'Filter by type, resource, or instance...', style: 'flex:1;min-width:200px;padding:4px 8px;' });
            minerFilterBar.appendChild(h('span', {}, 'Filter:'));
            minerFilterBar.appendChild(minerSearchInput);
            s3.appendChild(minerFilterBar);

            var minerTableContainer = h('div', {});
            s3.appendChild(minerTableContainer);

            var minerPagerBar = h('div', { style: 'margin-top:8px;display:flex;gap:8px;align-items:center;justify-content:space-between;' });
            var minerPageInfo = h('span', {});
            var minerPagerButtons = h('div', { style: 'display:flex;gap:4px;' });
            minerPagerBar.appendChild(minerPageInfo);
            minerPagerBar.appendChild(minerPagerButtons);
            s3.appendChild(minerPagerBar);

            function renderMinerPage() {
                minerTableContainer.innerHTML = '';
                var start = minerState.page * minerPageSize;
                var end = Math.min(start + minerPageSize, minerState.filtered.length);
                var pageRows = [];
                for (var i = start; i < end; i++) {
                    pageRows.push(minerState.filtered[i]);
                }
                minerTableContainer.appendChild(sortableTable(
                    [{label: 'Type'}, {label: 'Clock', type: 'number'}, {label: 'Purity'}, {label: 'Resource'}, {label: 'Capacity', type: 'number'}, {label: 'Location'}, {label: 'Input Belts'}, {label: 'Output Belts'}, {label: 'Instance', cls: 'type-path-col'}],
                    pageRows,
                    {scrollable: true}
                ));

                var totalPages = Math.ceil(minerState.filtered.length / minerPageSize);
                minerPageInfo.textContent = 'Showing ' + (start + 1) + '-' + end + ' of ' + minerState.filtered.length + (minerState.filtered.length !== minerList.length ? ' (filtered from ' + minerList.length + ')' : '');
                minerPagerButtons.innerHTML = '';
                var prevBtn = h('button', { disabled: minerState.page === 0 }, '◀ Prev');
                prevBtn.style.padding = '4px 12px';
                prevBtn.addEventListener('click', function() { if (minerState.page > 0) { minerState.page--; renderMinerPage(); } });
                minerPagerButtons.appendChild(prevBtn);

                var pageInput = h('span', { style: 'padding:0 8px;' }, (minerState.page + 1) + ' / ' + Math.max(1, totalPages));
                minerPagerButtons.appendChild(pageInput);

                var nextBtn = h('button', { disabled: minerState.page >= totalPages - 1 }, 'Next ▶');
                nextBtn.style.padding = '4px 12px';
                nextBtn.addEventListener('click', function() { if (minerState.page < totalPages - 1) { minerState.page++; renderMinerPage(); } });
                minerPagerButtons.appendChild(nextBtn);
            }

            function applyMinerFilter() {
                var q = minerSearchInput.value.toLowerCase().trim();
                minerState.filtered = minerList.filter(function(row) {
                    if (!q) return true;
                    return row[0].toLowerCase().indexOf(q) >= 0 ||
                           (row[3] || '').toLowerCase().indexOf(q) >= 0 ||
                           (row[8] || '').toLowerCase().indexOf(q) >= 0;
                });
                minerState.page = 0;
                renderMinerPage();
            }

            minerSearchInput.addEventListener('input', applyMinerFilter);
            renderMinerPage();
            c.appendChild(s3);
        }

        // Fluid extractors
        if (Object.keys(fluidEx).length > 0) {
            var s4 = section('Fluid Extractors');
            var rows = Object.keys(fluidEx).map(function(type) {
                var f = fluidEx[type];
                return [type, f.avg_clock_speed ? f.avg_clock_speed.toFixed(1) + '%' : '100%', f.base_rate || 60, ''];
            });
            s4.appendChild(sortableTable(
                [{label: 'Type'}, {label: 'Avg Clock', type: 'number'}, {label: 'Base Rate', type: 'number'}, {label: 'Details'}],
                rows
            ));
            c.appendChild(s4);
        }

        // Production buildings
        if (Object.keys(prodBuildings).length > 0) {
            var s5 = section('Production Buildings');
            var rows = Object.keys(prodBuildings).map(function(type) {
                return [displayName(type), fmt(prodBuildings[type]), type];
            }).sort(function(a, b) {
                return parseInt(b[1].replace(/,/g, '')) - parseInt(a[1].replace(/,/g, ''));
            });
            s5.appendChild(sortableTable(
                [{label: 'Building', width: '45%'}, {label: 'Count', type: 'number', width: '10%'}, {label: 'Type Path', cls: 'type-path-col', width: '45%'}],
                rows,
                {scrollable: true}
            ));
            c.appendChild(s5);
        }
    }

    // ===== Manufacturing =====

    function renderManufacturing() {
        var c = document.getElementById('tab-manufacturing');
        c.innerHTML = '';
        var d = DATA;
        c.appendChild(tabDescription('Production building efficiency and recipe analysis.'));
        var mfg = (d.analytics && d.analytics.manufacturing) || {};
        var eff = (d.analytics && d.analytics.efficiency) || {};

        // Summary cards
        var s1 = section('Manufacturing Summary');
        var grid = h('div', { class: 'card-grid' });
        grid.appendChild(card('Active Buildings', fmt(mfg.active_count || 0), ''));
        grid.appendChild(card('Total Buildings', fmt(mfg.total_manufacturing_buildings || 0), ''));
        grid.appendChild(card('Recipes in Use', fmt(mfg.total_recipes_in_use || 0), ''));
        s1.appendChild(grid);
        c.appendChild(s1);

        // Helper functions for manufacturing display
        function fmtLoc(loc) {
            if (!loc) return '';
            return loc.X + ', ' + loc.Y + ' (alt: ' + loc.Z + 'm)';
        }
        function fmtType(typePath) {
            if (!typePath) return 'Unknown';
            var parts = typePath.split('/');
            var last = parts[parts.length - 1];
            return last.split('.')[0].replace(/^Build_/, '').replace(/_C$/, '');
        }

        // Recipe production table
        if (mfg.active_production && Object.keys(mfg.active_production).length > 0) {
            var s2 = section('Production by Recipe');
            Object.keys(mfg.active_production).forEach(function(recipeKey) {
                var r = mfg.active_production[recipeKey];
                var effPct = r.efficiency || 0;
                var color = effPct > 100 ? '#ff6464' : (effPct > 80 ? '#00ff00' : (effPct > 50 ? '#ffa500' : '#ff6464'));
                var effBar = html('<div style="display:flex;align-items:center;gap:8px;"><div style="flex:1;min-width:80px;">' + progressBar(effPct, 200, color).outerHTML + '</div><span>' + effPct.toFixed(1) + '%</span></div>');
                var displayName = r.recipe_name || recipeKey;
                var rows = [[displayName, fmt(r.building_count || r.count || 0), (r.avg_clock_speed || 0) + '%', fmt(r.actual_output || 0), fmt(r.theoretical_max || 0), effBar]];

                var det = h('details', {});
                det.appendChild(h('summary', { text: displayName + ' (' + (r.building_count || r.count || 0) + ' buildings, ' + effPct.toFixed(1) + '% efficiency)' }));
                det.appendChild(sortableTable(
                    [{label: 'Recipe'}, {label: 'Buildings', type: 'number'}, {label: 'Avg Clock', type: 'number'}, {label: 'Output/min', type: 'number'}, {label: 'Max/min', type: 'number'}, {label: 'Efficiency'}],
                    rows,
                    {scrollable: true}
                ));

                // Building details with location and type (paginated for performance)
                if (r.buildings && r.buildings.length > 0) {
                    var bldgDet = h('details', { style: 'margin-top:8px;' });
                    bldgDet.appendChild(h('summary', { text: 'Show ' + r.buildings.length + ' Buildings' }));

                    var bldgPageSize = 200;
                    var bldgState = { filtered: r.buildings, page: 0 };

                    var bldgFilterBar = h('div', { class: 'filter-bar', style: 'margin-bottom:8px;display:flex;gap:8px;align-items:center;flex-wrap:wrap;' });
                    var bldgSearchInput = h('input', { type: 'text', placeholder: 'Filter by type or instance...', style: 'flex:1;min-width:200px;padding:4px 8px;' });
                    bldgFilterBar.appendChild(h('span', {}, 'Filter:'));
                    bldgFilterBar.appendChild(bldgSearchInput);
                    bldgDet.appendChild(bldgFilterBar);

                    var bldgTableContainer = h('div', {});
                    bldgDet.appendChild(bldgTableContainer);

                    var bldgPagerBar = h('div', { style: 'margin-top:8px;display:flex;gap:8px;align-items:center;justify-content:space-between;' });
                    var bldgPageInfo = h('span', {});
                    var bldgPagerButtons = h('div', { style: 'display:flex;gap:4px;' });
                    bldgPagerBar.appendChild(bldgPageInfo);
                    bldgPagerBar.appendChild(bldgPagerButtons);
                    bldgDet.appendChild(bldgPagerBar);

                    function renderBldgPage() {
                        bldgTableContainer.innerHTML = '';
                        var start = bldgState.page * bldgPageSize;
                        var end = Math.min(start + bldgPageSize, bldgState.filtered.length);
                        var pageRows = [];
                        for (var i = start; i < end; i++) {
                            var b = bldgState.filtered[i];
                            pageRows.push([fmtType(b.type), b.clock_speed + '%', fmtLoc(b.location), (b.input_belts || []).join(', '), (b.output_belts || []).join(', '), b.instance]);
                        }
                        bldgTableContainer.appendChild(sortableTable(
                            [{label: 'Type'}, {label: 'Clock', type: 'number'}, {label: 'Location'}, {label: 'Input Belts'}, {label: 'Output Belts'}, {label: 'Instance', cls: 'type-path-col'}],
                            pageRows,
                            {scrollable: true}
                        ));

                        var totalPages = Math.ceil(bldgState.filtered.length / bldgPageSize);
                        bldgPageInfo.textContent = 'Showing ' + (start + 1) + '-' + end + ' of ' + bldgState.filtered.length + (bldgState.filtered.length !== r.buildings.length ? ' (filtered from ' + r.buildings.length + ')' : '');
                        bldgPagerButtons.innerHTML = '';
                        var prevBtn = h('button', { disabled: bldgState.page === 0 }, '◀ Prev');
                        prevBtn.style.padding = '4px 12px';
                        prevBtn.addEventListener('click', function() { if (bldgState.page > 0) { bldgState.page--; renderBldgPage(); } });
                        bldgPagerButtons.appendChild(prevBtn);

                        var pageInput = h('span', { style: 'padding:0 8px;' }, (bldgState.page + 1) + ' / ' + Math.max(1, totalPages));
                        bldgPagerButtons.appendChild(pageInput);

                        var nextBtn = h('button', { disabled: bldgState.page >= totalPages - 1 }, 'Next ▶');
                        nextBtn.style.padding = '4px 12px';
                        nextBtn.addEventListener('click', function() { if (bldgState.page < totalPages - 1) { bldgState.page++; renderBldgPage(); } });
                        bldgPagerButtons.appendChild(nextBtn);
                    }

                    function applyBldgFilter() {
                        var q = bldgSearchInput.value.toLowerCase().trim();
                        bldgState.filtered = r.buildings.filter(function(b) {
                            if (!q) return true;
                            return (b.type || '').toLowerCase().indexOf(q) >= 0 ||
                                   (b.instance || '').toLowerCase().indexOf(q) >= 0;
                        });
                        bldgState.page = 0;
                        renderBldgPage();
                    }

                    bldgSearchInput.addEventListener('input', applyBldgFilter);
                    bldgDet.addEventListener('toggle', function() { if (bldgDet.open && bldgTableContainer.children.length === 0) renderBldgPage(); });
                    det.appendChild(bldgDet);
                }
                s2.appendChild(det);
            });
            c.appendChild(s2);
        }

    }

    // ===== Storage =====

    function renderStorage() {
        var c = document.getElementById('tab-storage');
        c.innerHTML = '';
        var d = DATA;
        c.appendChild(tabDescription('Inventory and storage container overview.'));
        var st = (d.analytics && d.analytics.storage) || {};
        var inv = (d.analytics && d.analytics.inventory) || {};

        // Container summary
        var s1 = section('Storage Containers');
        var grid = h('div', { class: 'card-grid' });
        grid.appendChild(card('Total Slots', fmt(st.total_slots || 0), ''));
        var containers = st.containers || {};
        Object.keys(containers).forEach(function(type) {
            var ct = containers[type];
            if (!ct.count || ct.count === 0) return;
            grid.appendChild(card(type.replace(/_/g, ' ').replace(/\b\w/g, function(c) { return c.toUpperCase(); }), fmt(ct.count), fmt(ct.slots) + ' slots each'));
        });
        s1.appendChild(grid);
        c.appendChild(s1);

        // Inventory items
        if (inv.items_by_count && Object.keys(inv.items_by_count).length > 0) {
            var s2 = section('Inventory Items (' + inv.unique_item_types + ' unique types, ' + fmt(inv.total_items) + ' total)');
            var rows = Object.keys(inv.items_by_count).map(function(item) {
                return [item.replace(/([a-z0-9])([A-Z])/g, '$1 $2'), fmt(inv.items_by_count[item])];
            }).sort(function(a, b) {
                return parseInt(b[1].replace(/,/g, '')) - parseInt(a[1].replace(/,/g, ''));
            });
            s2.appendChild(sortableTable(
                [{label: 'Item', width: '75%'}, {label: 'Count', type: 'number', width: '25%'}],
                rows,
                {scrollable: true}
            ));
            c.appendChild(s2);
        }

        // Inventory by category
        if (inv.categories && Object.keys(inv.categories).length > 0) {
            var s3 = section('Inventory by Category');
            Object.keys(inv.categories).forEach(function(cat) {
                var items = inv.categories[cat];
                if (Object.keys(items).length === 0) return;
                var det = h('details', {});
                det.appendChild(h('summary', { text: cat.charAt(0).toUpperCase() + cat.slice(1) + ' (' + Object.keys(items).length + ' types)' }));
                var rows = Object.keys(items).map(function(item) {
                    return [item.replace(/([a-z0-9])([A-Z])/g, '$1 $2'), fmt(items[item])];
                }).sort(function(a, b) {
                    return parseInt(b[1].replace(/,/g, '')) - parseInt(a[1].replace(/,/g, ''));
                });
                det.appendChild(sortableTable([{label: 'Item', width: '75%'}, {label: 'Count', type: 'number', width: '25%'}], rows, {scrollable: true}));
                s3.appendChild(det);
            });
            c.appendChild(s3);
        }
    }

    // ===== Transport =====

    function renderTransport() {
        var c = document.getElementById('tab-transport');
        c.innerHTML = '';
        var d = DATA;
        c.appendChild(tabDescription('Belts, pipes, power lines, power poles, and vehicle infrastructure.'));
        var tr = (d.analytics && d.analytics.transport) || {};
        var prod = d.production || {};

        // Infrastructure breakdown
        var s1 = section('Infrastructure');
        var grid = h('div', { class: 'card-grid' });
        var ib = tr.infrastructure_breakdown || {};
        Object.keys(ib).forEach(function(key) {
            if (!ib[key] || ib[key] === 0) return;
            var label = key.replace(/_/g, ' ').replace(/\b\w/g, function(c) { return c.toUpperCase(); });
            grid.appendChild(card(label, ib[key].toFixed(2) + ' km', ''));
        });
        s1.appendChild(grid);
        c.appendChild(s1);

        // Belt types
        if (prod.belts && Object.keys(prod.belts).length > 0) {
            var s2 = section('Conveyor Belts');
            var rows = Object.keys(prod.belts).map(function(type) {
                var b = prod.belts[type];
                return [displayName(type), fmt(b.Count), fmtKm(b.TotalLength)];
            }).sort(function(a, b) { return parseInt(b[1].replace(/,/g, '')) - parseInt(a[1].replace(/,/g, '')); });
            s2.appendChild(sortableTable(
                [{label: 'Type', width: '45%'}, {label: 'Count', type: 'number', width: '20%'}, {label: 'Length', type: 'number', width: '35%'}],
                rows
            ));
            c.appendChild(s2);
        }

        // Pipe types
        if (prod.pipes && Object.keys(prod.pipes).length > 0) {
            var s3 = section('Pipes');
            var rows = Object.keys(prod.pipes).map(function(type) {
                var p = prod.pipes[type];
                return [displayName(type), fmt(p.Count), fmtKm(p.TotalLength)];
            }).sort(function(a, b) { return parseInt(b[1].replace(/,/g, '')) - parseInt(a[1].replace(/,/g, '')); });
            s3.appendChild(sortableTable(
                [{label: 'Type', width: '45%'}, {label: 'Count', type: 'number', width: '20%'}, {label: 'Length', type: 'number', width: '35%'}],
                rows
            ));
            c.appendChild(s3);
        }

        // Lifts
        if (prod.lifts && Object.keys(prod.lifts).length > 0) {
            var s4 = section('Conveyor Lifts');
            var rows = Object.keys(prod.lifts).map(function(type) {
                var l = prod.lifts[type];
                return [displayName(type), fmt(l.Count), fmtKm(l.TotalLength)];
            }).sort(function(a, b) { return parseInt(b[1].replace(/,/g, '')) - parseInt(a[1].replace(/,/g, '')); });
            s4.appendChild(sortableTable(
                [{label: 'Type', width: '45%'}, {label: 'Count', type: 'number', width: '20%'}, {label: 'Height', type: 'number', width: '35%'}],
                rows
            ));
            c.appendChild(s4);
        }

        // Power lines & poles
        if ((prod.powerLines && prod.powerLines.length > 0) || prod.powerPoles) {
            var s5 = section('Power Lines');
            var grid5 = h('div', { class: 'card-grid' });
            if (prod.powerLines && prod.powerLines.length > 0) {
                var total = prod.powerLines.reduce(function(a, b) { return a + b; }, 0);
                grid5.appendChild(card('Total Lines', fmt(prod.powerLines.length), fmtKm(total)));
            }
            if (prod.powerPoles) grid5.appendChild(card('Power Poles', fmt(prod.powerPoles), ''));
            s5.appendChild(grid5);
            c.appendChild(s5);
        }

        // Hypertubes
        if (prod.hypertubes && prod.hypertubes.length > 0) {
            var s6 = section('Hypertubes');
            var total = prod.hypertubes.reduce(function(a, b) { return a + b; }, 0);
            var grid6 = h('div', { class: 'card-grid' });
            grid6.appendChild(card('Total Segments', fmt(prod.hypertubes.length), fmtKm(total)));
            if (prod.hypertubeJunctions) grid6.appendChild(card('Junctions & Entrances', fmt(prod.hypertubeJunctions), ''));
            s6.appendChild(grid6);
            c.appendChild(s6);
        }

        // Rails
        if (prod.rails && prod.rails.TotalLength) {
            var s7 = section('Rails');
            s7.appendChild(card('Total Length', fmtKm(prod.rails.TotalLength), (prod.rails.Count || 0) + ' segments'));
            c.appendChild(s7);
        }

        // Trains
        var trains = prod.trains || {};
        var trainTotal = (trains.Locomotives || 0) + (trains.Wagons || 0) + (trains.Stations || 0);
        if (trainTotal > 0) {
            var s8 = section('Trains');
            var grid8 = h('div', { class: 'card-grid' });
            if (trains.Locomotives) grid8.appendChild(card('Locomotives', fmt(trains.Locomotives), ''));
            if (trains.Wagons) grid8.appendChild(card('Wagons', fmt(trains.Wagons), ''));
            if (trains.Stations) grid8.appendChild(card('Stations', fmt(trains.Stations), ''));
            s8.appendChild(grid8);
            c.appendChild(s8);
        }

        // Drones
        var drones = prod.drones || {};
        var droneTotal = (drones.Stations || 0) + (drones.FreightPlatforms || 0);
        if (droneTotal > 0) {
            var s9 = section('Drones');
            var grid9 = h('div', { class: 'card-grid' });
            if (drones.Stations) grid9.appendChild(card('Drone Stations', fmt(drones.Stations), ''));
            if (drones.FreightPlatforms) grid9.appendChild(card('Freight Platforms', fmt(drones.FreightPlatforms), ''));
            s9.appendChild(grid9);
            c.appendChild(s9);
        }

        // Vehicles
        var vehicles = prod.vehicles || {};
        var vehicleTotal = (vehicles.Trucks || 0) + (vehicles.Explorers || 0) + (vehicles.Tractors || 0) + (vehicles.Cykles || 0);
        if (vehicleTotal > 0) {
            var s10 = section('Vehicles');
            var grid10 = h('div', { class: 'card-grid' });
            if (vehicles.Trucks) grid10.appendChild(card('Trucks', fmt(vehicles.Trucks), ''));
            if (vehicles.Explorers) grid10.appendChild(card('Explorers', fmt(vehicles.Explorers), ''));
            if (vehicles.Tractors) grid10.appendChild(card('Tractors', fmt(vehicles.Tractors), ''));
            if (vehicles.Cykles) grid10.appendChild(card('Cykles', fmt(vehicles.Cykles), ''));
            s10.appendChild(grid10);
            c.appendChild(s10);
        }

        // Splitters/Mergers
        var splitterMergerTotal = (prod.splitters || 0) + (prod.smartSplitters || 0) + (prod.programmableSplitters || 0) + (prod.mergers || 0);
        if (splitterMergerTotal > 0) {
            var s11 = section('Splitters & Mergers');
            var grid11 = h('div', { class: 'card-grid' });
            if (prod.splitters) grid11.appendChild(card('Splitters', fmt(prod.splitters), ''));
            if (prod.smartSplitters) grid11.appendChild(card('Smart Splitters', fmt(prod.smartSplitters), ''));
            if (prod.programmableSplitters) grid11.appendChild(card('Programmable Splitters', fmt(prod.programmableSplitters), ''));
            if (prod.mergers) grid11.appendChild(card('Mergers', fmt(prod.mergers), ''));
            s11.appendChild(grid11);
            c.appendChild(s11);
        }
    }

    // ===== Map =====

    function renderMap() {
        var c = document.getElementById('tab-map');
        c.innerHTML = '';
        var d = DATA;
        c.appendChild(tabDescription('Geographic distribution and density of your factory.'));
        var mp = (d.analytics && d.analytics.map) || {};

        var s1 = section('Map Statistics');
        var grid = h('div', { class: 'card-grid' });
        var bb = mp.bounding_box || {};
        grid.appendChild(card('Total Area', mp.total_area_km2 ? mp.total_area_km2.toFixed(3) + ' km\u00b2' : '0', fmt(Math.round(mp.total_area_m2 || 0)) + ' m\u00b2'));
        grid.appendChild(card('Total Buildings', fmt(mp.total_buildings || 0), ''));
        grid.appendChild(card('Building Density', mp.building_density ? Math.round(mp.building_density).toLocaleString('en-US') + '/km\u00b2' : '0', ''));
        grid.appendChild(card('Foundation Count', fmt(mp.foundation_count || 0), ''));
        if (bb.width_m && bb.length_m) {
            grid.appendChild(card('Build Area', bb.width_m.toFixed(0) + 'm \u00d7 ' + bb.length_m.toFixed(0) + 'm', 'Bounding box of all buildings'));
        }
        s1.appendChild(grid);
        c.appendChild(s1);
    }

    // ===== Nuclear Waste =====

    function renderNuclear() {
        var c = document.getElementById('tab-nuclear');
        c.innerHTML = '';
        var d = DATA;
        var nw = (d.analytics && d.analytics.nuclearWaste) || {};

        if (!nw || !nw.plant_count) {
            c.appendChild(section('Nuclear Power'));
            c.appendChild(h('div', { class: 'empty-state', text: 'No nuclear power plants found.' }));
            return;
        }

        // Overview cards
        var s1 = section('\u2622\ufe0e Nuclear Power Overview ' + fmtMw(nw.total_power_output || 0));
        var grid = h('div', { class: 'card-grid' });
        grid.appendChild(card('Nuclear Plants', nw.plant_count || 0, ''));
        grid.appendChild(card('Avg Clock Speed', (nw.average_clock_speed || 0).toFixed(1) + '%', ''));
        grid.appendChild(card('Waste Production', (nw.total_waste_production || 0).toFixed(1) + ' /min', ''));
        grid.appendChild(card('Stored Waste', fmt(nw.total_stored_waste || 0), (nw.waste_containers || 0) + ' containers'));
        s1.appendChild(grid);
        c.appendChild(s1);

        // Fuel type breakdown
        var ftc = nw.fuel_type_counts || {};
        if (Object.keys(ftc).length > 0) {
            var s2 = section('Fuel Types');
            var grid2 = h('div', { class: 'card-grid' });
            var fuelColors = { 'Uranium': '#00ff00', 'Plutonium': '#00d4ff', 'Ficsonium': '#ff6464' };
            Object.keys(ftc).sort().forEach(function(ft) {
                grid2.appendChild(card(ft, ftc[ft] + ' plant' + (ftc[ft] !== 1 ? 's' : ''), '', fuelColors[ft] || '#888'));
            });
            s2.appendChild(grid2);
            c.appendChild(s2);
        }

        // Per-plant details (paginated for performance with large saves)
        if (nw.plants && nw.plants.length > 0) {
            var s3 = section('Nuclear Plants (' + nw.plants.length + ')');
            var nucPageSize = 200;
            var nucState = { filtered: nw.plants, page: 0 };

            var nucFilterBar = h('div', { class: 'filter-bar', style: 'margin-bottom:8px;display:flex;gap:8px;align-items:center;flex-wrap:wrap;' });
            var nucSearchInput = h('input', { type: 'text', placeholder: 'Filter by fuel type or instance...', style: 'flex:1;min-width:200px;padding:4px 8px;' });
            nucFilterBar.appendChild(h('span', {}, 'Filter:'));
            nucFilterBar.appendChild(nucSearchInput);
            s3.appendChild(nucFilterBar);

            var nucTableContainer = h('div', {});
            s3.appendChild(nucTableContainer);

            var nucPagerBar = h('div', { style: 'margin-top:8px;display:flex;gap:8px;align-items:center;justify-content:space-between;' });
            var nucPageInfo = h('span', {});
            var nucPagerButtons = h('div', { style: 'display:flex;gap:4px;' });
            nucPagerBar.appendChild(nucPageInfo);
            nucPagerBar.appendChild(nucPagerButtons);
            s3.appendChild(nucPagerBar);

            function renderNucPage() {
                nucTableContainer.innerHTML = '';
                var start = nucState.page * nucPageSize;
                var end = Math.min(start + nucPageSize, nucState.filtered.length);
                var pageRows = [];
                for (var i = start; i < end; i++) {
                    var p = nucState.filtered[i];
                    var origIdx = nw.plants.indexOf(p);
                    var fuelColor = { 'Uranium': '#00ff00', 'Plutonium': '#00d4ff', 'Ficsonium': '#ff6464' }[p.fuelType] || '#888';
                    var fuelBadge = h('span', { class: 'badge', style: 'background: ' + fuelColor + '22; color: ' + fuelColor + ';', text: p.fuelType || 'Unknown' });
                    pageRows.push([
                        '#' + (origIdx + 1),
                        { __el: fuelBadge },
                        (p.clockSpeed || 0).toFixed(1) + '%',
                        fmtMw(p.powerOutput || 0),
                        (p.wasteProductionRate || 0).toFixed(1) + ' waste/min'
                    ]);
                }
                nucTableContainer.appendChild(sortableTable(
                    [{label: 'Plant'}, {label: 'Fuel Type'}, {label: 'Clock Speed', type: 'number'}, {label: 'Power Output', type: 'number'}, {label: 'Waste Production', type: 'number'}],
                    pageRows,
                    {scrollable: true}
                ));

                var totalPages = Math.ceil(nucState.filtered.length / nucPageSize);
                nucPageInfo.textContent = 'Showing ' + (start + 1) + '-' + end + ' of ' + nucState.filtered.length + (nucState.filtered.length !== nw.plants.length ? ' (filtered from ' + nw.plants.length + ')' : '');
                nucPagerButtons.innerHTML = '';
                var prevBtn = h('button', { disabled: nucState.page === 0 }, '◀ Prev');
                prevBtn.style.padding = '4px 12px';
                prevBtn.addEventListener('click', function() { if (nucState.page > 0) { nucState.page--; renderNucPage(); } });
                nucPagerButtons.appendChild(prevBtn);

                var pageInput = h('span', { style: 'padding:0 8px;' }, (nucState.page + 1) + ' / ' + Math.max(1, totalPages));
                nucPagerButtons.appendChild(pageInput);

                var nextBtn = h('button', { disabled: nucState.page >= totalPages - 1 }, 'Next ▶');
                nextBtn.style.padding = '4px 12px';
                nextBtn.addEventListener('click', function() { if (nucState.page < totalPages - 1) { nucState.page++; renderNucPage(); } });
                nucPagerButtons.appendChild(nextBtn);
            }

            function applyNucFilter() {
                var q = nucSearchInput.value.toLowerCase().trim();
                nucState.filtered = nw.plants.filter(function(p) {
                    if (!q) return true;
                    return (p.fuelType || '').toLowerCase().indexOf(q) >= 0 ||
                           (p.instanceName || '').toLowerCase().indexOf(q) >= 0;
                });
                nucState.page = 0;
                renderNucPage();
            }

            nucSearchInput.addEventListener('input', applyNucFilter);
            renderNucPage();
            c.appendChild(s3);
        }
    }

    // ===== Collectibles =====

    function renderCollectibles() {
        var c = document.getElementById('tab-collectibles');
        c.innerHTML = '';
        var d = DATA;
        var col = (d.analytics && d.analytics.collectibles) || {};

        c.appendChild(tabDescription('Collectibles and scattered items found on the map. World collectibles (Power Slugs, Somersloops, Mercer Spheres, Crash Sites) are tracked using the collectables list embedded in the save file, which records each item the player has collected or dismantled. Item Pickups show what is still available on the map with exact locations.'));

        // ===== World Collectibles (slugs, sloops, spheres, crash sites) =====
        var wc = col.world_collectibles;
        if (wc) {
            var ws = section('World Collectibles');

            // Power Slugs
            if (wc.power_slugs) {
                var ps = wc.power_slugs;
                var slugGrid = h('div', { class: 'card-grid' });
                slugGrid.appendChild(progressCard('Power Slugs', ps.collected, ps.total, ps.remaining + ' remaining'));
                if (ps.blue) slugGrid.appendChild(progressCard('Blue Slugs', ps.blue.collected, ps.blue.total, ps.blue.remaining + ' remaining', '#4a9eff'));
                if (ps.yellow) slugGrid.appendChild(progressCard('Yellow Slugs', ps.yellow.collected, ps.yellow.total, ps.yellow.remaining + ' remaining', '#f5a623'));
                if (ps.purple) slugGrid.appendChild(progressCard('Purple Slugs', ps.purple.collected, ps.purple.total, ps.purple.remaining + ' remaining', '#9b59b6'));
                ws.appendChild(slugGrid);
            }

            // Somersloops, Mercer Spheres, Crash Sites — combined in one row
            var otherGrid = h('div', { class: 'card-grid' });
            if (wc.somersloops) {
                var sl = wc.somersloops;
                otherGrid.appendChild(progressCard('Somersloops', sl.collected, sl.total, sl.remaining + ' remaining', '#e91e63'));
            }
            if (wc.mercer_spheres) {
                var ms = wc.mercer_spheres;
                otherGrid.appendChild(progressCard('Mercer Spheres', ms.collected, ms.total, ms.remaining + ' remaining', '#2ecc71'));
            }
            if (wc.crash_sites) {
                var cs = wc.crash_sites;
                var csSub = (cs.opened || 0) + ' opened, ' + (cs.unopened || 0) + ' unopened, ' + (cs.dismantled || 0) + ' dismantled';
                otherGrid.appendChild(progressCard('Crash Sites', cs.collected, cs.total, csSub, '#e67e22'));
            }
            if (otherGrid.children.length > 0) ws.appendChild(otherGrid);

            c.appendChild(ws);
        }

        var s1 = section('Other Collectibles');
        var grid = h('div', { class: 'card-grid' });
        grid.appendChild(card('Total Pickups', fmt(col.total_pickups || 0), 'pickup objects destroyed (collected by player)'));
        if (col.tapes) grid.appendChild(card('Tapes', fmt(col.tapes.total_collected || 0), 'collected'));

        // Item pickups summary cards
        var ip = col.item_pickups || {};
        if (ip.available_count) {
            grid.appendChild(card('Item Pickups', fmt(ip.available_count || 0), 'pickup objects still on map (not yet collected)'));
            grid.appendChild(card('Total Items', fmt(ip.total_items || 0), 'total items from remaining pickups'));
        }
        s1.appendChild(grid);
        c.appendChild(s1);

        // Item pickups table
        if (ip.by_type && Object.keys(ip.by_type).length > 0) {
            var s2 = section('Item Pickups by Type');
            var byType = ip.by_type;
            var types = Object.keys(byType).sort(function(a, b) {
                return (byType[b].total_items || 0) - (byType[a].total_items || 0);
            });

            types.forEach(function(itemName) {
                var info = byType[itemName];
                var det = h('details', {});
                det.appendChild(h('summary', { text: itemName + ' (' + info.count + ' pickups, ' + fmt(info.total_items) + ' items)' }));

                var rows = (info.pickups || []).map(function(p) {
                    var pos = p.position || {};
                    var locStr = '';
                    if (pos.X !== undefined) {
                        locStr = pos.X + ', ' + pos.Y + ' (alt: ' + pos.Z + 'm)';
                    }
                    return [itemName, fmt(p.num_items || 0), locStr];
                });

                det.appendChild(sortableTable(
                    [{label: 'Item', width: '25%'}, {label: 'Quantity', type: 'number', width: '15%'}, {label: 'Location', width: '60%'}],
                    rows,
                    {scrollable: true}
                ));
                s2.appendChild(det);
            });
            c.appendChild(s2);
        }

        // Tapes list
        if (col.tapes && col.tapes.collected && col.tapes.collected.length > 0) {
            var s3 = section('Collected Tapes');
            var rows = col.tapes.collected.map(function(tape) {
                return [tape.replace(/Tape_/, '').replace(/_/g, ' ')];
            });
            s3.appendChild(sortableTable([{label: 'Tape'}], rows));
            c.appendChild(s3);
        }
    }

    // ===== Extras =====

    function renderExtras() {
        var c = document.getElementById('tab-extras');
        c.innerHTML = '';
        var d = DATA;

        // --- MAM Research ---
        var gp = (d.analytics && d.analytics.gameProgression) || {};
        var mam = gp.mam;
        if (mam && mam.unlocked_count > 0) {
            var mamSection = section('MAM Research');
            var mamGrid = h('div', { class: 'card-grid' });
            mamGrid.appendChild(card('Research Activated', mam.is_activated ? 'Yes' : 'No', ''));
            mamGrid.appendChild(card('Unlocked Trees', fmt(mam.unlocked_count), 'research trees discovered'));
            mamSection.appendChild(mamGrid);

            var progress = mam.progress || {};
            var treeNames = Object.keys(progress).sort();
            if (treeNames.length > 0) {
                var progGrid = h('div', { class: 'card-grid' });
                treeNames.forEach(function(tree) {
                    var p = progress[tree] || {};
                    var completed = p.completed || 0;
                    var total = p.total || 0;
                    var pct = p.percentage || 0;
                    var color = pct >= 100 ? '#4caf50' : '#00d4ff';
                    progGrid.appendChild(progressCard(tree, completed, total, pct + '% complete', color));
                });
                mamSection.appendChild(progGrid);
            }
            c.appendChild(mamSection);
        }

        // --- Blueprints & Systems ---
        var blueprints = d.blueprints || {};
        var bpKeys = Object.keys(blueprints).filter(function(type) { return blueprints[type] > 0; });
        if (bpKeys.length > 0) {
            var bpTotal = 0;
            bpKeys.forEach(function(type) { bpTotal += blueprints[type]; });
            var bpSection = section('Blueprint Proxies & Game Subsystems (' + fmt(bpTotal) + ' total)');
            var bpRows = bpKeys.map(function(type) {
                var name = displayName(type);
                var desc = '';
                var lower = type.toLowerCase();
                if (lower.indexOf('blueprintproxy') >= 0) {
                    desc = 'Placed blueprints overall';
                } else if (lower.indexOf('blueprintshortcut') >= 0) {
                    desc = 'Blueprint shortcuts set in player UI';
                }
                return [name + (desc ? ' (' + desc + ')' : ''), fmt(blueprints[type]), type];
            }).sort(function(a, b) {
                return parseInt(b[1].replace(/,/g, '')) - parseInt(a[1].replace(/,/g, ''));
            });
            bpSection.appendChild(sortableTable(
                [{label: 'Type', width: '45%'}, {label: 'Count', type: 'number', width: '10%'}, {label: 'Type Path', cls: 'type-path-col', width: '45%'}],
                bpRows,
                {scrollable: bpRows.length > 20}
            ));
            c.appendChild(bpSection);
        }

        // --- Pets ---
        var pets = d.pets || {};
        if (pets.tamedDoggos > 0) {
            var petSection = section('Pets');
            var grid = h('div', { class: 'card-grid' });
            grid.appendChild(card('Lizard Doggos (Tamed)', fmt(pets.tamedDoggos || 0), 'tamed by players'));
            petSection.appendChild(grid);
            c.appendChild(petSection);
        }

        // Show a message if no extras data is available
        if (c.children.length === 0) {
            c.appendChild(h('p', { text: 'No extras data available for this save.' }));
        }
    }

    // ===== Tab System =====

    // Tabs visible per mode. QUICK = counts only, DEEP = full analysis.
    var QUICK_TABS = ['overview', 'structures', 'buildings', 'extras'];
    var FULL_TABS  = ['overview', 'structures', 'buildings', 'clockspeeds', 'power',
                      'production', 'manufacturing', 'storage', 'transport', 'map',
                      'nuclear', 'collectibles', 'extras'];

    function getVisibleTabs() {
        var mode = (DATA && DATA.mode) ? DATA.mode.toUpperCase() : 'DEEP';
        if (mode === 'QUICK') return QUICK_TABS;
        return FULL_TABS; // DEEP and any non-QUICK mode gets all tabs
    }

    var ALL_TABS = [
        {id: 'overview', label: 'Overview', render: renderOverview},
        {id: 'structures', label: 'Structures', render: renderStructures},
        {id: 'buildings', label: 'Buildings', render: renderBuildings},
        {id: 'clockspeeds', label: 'Clock Speeds', render: renderClockSpeeds},
        {id: 'power', label: 'Power', render: renderPower},
        {id: 'production', label: 'Production', render: renderProduction},
        {id: 'manufacturing', label: 'Manufacturing', render: renderManufacturing},
        {id: 'storage', label: 'Storage', render: renderStorage},
        {id: 'transport', label: 'Transport', render: renderTransport},
        {id: 'map', label: 'Map', render: renderMap},
        {id: 'nuclear', label: 'Nuclear', render: renderNuclear},
        {id: 'collectibles', label: 'Collectibles', render: renderCollectibles},
        {id: 'extras', label: 'Extras', render: renderExtras}
    ];

    var TABS = ALL_TABS;

    function setupTabs() {
        var nav = document.getElementById('tabs');
        var visible = getVisibleTabs();
        TABS = ALL_TABS.filter(function(tab) {
            return visible.indexOf(tab.id) >= 0;
        });
        TABS.forEach(function(tab) {
            var btn = h('button', { class: 'tab-btn', 'data-tab': tab.id, text: tab.label });
            btn.addEventListener('click', function() { switchTab(tab.id); });
            nav.appendChild(btn);
        });
    }

    function switchTab(tabId) {
        currentTab = tabId;
        document.querySelectorAll('.tab-btn').forEach(function(b) {
            b.classList.toggle('active', b.dataset.tab === tabId);
        });
        document.querySelectorAll('.tab-content').forEach(function(c) {
            c.classList.toggle('active', c.id === 'tab-' + tabId);
        });
        // Clear search when switching tabs
        var search = document.getElementById('search');
        if (search) search.value = '';
        filterRows('');
    }

    // ===== Search =====

    function setupSearch() {
        var input = document.getElementById('search');
        if (!input) return;
        input.addEventListener('input', function() {
            filterRows(input.value.toLowerCase());
        });
    }

    function setupExpertToggle() {
        var btn = document.getElementById('expert-toggle');
        if (!btn) return;
        btn.addEventListener('click', function() {
            document.body.classList.toggle('expert-mode');
            btn.classList.toggle('active');
        });
    }

    function filterRows(query) {
        var active = document.querySelector('.tab-content.active');
        if (!active) return;

        if (!query) {
            // Restore everything
            active.querySelectorAll('tbody tr').forEach(function(tr) { tr.style.display = ''; });
            active.querySelectorAll('div.section, details').forEach(function(el) { el.style.display = ''; });
            return;
        }

        // Filter table rows
        active.querySelectorAll('tbody tr').forEach(function(tr) {
            var text = tr.textContent.toLowerCase();
            tr.style.display = text.includes(query) ? '' : 'none';
        });

        // Hide sections (div.section) with no visible rows
        active.querySelectorAll('div.section').forEach(function(sec) {
            var hasVisible = false;
            // Check tables
            sec.querySelectorAll('table').forEach(function(tbl) {
                var rows = tbl.querySelectorAll('tbody tr');
                for (var i = 0; i < rows.length; i++) {
                    if (rows[i].style.display !== 'none') { hasVisible = true; break; }
                }
            });
            // Check card-grid content
            if (!hasVisible) {
                var cards = sec.querySelectorAll('.card-grid > *');
                for (var i = 0; i < cards.length; i++) {
                    if (cards[i].textContent.toLowerCase().includes(query)) { hasVisible = true; break; }
                }
            }
            // Check nested details with visible rows
            if (!hasVisible) {
                var dets = sec.querySelectorAll('details');
                for (var i = 0; i < dets.length; i++) {
                    var detRows = dets[i].querySelectorAll('tbody tr');
                    for (var j = 0; j < detRows.length; j++) {
                        if (detRows[j].style.display !== 'none') { hasVisible = true; break; }
                    }
                    if (hasVisible) break;
                }
            }
            sec.style.display = hasVisible ? '' : 'none';
        });

        // Hide standalone details elements with no visible rows
        active.querySelectorAll('details').forEach(function(det) {
            var rows = det.querySelectorAll('tbody tr');
            var hasVisible = false;
            for (var i = 0; i < rows.length; i++) {
                if (rows[i].style.display !== 'none') { hasVisible = true; break; }
            }
            // Also check summary text for non-table details (e.g. tapes)
            if (!hasVisible && rows.length === 0) {
                hasVisible = det.textContent.toLowerCase().includes(query);
            }
            det.style.display = hasVisible ? '' : 'none';
        });
    }

    // ===== Empty Tab/Section Hiding =====

    function hideEmptyTabs() {
        TABS.forEach(function(tab) {
            if (tab.id === 'overview') return;
            var content = document.getElementById('tab-' + tab.id);
            if (!content) return;
            var sections = content.querySelectorAll('.section');
            var hasContent = false;
            sections.forEach(function(sec) {
                // Check for tables with rows
                if (sec.querySelector('table tbody tr')) hasContent = true;
                // Check for cards with non-zero values
                if (!hasContent) {
                    var cards = sec.querySelectorAll('.card-grid > .card');
                    for (var i = 0; i < cards.length; i++) {
                        var val = cards[i].querySelector('.value');
                        if (val) {
                            var text = val.textContent.trim();
                            // Skip cards showing only zeros or empty
                            if (text && text !== '0' && text !== '0 MW' && text !== '0 km' && text !== '0 / 0') {
                                hasContent = true;
                                break;
                            }
                        }
                    }
                }
                // Check for details with content
                if (!hasContent && sec.querySelector('details')) hasContent = true;
                // Check for progress bars with content
                if (!hasContent && sec.querySelector('.bar-container .bar-fill[style*="width"]')) {
                    var fills = sec.querySelectorAll('.bar-fill');
                    for (var i = 0; i < fills.length; i++) {
                        var w = fills[i].style.width;
                        if (w && w !== '0%') { hasContent = true; break; }
                    }
                }
            });
            if (!hasContent) {
                var btn = document.querySelector('.tab-btn[data-tab="' + tab.id + '"]');
                if (btn) btn.style.display = 'none';
            }
        });
    }

    function hideEmptySections(container) {
        if (!container) return;
        container.querySelectorAll('.section').forEach(function(sec) {
            var hasContent = false;
            if (sec.querySelector('table tbody tr')) hasContent = true;
            if (!hasContent) {
                var cards = sec.querySelectorAll('.card-grid > .card');
                for (var i = 0; i < cards.length; i++) {
                    var val = cards[i].querySelector('.value');
                    if (val) {
                        var text = val.textContent.trim();
                        if (text && text !== '0' && text !== '0 MW' && text !== '0 km' && text !== '0 / 0') {
                            hasContent = true;
                            break;
                        }
                    }
                }
            }
            if (!hasContent && sec.querySelector('details')) hasContent = true;
            if (!hasContent) sec.style.display = 'none';
        });
    }

    // ===== Init =====

    async function init() {
        var loading = document.getElementById('loading');
        var app = document.getElementById('app');
        try {
            DATA = await loadData();
            loading.style.display = 'none';
            app.style.display = 'block';

            // Set title
            var hdr = DATA.header || {};
            var title = hdr.SaveName || hdr.SessionName || 'Save Report';
            document.getElementById('title').textContent = title;

            // Setup tabs
            setupTabs();

            // Render all tabs
            TABS.forEach(function(tab) { tab.render(); });

            // Hide empty tabs and sections
            hideEmptyTabs();
            TABS.forEach(function(tab) {
                hideEmptySections(document.getElementById('tab-' + tab.id));
            });

            // Activate first tab
            switchTab('overview');

            // Setup search
            setupSearch();

            // Setup expert mode toggle
            setupExpertToggle();
        } catch (err) {
            loading.style.display = 'block';
            loading.innerHTML = '<p style="color: #ff6464; font-size: 1.2em;">Error loading data: ' + esc(err.message) + '</p>';
            console.error('SatisFacts:', err);
        }
    }

    init();
})();
