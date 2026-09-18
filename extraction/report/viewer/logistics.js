    function renderLogistics() {
        var c = document.getElementById('tab-logistics');
        c.innerHTML = '';
        var d = DATA;
        c.appendChild(tabDescription(t('desc.tab_logistics')));
        var tr = (d.analytics && d.analytics.transport) || {};
        var prod = d.production || {};
        var TAB_COLOR = '#9ca3af';

        // Infrastructure (including splitters/mergers)
        var s1 = section(t('section.infrastructure'));
        s1.classList.add('card-section');
        var grid = cardGrid(5);
        // Filter out infrastructure categories that have their own dedicated tabs:
        // vehicle paths -> Transport, hypertubes -> Transport, trains -> Transport,
        // power lines -> Power. Only belts and pipes belong here.
        var ib = tr.infrastructure_breakdown || {};
        var infraKeyMap = {
            'belts_km': 'card.conveyor_belts',
            'lifts_km': 'card.conveyor_lifts',
            'pipes_km': 'card.pipelines'
        };
        Object.keys(ib).forEach(function(key) {
            if (!ib[key] || ib[key] === 0) return;
            if (key === 'vehicle_paths_km') return;
            if (key === 'hypertubes_km') return;
            if (key === 'trains_km') return;
            if (key === 'power_lines_km') return;
            var tKey = infraKeyMap[key];
            var label = tKey ? t(tKey) : key.replace(/_/g, ' ').replace(/\b\w/g, function(c) { return c.toUpperCase(); });
            grid.appendChild(card(label, ib[key].toFixed(2) + ' km', '', TAB_COLOR));
        });
        s1.appendChild(grid);

        // Splitters & Mergers as second row within Infrastructure
        var splitterMergerTotal = (prod.splitters || 0) + (prod.smartSplitters || 0) + (prod.programmableSplitters || 0) + (prod.mergers || 0);
        if (splitterMergerTotal > 0) {
            var gridSM = cardGrid(5);
            if (prod.splitters) gridSM.appendChild(card(t('card.splitters'), fmt(prod.splitters), '', TAB_COLOR));
            if (prod.smartSplitters) gridSM.appendChild(card(t('card.smart_splitters'), fmt(prod.smartSplitters), '', TAB_COLOR));
            if (prod.programmableSplitters) gridSM.appendChild(card(t('card.programmable_splitters'), fmt(prod.programmableSplitters), '', TAB_COLOR));
            if (prod.mergers) gridSM.appendChild(card(t('card.mergers'), fmt(prod.mergers), '', TAB_COLOR));
            s1.appendChild(gridSM);
        }

        c.appendChild(s1);

        var beltHeaders = [
            {label: t('col.type'), width: '200px'},
            {label: t('col.count'), type: 'number', width: '100px'},
            {label: t('col.length'), type: 'number', width: '100px'}
        ];

        // Belts + Lifts side by side
        var hasBelts = prod.belts && Object.keys(prod.belts).length > 0;
        var hasLifts = prod.lifts && Object.keys(prod.lifts).length > 0;
        if (hasBelts || hasLifts) {
            var row = wrapRow();
            if (hasBelts) {
                var s2 = section(t('card.conveyor_belts'));
                s2.classList.add('auto-width');
                var beltRows = Object.keys(prod.belts).map(function(type) {
                    var b = prod.belts[type];
                    return [html(iconName(type)), fmt(b.Count), fmtKm(b.TotalLength)];
                }).sort(byNumDesc(1));
                s2.appendChild(sortableTable(beltHeaders, beltRows, {scrollable: true, autoWidth: true}));
                row.appendChild(s2);
            }
            if (hasLifts) {
                var s4 = section(t('card.conveyor_lifts'));
                s4.classList.add('auto-width');
                var liftRows = Object.keys(prod.lifts).map(function(type) {
                    var l = prod.lifts[type];
                    return [html(iconName(type)), fmt(l.Count), fmtKm(l.TotalLength)];
                }).sort(byNumDesc(1));
                s4.appendChild(sortableTable(beltHeaders, liftRows, {scrollable: true, autoWidth: true}));
                row.appendChild(s4);
            }
            c.appendChild(row);
        }

        // Pipes alone
        if (prod.pipes && Object.keys(prod.pipes).length > 0) {
            var s3 = section(t('section.pipes'));
            s3.classList.add('auto-width');
            var pipeRows = Object.keys(prod.pipes).map(function(type) {
                var p = prod.pipes[type];
                return [html(iconName(type)), fmt(p.Count), fmtKm(p.TotalLength)];
            }).sort(byNumDesc(1));
            s3.appendChild(sortableTable(beltHeaders, pipeRows, {scrollable: true, autoWidth: true}));
            c.appendChild(s3);
        }

        // Pipeline Pumps
        if (prod.pipePumps && Object.keys(prod.pipePumps).length > 0) {
            var s5 = section(t('section.pipeline_pumps'));
            s5.classList.add('auto-width');
            var pumpRows = Object.keys(prod.pipePumps).map(function(type) {
                var p = prod.pipePumps[type];
                var count = (typeof p === 'object') ? (p.Count || p.count || 0) : p;
                return [html(iconName(type)), fmt(count)];
            }).sort(byNumDesc(1));
            s5.appendChild(sortableTable(
                [{label: t('col.type'), width: '280px'},
                {label: t('col.count'), type: 'number', width: '80px'}],
                pumpRows,
                {autoWidth: true}
            ));
            c.appendChild(s5);
        }
    }