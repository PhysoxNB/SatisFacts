    function renderManufacturing() {
        var c = document.getElementById('tab-manufacturing');
        c.innerHTML = '';
        var d = DATA;
        c.appendChild(tabDescription(t('desc.tab_manufacturing')));
        var mfg = (d.analytics && d.analytics.manufacturing) || {};
        var TAB_COLOR = '#9ca3af';

        // Sum up slooped buildings across all recipes for the summary card.
        // Each building entry has a 'slooped' field = number of sloop slots used.
        var sloopedCount = 0, totalSloops = 0;
        if (mfg.active_production) {
            Object.keys(mfg.active_production).forEach(function(recipeKey) {
                var r = mfg.active_production[recipeKey];
                if (r.buildings) {
                    r.buildings.forEach(function(b) {
                        if (b.slooped) {
                            sloopedCount++;
                            totalSloops += b.slooped;
                        }
                    });
                }
            });
        }

        // Summary cards
        var s1 = section(t('section.manufacturing_summary'));
        s1.classList.add('card-section');
        var grid = cardGrid(5);
        grid.appendChild(card(t('card.active_buildings'), fmt(mfg.active_count || 0), '', TAB_COLOR));
        grid.appendChild(card(t('card.total_buildings_mfg'), fmt(mfg.total_manufacturing_buildings || 0), '', TAB_COLOR));
        grid.appendChild(card(t('card.recipes_in_use'), fmt(mfg.total_recipes_in_use || 0), '', TAB_COLOR));
        grid.appendChild(card(t('card.slooped_buildings'), fmt(sloopedCount), t('desc.sloops_total', fmt(totalSloops)), '#c084fc'));
        s1.appendChild(grid);
        c.appendChild(s1);

        // Recipe production table
        if (mfg.active_production && Object.keys(mfg.active_production).length > 0) {
            var s2 = section(t('section.production_by_recipe'));
            s2.classList.add('sync-details-header');
            var detWrap = wrapRow();
            Object.keys(mfg.active_production).forEach(function(recipeKey) {
                var r = mfg.active_production[recipeKey];
                // Efficiency can exceed 100% when overclocked + slooped.
                // Bar scale is 200% to accommodate overclocked setups.
                var effPct = r.efficiency || 0;
                var color = effPct > 100 ? '#ff6464' : (effPct > 80 ? '#00ff00' : (effPct > 50 ? '#ffa500' : '#ff6464'));
                var effBar = html('<div style="display:flex;align-items:center;gap:8px;"><div style="flex:1;min-width:80px;">' + progressBar(effPct, 200, color).outerHTML + '</div><span>' + effPct.toFixed(1) + '%</span></div>');
                var recipeDisplayName = displayName(recipeKey) || r.recipe_name || recipeKey;

                // Count slooped buildings in this recipe
                var recipeSlooped = 0, recipeSloops = 0;
                if (r.buildings) {
                    r.buildings.forEach(function(b) {
                        if (b.slooped) {
                            recipeSlooped++;
                            recipeSloops += b.slooped;
                        }
                    });
                }
                var summaryText = recipeDisplayName + ' (' + t('label.buildings_efficiency', (r.building_count || r.count || 0), effPct.toFixed(1)) + ')';
                if (recipeSlooped > 0) {
                    summaryText += ' \u2014 ' + t('label.slooped_sloops', recipeSlooped, recipeSloops);
                }

                var det = h('details', { class: 'det-wide' });
                det.appendChild(h('summary', { text: summaryText }));

                var recipeRows = [[recipeDisplayName, (r.avg_clock_speed || 0).toFixed(1) + '%', fmt(r.building_count || r.count || 0), fmt(r.actual_output || 0), fmt(r.theoretical_max || 0), effBar]];
                det.appendChild(sortableTable(
                    [
                        {label: t('col.recipe'), width: '90px'},
                        {label: t('col.avg_clock'), type: 'number', width: '60px'},
                        {label: t('col.buildings'), type: 'number', width: '80px'},
                        {label: t('col.output_min'), type: 'number', width: '60px'},
                        {label: t('col.max_min'), type: 'number', width: '60px'},
                        {label: t('col.efficiency'), width: '180px'}
                    ],
                    recipeRows,
                    {scrollable: true}
                ));

                // Building details (paginated, with slooped column)
                if (r.buildings && r.buildings.length > 0) {
                    var bldgHeaders = [
                        {label: t('col.type'), width: '90px'},
                        {label: t('col.clock'), type: 'number', width: '50px'},
                        {label: t('col.sloops'), type: 'number', width: '50px'},
                        {label: t('col.input_belts'), width: '130px'},
                        {label: t('col.output_belts'), width: '80px'},
                        {label: '', width: '40px'}
                    ];

                    var bldgRows = r.buildings.map(function(b) {
                        var sloopCell = b.slooped ? String(b.slooped) : '';
                        var inBelts = (b.input_belts || []).map(translateBeltLabel).join(', ');
                        var outBelts = (b.output_belts || []).map(translateBeltLabel).join(', ');
                        return [html(iconName(b.type)), (b.clock_speed || 100).toFixed(2) + '%', sloopCell, inBelts, outBelts, mapLink(b.location)];
                    });

                    var bldgDet = h('details', { style: 'margin-top:8px; width: 100%; box-sizing: border-box;' });
                    bldgDet.appendChild(h('summary', { text: t('btn.show_buildings', r.buildings.length) }));

                    var pt = paginatedTable(bldgRows, bldgHeaders, {
                        container: bldgDet,
                        filterPlaceholder: t('filter.by_type'),
                        scrollable: true,
                        deferRender: true
                    });

                    // Lazy-load: only render the building table when the details
                    // element is first opened, to avoid rendering all recipes upfront
                    bldgDet.addEventListener('toggle', function() { if (bldgDet.open && pt.render) pt.render(); });
                    det.appendChild(bldgDet);
                }
                detWrap.appendChild(det);
            });
            s2.appendChild(detWrap);
            c.appendChild(s2);
        }

    }