    function renderNuclear() {
        var c = document.getElementById('tab-nuclear');
        c.innerHTML = '';
        var d = DATA;
        c.appendChild(tabDescription(t('desc.tab_nuclear')));
        var nw = (d.analytics && d.analytics.nuclearWaste) || {};
        var TAB_COLOR = '#9ca3af';

        if (!nw || !nw.plant_count) {
            c.appendChild(section(t('section.nuclear_power')));
            c.appendChild(emptyState(t('empty.no_nuclear')));
            return;
        }

        // Nuclear tab uses gray for summary cards, but fuel type
        // cards keep 3 green shades: Uranium=bright, Plutonium=light, Ficsonium=dark
        var s1 = section(t('section.nuclear_overview') + ' ' + fmtMw(nw.total_power_output || 0));
        s1.classList.add('card-section');
        var grid = cardGrid(5);
        grid.appendChild(card(t('card.nuclear_plants'), nw.plant_count || 0, '', TAB_COLOR));
        grid.appendChild(card(t('card.avg_clock_speed'), (nw.average_clock_speed || 0).toFixed(1) + '%', '', TAB_COLOR));
        grid.appendChild(card(t('card.waste_production'), (nw.total_waste_production || 0).toFixed(1) + ' /min', '', TAB_COLOR));
        grid.appendChild(card(t('card.stored_waste'), fmt(nw.total_stored_waste || 0), t('desc.containers', (nw.waste_containers || 0)), TAB_COLOR));
        s1.appendChild(grid);
        c.appendChild(s1);

        // Fuel type breakdown
        var ftc = nw.fuel_type_counts || {};
        if (Object.keys(ftc).length > 0) {
            var s2 = section(t('section.fuel_types'));
            s2.classList.add('card-section');
            var grid2 = cardGrid(5);
            var fuelColors = { 'Uranium': '#00ff00', 'Plutonium': '#4ade80', 'Ficsonium': '#15803d' };
            Object.keys(ftc).sort().forEach(function(ft) {
                grid2.appendChild(card(ft, ftc[ft] === 1 ? t('desc.plant_singular', ftc[ft]) : t('desc.plant_plural', ftc[ft]), '', fuelColors[ft] || '#888'));
            });
            s2.appendChild(grid2);
            c.appendChild(s2);
        }

        // Per-plant details (paginated for performance with large saves)
        if (nw.plants && nw.plants.length > 0) {
            var plantsWithIdx = nw.plants.map(function(p, i) { return { plant: p, origIdx: i }; });
            var s3 = paginatedTable(plantsWithIdx,
                [{label: t('col.plant'), width: '80px'},
                {label: t('col.fuel_type'), width: '120px'},
                {label: t('col.clock_speed'), type: 'number', width: '100px'},
                {label: t('col.power_output'), type: 'number', width: '120px'},
                {label: t('col.waste_production'), type: 'number', width: '150px'},
                {label: '', width: '40px'}],
                {
                    sectionTitle: t('section.nuclear_plants') + ' (' + nw.plants.length + ')',
                    autoWidth: true,
                    filterPlaceholder: t('filter.by_fuel_type'),
                    scrollable: true,
                    filterCols: [1],
                    rowBuilder: function(item) {
                        var p = item.plant;
                        var fuelColor = { 'Uranium': '#00ff00', 'Plutonium': '#4ade80', 'Ficsonium': '#15803d' }[p.fuelType] || '#888';
                        var fuelBadge = h('span', { class: 'badge', style: 'background: ' + fuelColor + '22; color: ' + fuelColor + ';', text: p.fuelType || t('status.unknown') });
                        return [
                            '#' + (item.origIdx + 1),
                            { __el: fuelBadge },
                            (p.clockSpeed || 0).toFixed(1) + '%',
                            fmtMw(p.powerOutput || 0),
                            t('label.waste_min', (p.wasteProductionRate || 0).toFixed(1)),
                            mapLink(p.location)
                        ];
                    }
                });
            c.appendChild(s3);
        }
    }