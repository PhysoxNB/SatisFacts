    // Map short type names from backend to full class paths for icon lookup.
    var extractorTypeMap = {
        'MinerMk1': '/Game/FactoryGame/Buildable/Factory/Miner/Build_MinerMk1.Build_MinerMk1_C',
        'MinerMk2': '/Game/FactoryGame/Buildable/Factory/Miner/Build_MinerMk2.Build_MinerMk2_C',
        'MinerMk3': '/Game/FactoryGame/Buildable/Factory/Miner/Build_MinerMk3.Build_MinerMk3_C',
        'Fracking Unit': '/Game/FactoryGame/Buildable/Factory/FrackingExtractor/Build_FrackingExtractor.Build_FrackingExtractor_C',
        'FrackingExtractor': '/Game/FactoryGame/Buildable/Factory/FrackingExtractor/Build_FrackingExtractor.Build_FrackingExtractor_C',
        'Oil Pump': '/Game/FactoryGame/Buildable/Factory/OilPump/Build_OilPump.Build_OilPump_C',
        'Water Pump': '/Game/FactoryGame/Buildable/Factory/WaterPump/Build_WaterPump.Build_WaterPump_C'
    };
    function extractorIconName(type) {
        var path = extractorTypeMap[type] || type;
        return iconName(path);
    }

    function renderExtraction() {
        var c = document.getElementById('tab-production');
        c.innerHTML = '';
        var d = DATA;
        c.appendChild(tabDescription(t('desc.tab_extraction')));
        var prod = (d.analytics && d.analytics.production) || {};
        var miners = prod.miners || {};
        var minerDetails = prod.miner_details || {};
        var fluidEx = prod.fluid_extractors || {};
        var fluidExDetails = prod.fluid_extractor_details || {};

        // Summary cards
        var s1 = section(t('section.mining_extraction'));
        s1.classList.add('card-section');
        var grid = cardGrid();
        var TAB_COLOR = '#9ca3af';
        grid.appendChild(card(t('card.miner_instances'), Object.keys(minerDetails).length, '', TAB_COLOR));
        grid.appendChild(card(t('card.fluid_extractor_instances'), Object.keys(fluidExDetails).length, '', TAB_COLOR));
        var prodByRes = prod.production_by_resource || {};
        grid.appendChild(card(t('card.resources'), Object.keys(prodByRes).length, '', TAB_COLOR));
        s1.appendChild(grid);
        c.appendChild(s1);

        // Miner summary by type + Fluid extractors side-by-side
        var hasMiners = Object.keys(miners).length > 0;
        var hasFluidEx = Object.keys(fluidEx).length > 0;
        if (hasMiners || hasFluidEx) {
            var fr = wrapRow();

            if (hasMiners) {
                var s2 = section(t('section.miners_by_type'));
                s2.classList.add('auto-width');
                var minerRows = Object.keys(miners).map(function(type) {
                    var m = miners[type];
                    return [html(extractorIconName(type)), m.count || 0, m.avg_clock_speed ? m.avg_clock_speed.toFixed(1) + '%' : '100%'];
                });
                s2.appendChild(sortableTable(
                    [{label: t('col.type'), width: '280px'},
                    {label: t('col.count'), type: 'number', width: '80px'},
                    {label: t('col.avg_clock'), type: 'number', width: '100px'}],
                    minerRows,
                    {autoWidth: true}
                ));
                fr.appendChild(s2);
            }

            if (hasFluidEx) {
                var s4 = section(t('section.fluid_extractors'));
                s4.classList.add('auto-width');
                var fluidRows = Object.keys(fluidEx).map(function(type) {
                    var f = fluidEx[type];
                    return [html(extractorIconName(type)), f.count || 0, f.avg_clock_speed ? f.avg_clock_speed.toFixed(1) + '%' : '100%'];
                });
                s4.appendChild(sortableTable(
                    [{label: t('col.type'), width: '280px'},
                    {label: t('col.count'), type: 'number', width: '80px'},
                    {label: t('col.avg_clock'), type: 'number', width: '100px'}],
                    fluidRows,
                    {autoWidth: true}
                ));
                fr.appendChild(s4);
            }

            c.appendChild(fr);
        }

        // Production by Resource
        if (Object.keys(prodByRes).length > 0) {
            var s5 = section(t('section.production_by_resource'));
            s5.classList.add('auto-width');
            var resKeys = Object.keys(prodByRes).sort(function(a, b) {
                return (prodByRes[b].effective_output || 0) - (prodByRes[a].effective_output || 0);
            });
            var resRows = resKeys.map(function(res) {
                var r = prodByRes[res];
                var bn = r.bottlenecked || 0;
                var curOut = r.effective_output !== undefined ? fmt(Math.round(r.effective_output)) + '/min' : '—';
                if (bn > 0) curOut = '<span style="color:#ef4444;font-weight:bold">\u26A0</span> ' + curOut;
                return [
                    html(iconName(res)),
                    r.count || 0,
                    r.total_output !== undefined ? fmt(Math.round(r.total_output)) + '/min' : '—',
                    {__html: curOut}
                ];
            });
            s5.appendChild(sortableTable(
                [{label: t('col.resource'), width: '200px'},
                {label: t('col.extractors'), type: 'number', width: '100px'},
                {label: t('col.theoretical'), type: 'number', width: '120px'},
                {label: t('col.current_output'), type: 'number', width: '130px'}],
                resRows,
                {autoWidth: true}
            ));
            c.appendChild(s5);
        }

        // Miner and fluid extractor details are combined into one table since
        // they share the same column structure. The only difference is miners
        // have output_belts and fluid extractors have output_pipes.
        var allDetails = [];
        Object.keys(minerDetails).forEach(function(inst) {
            var m = minerDetails[inst];
            var locCell = mapLink(m.location);
            var outLabel = esc(translateBeltLabel(m.belt_label));
            if (m.bottleneck) outLabel = '<span style="color:#ef4444;font-weight:bold">\u26A0</span> ' + outLabel;
            var row = [
                m.type ? html(extractorIconName(m.type)) : t('status.unknown'),
                (m.clock_speed || 100).toFixed(2) + '%',
                m.purity ? t('purity.' + m.purity) : t('purity.unknown'),
                html(iconName(m.resource || 'Unknown')),
                m.capacity !== undefined ? fmt(m.capacity) + '/min' : '—',
                {__html: outLabel},
                locCell
            ];
            row._bottleneck = !!m.bottleneck;
            allDetails.push(row);
        });
        Object.keys(fluidExDetails).forEach(function(inst) {
            var f = fluidExDetails[inst];
            var locCell = mapLink(f.location);
            var outLabel = esc(translateBeltLabel(f.belt_label));
            if (f.bottleneck) outLabel = '<span style="color:#ef4444;font-weight:bold">\u26A0</span> ' + outLabel;
            var row = [
                f.type ? html(extractorIconName(f.type)) : t('status.unknown'),
                (f.clock_speed || 100).toFixed(2) + '%',
                f.purity ? t('purity.' + f.purity) : t('purity.unknown'),
                html(iconName(f.resource || 'Unknown')),
                f.capacity !== undefined ? fmt(f.capacity) + '/min' : '—',
                {__html: outLabel},
                locCell
            ];
            row._bottleneck = !!f.bottleneck;
            allDetails.push(row);
        });

        if (allDetails.length > 0) {
            var s3 = paginatedTable(allDetails,
                [{label: t('col.type'), width: '120px'},
                {label: t('col.clock'), type: 'number', width: '100px'},
                {label: t('col.purity'), width: '70px'},
                {label: t('col.resource'), width: '120px'},
                {label: t('col.output'), type: 'number', width: '90px'},
                {label: t('col.belt_pipe'), width: '130px'},
                {label: '', width: '40px'}],
                {
                    sectionTitle: t('section.extractor_details') + ' (' + allDetails.length + ')',
                    autoWidth: true,
                    filterPlaceholder: t('filter.by_type_resource'),
                    scrollable: true,
                    filterCols: [0, 3],
                    extraFilters: [{
                        allLabel: t('filter.all_statuses'),
                        values: ['bottlenecked', 'ok'],
                        labels: [t('filter.bottlenecked'), t('filter.ok')],
                        match: function(row, val) {
                            if (val === t('filter.bottlenecked')) return !!row._bottleneck;
                            return !row._bottleneck;
                        }
                    }]
                });
            c.appendChild(s3);
        }
    }