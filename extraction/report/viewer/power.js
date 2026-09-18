    function renderPower() {
        var c = document.getElementById('tab-power');
        c.innerHTML = '';
        var d = DATA;
        var pw = (d.analytics && d.analytics.power) || {};
        var gen = pw.generators || {};
        var con = pw.consumers || {};
        var bal = pw.power_balance || {};
        var storage = pw.power_storage || {};
        var apa = pw.apa || {};
        var loco = pw.locomotives || {};
        var gp = (d.extraction && d.extraction.GeneratorPower) || {};
        var pc = (d.extraction && d.extraction.PowerConsumption) || {};

        // Skip the entire Power tab when there is no power-related data at all
        // (fresh save with no generators, no consumers, no power grid).
        var hasGenerators = (gen.by_type && Object.keys(gen.by_type).length > 0) || (gp.Generators && gp.Generators.length > 0);
        var hasConsumers = (pc.Consumers && pc.Consumers.length > 0);
        var hasBalance = (bal.capacity_mw && bal.capacity_mw > 0) || (bal.consumption_mw && bal.consumption_mw > 0);
        var pgd = d.powerGridData;
        var hasGrid = pgd && (pgd.Circuits || pgd.circuits) && (pgd.Circuits || pgd.circuits).length > 0;
        if (!hasGenerators && !hasConsumers && !hasBalance && !hasGrid) return;

        c.appendChild(tabDescription(t('desc.tab_power')));

        // Summary cards + APA info side-by-side
        var s1 = section(t('section.power_balance'));
        s1.classList.add('card-section');
        var grid = cardGrid(3);
        // APA (Alien Power Augmenter) adds a multiplier to total grid capacity.
        // Each fueled APA adds +30%, unfueled adds +10%. The multiplier applies
        // to the sum of running generator capacity + APA base power (500 MW each).
        var surplus = bal.surplus_mw || 0;
        var genCap = bal.generator_capacity_mw || bal.capacity_mw || 0;
        var apaBase = bal.apa_base_power_mw || 0;
        var apaMult = bal.apa_multiplier || 1;
        var totalCap = bal.capacity_mw || genCap;
        var genSubtext = t('desc.running_generators', (gen.active_generators || 0));
        if (apaBase > 0) genSubtext += ' + ' + fmtMw(apaBase) + ' ' + t('label.apa_base');
        grid.appendChild(card(t('card.generation'), fmtMw(totalCap), genSubtext, '#22c55e'));
        var theoMax = bal.theoretical_max_mw || 0;
        if (theoMax > 0) {
            var theoAug = (theoMax + apaBase) * apaMult;
            grid.appendChild(card(t('card.max_theoretical'), fmtMw(theoAug), t('desc.all_generators_apa', apaMult.toFixed(1)), '#f59e0b'));
        }
        var locoMW = bal.locomotive_consumption_mw || 0;
        var consumptionSub = t('desc.active_buildings', (con.active_buildings || 0));
        if (locoMW > 0) {
            consumptionSub += ' + ' + fmtMw(locoMW) + ' ' + t('label.rail_traction');
        }
        var theoCons = con.total_theoretical_mw || 0;
        if (theoCons > 0) {
            consumptionSub += ' \u00B7 ' + t('col.max') + ': ' + fmtMw(theoCons);
        }
        grid.appendChild(card(t('card.consumption'), fmtMw(bal.consumption_mw), consumptionSub, '#ef4444'));
        grid.appendChild(card(surplus >= 0 ? t('card.surplus') : t('card.deficit'), (surplus >= 0 ? '+' : '') + fmtMw(surplus), surplus >= 0 ? t('desc.power_available') : t('desc.not_enough_power'), surplus >= 0 ? '#22c55e' : '#ef4444'));
        if (storage.capacity_mwh) {
            var storageSubtext = (storage.count || 0) + ' ' + t('label.units') + ' \u00B7 ' + fmtMw(storage.max_charge_rate_mw || 0) + ' ' + t('label.max_charge');
            if (storage.stored_mwh !== undefined) {
                storageSubtext = fmt(storage.stored_mwh) + ' / ' + fmt(storage.capacity_mwh) + ' ' + t('label.mwh_stored') + ' \u00B7 ' + (storage.count || 0) + ' ' + t('label.units');
            }
            grid.appendChild(card(t('card.power_storage'), fmt(storage.capacity_mwh) + ' MWh', storageSubtext, '#a855f7'));
        }
        if (apa.building_count) grid.appendChild(card(t('card.apa'), apa.building_count + ' ' + t('label.installed'), (apa.fueled_count || 0) + ' ' + t('label.fueled'), '#eab308'));
        // APA info box sits next to the balance cards, explaining the multiplier
        // breakdown. Only shown when APAs are present in the save.
        var apaBox = null;
        if (apa.building_count && apa.building_count > 0) {
            apaBox = h('div', {
                style: 'padding: 12px 14px; background: rgba(255, 200, 0, 0.12); border-left: 4px solid #ffc800; border-radius: 6px; width: 540px; flex-shrink: 0; align-self: center;'
            });
            var apaHeader = h('div', { style: 'display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;' });
            apaHeader.appendChild(h('strong', { style: 'font-size: 1.1em;', text: t('label.apa_full') }));
            var boostPct = ((apaMult - 1) * 100).toFixed(1);
            apaHeader.appendChild(h('span', { style: 'color: #ffc800; font-size: 1.2em; font-weight: bold;', text: '+' + boostPct + '% ' + t('label.grid_boost') }));
            apaBox.appendChild(apaHeader);

            var fueled = apa.fueled_count || 0;
            var unfueled = apa.unfueled_count || 0;
            var apaDetail = h('div', { style: 'font-size: 0.95em; margin-bottom: 8px;' });
            apaDetail.appendChild(h('strong', { text: apa.building_count + ' ' + t('label.apas') }));
            apaDetail.appendChild(h('span', { text: ' ' + t('label.installed_colon') + ' ' }));
            if (fueled > 0) apaDetail.appendChild(h('span', { style: 'color: #00ff00;', text: fueled + ' ' + t('label.fueled_30') }));
            if (fueled > 0 && unfueled > 0) apaDetail.appendChild(h('span', { text: ', ' }));
            if (unfueled > 0) apaDetail.appendChild(h('span', { style: 'color: #ffa500;', text: unfueled + ' ' + t('label.unfueled_10') }));
            apaBox.appendChild(apaDetail);

            var apaStats = h('div', { style: 'font-size: 0.9em; color: #aaa;' });
            apaStats.appendChild(h('div', { text: t('label.base_apa_power') + ': ' + fmtMw(apaBase) + ' (' + t('label.500mw_per_apa') + ')' }));
            apaStats.appendChild(h('div', { text: t('label.running_gen_capacity') + ': ' + fmtMw(gen.active_capacity_mw || 0) }));
            apaStats.appendChild(h('div', { style: 'margin-top: 4px;', text: t('label.total_augmented') + ': ' }));
            var augStrong = h('strong', { style: 'color: #ffc800;', text: fmtMw(apa.augmented_capacity_mw || totalCap) + ' (\u00D7' + apaMult.toFixed(1) + ' ' + t('label.multiplier') + ')' });
            apaStats.appendChild(augStrong);
            var maxAug = apa.max_augmented_capacity_mw || 0;
            if (maxAug > (apa.augmented_capacity_mw || 0)) {
                apaStats.appendChild(h('div', { style: 'margin-top: 6px; padding-top: 6px; border-top: 1px solid rgba(255, 200, 0, 0.2); color: #888;' }));
                apaStats.appendChild(h('div', { text: t('label.max_theoretical_all') + ':' }));
                apaStats.appendChild(h('strong', { style: 'color: #888;', text: fmtMw(maxAug) + ' (\u00D7' + apaMult.toFixed(1) + ' ' + t('label.multiplier') + ')' }));
                apaStats.appendChild(h('div', { style: 'font-size: 0.85em; color: #666; margin-top: 2px;', text: t('apa.based_on', fmtMw(bal.theoretical_max_mw || 0)) }));
            }
            apaBox.appendChild(apaStats);

            if (unfueled > 0) {
                var apaTip = h('div', { style: 'margin-top: 8px; padding-top: 8px; border-top: 1px solid rgba(255, 200, 0, 0.3); font-size: 0.85em; color: #aaa; white-space: nowrap;' });
                apaTip.appendChild(h('span', { text: t('apa.tip') }));
                apaBox.appendChild(apaTip);
            }
        }

        // Place card grid + APA box side-by-side inside the section
        if (apaBox) {
            s1.classList.add('sync-flex-header');
            var balanceRow = h('div', { style: 'display: flex; flex-wrap: wrap; gap: 16px; align-items: flex-start; justify-content: center; width: fit-content; max-width: 100%; margin: 0 auto;' });
            balanceRow.appendChild(grid);
            balanceRow.appendChild(apaBox);
            s1.appendChild(balanceRow);
        } else {
            s1.appendChild(grid);
        }
        c.appendChild(s1);

        // Generator types breakdown
        if (gen.by_type && Object.keys(gen.by_type).length > 0) {
            var s2 = section(t('section.generator_types'));
            var rows = Object.keys(gen.by_type).map(function(type) {
                var g = gen.by_type[type];
                // by_type is keyed by class path (Build_GeneratorCoal_C) but the
                // Individual Generators table below gets a short label ("Coal").
                // strip the "Generator " prefix so both tables agree - the
                // heading already says these are generators.
                var label = displayName(type).replace(/^Generator /, '');
                var icon = iconFor(type);
                var labelCell = icon ? html('<img class="row-icon" src="' + icon + '" alt=""> ' + esc(label)) : label;
                return [labelCell, g.count, g.active, g.standby, fmtMw(g.total_mw), fmtMw(g.active_mw)];
            }).sort(byNumDesc(4));

            // Geothermal info button (only when geothermal generators exist)
            var geoEntry = null;
            for (var gt in gen.by_type) {
                if (gt.toLowerCase().indexOf('geothermal') !== -1) {
                    geoEntry = gen.by_type[gt];
                    break;
                }
            }
            if (geoEntry && geoEntry.geo_min_mw !== undefined) {
                var geoBtn = h('button', { class: 'loc-toggle-btn', title: t('btn.info_geothermal'), style: 'color: #f97316; font-weight: bold; margin-left: 10px;' }, t('btn.info_geothermal'));
                s2.querySelector('h2').appendChild(geoBtn);
                var geoPopover = null;
                geoBtn.addEventListener('click', function(e) {
                    e.stopPropagation();
                    if (geoPopover) {
                        geoPopover.remove();
                        geoPopover = null;
                        return;
                    }
                    geoPopover = h('div', {
                        style: 'position: fixed; z-index: 1000; padding: 14px 16px; background: #1a2332; border-left: 4px solid #f97316; border-radius: 6px; box-shadow: 0 8px 24px rgba(0,0,0,0.5); max-width: 480px; font-size: 0.9em; line-height: 1.5;'
                    });
                    geoPopover.appendChild(h('div', { style: 'margin-bottom: 8px;' },
                        h('strong', { style: 'color: #f97316; font-size: 1.05em;', text: t('geo.fluctuation_title') })));
                    geoPopover.appendChild(h('div', { style: 'color: #ccc; margin-bottom: 6px;',
                        text: t('geo.fluctuation_explanation') }));
                    var purities = geoEntry.purities || {};
                    var purityLines = [];
                    if (purities.impure) purityLines.push(t('geo.impure_range', purities.impure));
                    if (purities.normal) purityLines.push(t('geo.normal_range', purities.normal));
                    if (purities.pure) purityLines.push(t('geo.pure_range', purities.pure));
                    if (purityLines.length > 0) {
                        geoPopover.appendChild(h('div', { style: 'color: #ccc; margin-bottom: 6px;', text: t('geo.your_generators') }));
                        purityLines.forEach(function(line) {
                            geoPopover.appendChild(h('div', { style: 'color: #ccc; margin-bottom: 3px; padding-left: 12px;', text: '\u2022 ' + line }));
                        });
                    }
                    geoPopover.appendChild(h('div', { style: 'color: #ccc; margin-top: 6px; margin-bottom: 6px;',
                        text: t('geo.total_range', fmtMw(geoEntry.geo_min_mw), fmtMw(geoEntry.geo_max_mw), fmtMw(geoEntry.geo_avg_mw)) }));
                    geoPopover.appendChild(h('div', { style: 'color: #fbbf24;',
                        text: t('geo.fluctuation_warn', fmtMw(geoEntry.total_mw)) }));
                    var rect = geoBtn.getBoundingClientRect();
                    geoPopover.style.top = (rect.bottom + 8) + 'px';
                    geoPopover.style.right = (window.innerWidth - rect.right) + 'px';
                    document.body.appendChild(geoPopover);
                    setTimeout(function() {
                        var closeHandler = function(ev) {
                            if (geoPopover && !geoPopover.contains(ev.target) && ev.target !== geoBtn) {
                                geoPopover.remove();
                                geoPopover = null;
                                document.removeEventListener('click', closeHandler);
                            }
                        };
                        document.addEventListener('click', closeHandler);
                    }, 0);
                });
            }

            s2.classList.add('auto-width');
            s2.appendChild(sortableTable(
                // 175px fits the longest label, "Integrated Biomass". careful
                // with these widths: td is overflow:hidden + ellipsis, so a
                // narrow column truncates silently instead of growing.
                [{label: t('col.type'), width: '175px'},
                {label: t('col.total'), type: 'number', width: '60px'},
                {label: t('col.active'), type: 'number', width: '60px'},
                {label: t('col.standby'), type: 'number', width: '80px'},
                {label: t('col.capacity'), type: 'number', width: '100px'},
                {label: t('col.active_mw'), type: 'number', width: '120px'}],
                rows,
                {autoWidth: true}
            ));
            c.appendChild(s2);
        }

        // Side-by-side container for generators + consumers
        var genConWrap = wrapRow();
        var hasGenCon = false;

        // Individual generators (paginated for performance with large saves)
        if (gp.Generators && gp.Generators.length > 0) {
            var s3 = paginatedTable(gp.Generators,
                [{label: t('col.type'), width: '190px'},
                {label: t('col.status'), width: '90px'},
                {label: t('col.actual'), type: 'number', width: '90px'},
                {label: t('col.max'), type: 'number', width: '140px'},
                {label: t('col.reason'), width: '200px'},
                {label: '', width: '40px'}],
                {
                    sectionTitle: t('section.individual_generators') + ' (' + gp.Generators.length + ')',
                    autoWidth: true,
                    filterPlaceholder: t('filter.by_type_status_reason'),
                    scrollable: true,
                    filterCols: [0, 4],
                    extraFilters: [{
                        allLabel: t('filter.all_statuses'),
                        values: ['Running', 'Standby'],
                        labels: [translateStatus('Running'), translateStatus('Standby')],
                        col: 1,
                        exact: true
                    }],
                    rowBuilder: function(g) {
                        var statusColor = g.Status === 'Running' ? 'green' : (g.Status === 'Standby' ? 'orange' : 'red');
                        return [html(iconName(g.TypePath)), {__el: badge(translateStatus(g.Status), statusColor)}, fmtMw(g.ActualMW), fmtMw(g.TheoreticalMaxMW || g.MaxMW), translateReason(g.Reason), mapLink(g.Location)];
                    }
                });
            genConWrap.appendChild(s3);
            hasGenCon = true;
        }

        // Individual consumers (paginated for performance with large saves)
        if (pc.Consumers && pc.Consumers.length > 0) {
            // Locomotive power info button (only when trains exist).
            // Opens a click-to-toggle popover anchored below the button.
            var extraBtns = [];
            if (loco.count > 0) {
                var locoBtn = h('button', { class: 'loc-toggle-btn', title: t('btn.info_locomotive_power'), style: 'color: #06b6d4; font-weight: bold;' }, t('btn.info_locomotive_power'));
                var locoPopover = null;
                locoBtn.addEventListener('click', function(e) {
                    e.stopPropagation();
                    if (locoPopover) {
                        locoPopover.remove();
                        locoPopover = null;
                        return;
                    }
                    locoPopover = h('div', {
                        style: 'position: absolute; z-index: 1000; padding: 14px 16px; background: #1a2332; border-left: 4px solid #06b6d4; border-radius: 6px; box-shadow: 0 8px 24px rgba(0,0,0,0.5); max-width: 520px; font-size: 0.9em; line-height: 1.5;'
                    });
                    var locoBase = loco.base_mw || 25;
                    var locoMax = loco.max_draw_mw || 110;
                    locoPopover.appendChild(h('div', { style: 'margin-bottom: 8px;' },
                        h('strong', { style: 'color: #06b6d4; font-size: 1.05em;', text: t('loco.title', loco.count, fmtMw(locoBase), fmtMw(loco.counted_mw || 0)) })));
                    locoPopover.appendChild(h('div', { style: 'color: #ccc; margin-bottom: 6px;',
                        text: t('loco.no_power_data') }));
                    locoPopover.appendChild(h('div', { style: 'color: #ccc; margin-bottom: 6px;',
                        text: t('loco.min_power', fmtMw(locoBase), fmtMw(locoMax)) }));
                    locoPopover.appendChild(h('div', { style: 'color: #fbbf24;',
                        text: t('loco.real_higher_warn', fmtMw(loco.upper_est_mw || 0)) }));
                    // Position below the button, anchored to the right edge so
                    // the popover opens leftward (avoids cutoff on small monitors)
                    var rect = locoBtn.getBoundingClientRect();
                    locoPopover.style.position = 'fixed';
                    locoPopover.style.top = (rect.bottom + 8) + 'px';
                    locoPopover.style.right = (window.innerWidth - rect.right) + 'px';
                    document.body.appendChild(locoPopover);
                    // Close on outside click
                    setTimeout(function() {
                        var closeHandler = function(ev) {
                            if (locoPopover && !locoPopover.contains(ev.target) && ev.target !== locoBtn) {
                                locoPopover.remove();
                                locoPopover = null;
                                document.removeEventListener('click', closeHandler);
                            }
                        };
                        document.addEventListener('click', closeHandler);
                    }, 0);
                });
                extraBtns.push(locoBtn);
            }

            var s4 = paginatedTable(pc.Consumers,
                [{label: t('col.type'), width: '190px'},
                {label: t('col.status'), width: '90px'},
                {label: t('col.actual'), type: 'number', width: '90px'},
                {label: t('col.max'), type: 'number', width: '140px'},
                {label: t('col.productivity'), type: 'number', width: '90px'},
                {label: t('col.reason'), width: '200px'},
                {label: '', width: '40px'}],
                {
                    sectionTitle: t('section.power_consumers') + ' (' + pc.Consumers.length + ')',
                    autoWidth: true,
                    filterPlaceholder: t('filter.by_type_status_reason'),
                    scrollable: true,
                    filterCols: [0, 5],
                    extraFilters: [{
                        allLabel: t('filter.all_statuses'),
                        values: ['Running', 'Idle', 'Paused', 'Blocked', 'Starving', 'Standby', 'Unpowered', 'Unknown'],
                        labels: ['Running', 'Idle', 'Paused', 'Blocked', 'Starving', 'Standby', 'Unpowered', 'Unknown'].map(translateStatus),
                        col: 1,
                        exact: true
                    }],
                    extraButtons: extraBtns,
                    rowBuilder: function(c) {
                        var statusColor = ({
                            'Running': 'green',
                            'Idle': 'blue',
                            'Paused': 'orange',
                            'Starving': 'yellow',
                            'Blocked': 'red',
                            'Standby': 'orange',
                            'Unpowered': 'red'
                        })[c.Status] || 'gray';
                        var prodCell = '';
                        if (c.Productivity > 0) {
                            var prodColor = c.Productivity >= 99 ? 'green' : (c.Productivity >= 50 ? 'orange' : 'red');
                            prodCell = '<span style="color:' + prodColor + ';">' + c.Productivity.toFixed(1) + '%</span>';
                        }
                        return [html(iconName(c.TypePath)), {__el: badge(translateStatus(c.Status), statusColor)}, fmtMw(c.ActualMW), fmtMw(c.MaxMW), prodCell ? html(prodCell) : '', translateReason(c.Reason), mapLink(c.Location)];
                    }
                });
            genConWrap.appendChild(s4);
            hasGenCon = true;
        }

        if (hasGenCon) c.appendChild(genConWrap);

        // Power Grid Topology
        if (pgd && (pgd.Circuits || pgd.circuits) && (pgd.Circuits || pgd.circuits).length > 0) {
            var circuits = pgd.Circuits || pgd.circuits;
            var s5 = section(t('section.power_grid'));
            s5.classList.add('card-section');

            // Summary stats grid
            var gridStats = cardGrid();
            gridStats.appendChild(card(t('card.circuits'), fmt(circuits.length), t('desc.power_networks'), '#00d4ff'));
            gridStats.appendChild(card(t('card.components'), fmt(pgd.ComponentCount || pgd.componentCount || 0), '', '#3b82f6'));
            var prod = d.production || {};
            var powerLineKm = 0;
            if (prod.powerLines && prod.powerLines.length > 0) {
                powerLineKm = prod.powerLines.reduce(function(a, b) { return a + b; }, 0);
            }
            gridStats.appendChild(card(t('card.power_lines'), fmt(pgd.LineCount || pgd.lineCount || 0), '', '#f59e0b'));
            if (powerLineKm > 0) {
                gridStats.appendChild(card(t('card.power_lines_length'), fmtKm(powerLineKm), '', '#f59e0b'));
            }

            // Count power switches from structures
            var switchCount = 0;
            if (d.structures && d.structures.details && d.structures.details.powerSwitches) {
                var sw = d.structures.details.powerSwitches;
                for (var swType in sw) {
                    if (sw.hasOwnProperty(swType)) switchCount += sw[swType];
                }
            }
            if (switchCount > 0) {
                gridStats.appendChild(card(t('card.switches'), fmt(switchCount), t('desc.power_switches'), '#f59e0b'));
            }
            s5.appendChild(gridStats);

            // Side-by-side: circuit table + pole/switch breakdown
            var fr = wrapRow();

            // Circuit load badge: >500 components is a warning (potential
            // brownout risk), >200 is moderate, below is healthy
            var circuitSection = section(t('section.circuits'));
            circuitSection.classList.add('auto-width');

            // Info button explaining what a circuit is
            var circuitInfoBtn = h('button', { class: 'loc-toggle-btn', title: t('circuit.title'), style: 'color: #00d4ff; font-weight: bold; margin-left: 10px;' }, t('btn.info_circuit'));
            circuitSection.querySelector('h2').appendChild(circuitInfoBtn);
            var circuitPopover = null;
            circuitInfoBtn.addEventListener('click', function(e) {
                e.stopPropagation();
                if (circuitPopover) {
                    circuitPopover.remove();
                    circuitPopover = null;
                    return;
                }
                circuitPopover = h('div', {
                    style: 'position: fixed; z-index: 1000; padding: 14px 16px; background: #1a2332; border-left: 4px solid #00d4ff; border-radius: 6px; box-shadow: 0 8px 24px rgba(0,0,0,0.5); max-width: 480px; font-size: 0.9em; line-height: 1.5;'
                });
                circuitPopover.appendChild(h('div', { style: 'margin-bottom: 8px;' },
                    h('strong', { style: 'color: #00d4ff; font-size: 1.05em;', text: t('circuit.title') })));
                circuitPopover.appendChild(h('div', { style: 'color: #ccc; margin-bottom: 6px;',
                    text: t('circuit.explanation') }));
                circuitPopover.appendChild(h('div', { style: 'color: #ccc; margin-bottom: 6px;',
                    text: t('circuit.component_count') }));
                circuitPopover.appendChild(h('div', { style: 'color: #ccc; margin-bottom: 4px; padding-left: 12px;',
                    text: t('circuit.components_1') }));
                circuitPopover.appendChild(h('div', { style: 'color: #ccc; margin-bottom: 4px; padding-left: 12px;',
                    text: t('circuit.components_2') }));
                circuitPopover.appendChild(h('div', { style: 'color: #ccc; margin-bottom: 4px; padding-left: 12px;',
                    text: t('circuit.components_3') }));
                circuitPopover.appendChild(h('div', { style: 'color: #ccc; margin-bottom: 4px; padding-left: 12px;',
                    text: t('circuit.components_4') }));
                circuitPopover.appendChild(h('div', { style: 'color: #ccc; margin-bottom: 6px; padding-left: 12px;',
                    text: t('circuit.components_5') }));
                circuitPopover.appendChild(h('div', { style: 'color: #fbbf24;',
                    text: t('circuit.dead_warn') }));
                var rect = circuitInfoBtn.getBoundingClientRect();
                circuitPopover.style.top = (rect.bottom + 8) + 'px';
                circuitPopover.style.right = (window.innerWidth - rect.right) + 'px';
                document.body.appendChild(circuitPopover);
                setTimeout(function() {
                    var closeHandler = function(ev) {
                        if (circuitPopover && !circuitPopover.contains(ev.target) && ev.target !== circuitInfoBtn) {
                            circuitPopover.remove();
                            circuitPopover = null;
                            document.removeEventListener('click', closeHandler);
                        }
                    };
                    document.addEventListener('click', closeHandler);
                }, 0);
            });

            var sorted = circuits.slice().sort(function(a, b) {
                return (b.ComponentCount || b.componentCount || 0) - (a.ComponentCount || a.componentCount || 0);
            });
            var circuitRows = sorted.map(function(circuit, i) {
                var cc = circuit.ComponentCount || circuit.componentCount || 0;
                var badgeClass = cc > 500 ? 'red' : (cc > 200 ? 'yellow' : 'green');
                return [t('label.circuit_num', (i + 1)), {__el: badge(cc + ' ' + t('label.components'), badgeClass)}];
            });
            circuitSection.appendChild(sortableTable(
                [{label: t('col.circuit'), width: '120px'},
                {label: t('col.load'), width: '200px'}],
                circuitRows,
                {scrollable: true, autoWidth: true}
            ));
            fr.appendChild(circuitSection);

            // Power poles & switches breakdown (right)
            var poleSection = section(t('section.power_poles_switches'));
            poleSection.classList.add('auto-width');
            var poleRows = [];
            if (prod.powerPoleTypes) {
                Object.keys(prod.powerPoleTypes).sort().forEach(function(type) {
                    poleRows.push([html(iconName(type)), prod.powerPoleTypes[type]]);
                });
            }
            if (d.structures && d.structures.details && d.structures.details.powerSwitches) {
                var swDetails = d.structures.details.powerSwitches;
                Object.keys(swDetails).sort().forEach(function(type) {
                    poleRows.push([html(iconName(type)), swDetails[type]]);
                });
            }
            if (poleRows.length > 0) {
                poleSection.appendChild(sortableTable(
                    [{label: t('col.type'), width: '280px'},
                    {label: t('col.count'), type: 'number', width: '80px'}],
                    poleRows,
                    {autoWidth: true}
                ));
            } else {
                poleSection.appendChild(emptyState(t('empty.no_power_poles')));
            }
            fr.appendChild(poleSection);

            s5.appendChild(fr);
            c.appendChild(s5);
        }

    }