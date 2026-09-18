    function renderClockSpeeds() {
        var c = document.getElementById('tab-clockspeeds');
        c.innerHTML = '';
        var d = DATA;
        c.appendChild(tabDescription(t('desc.tab_clockspeeds')));
        var csd = (d.analytics && d.analytics.clockSpeedDistribution) || {};
        var pbd = (d.analytics && d.analytics.productionBoostDistribution) || {};
        var css = (d.analytics && d.analytics.clockSpeedSloopCounts) || {};

        if (Object.keys(csd).length === 0) {
            c.appendChild(emptyState(t('empty.no_clock_speed_data')));
            return;
        }

        // Legend / explanation
        var legend = section(t('section.clock_speed_legend'));
        legend.classList.add('card-section');
        legend.classList.add('sync-text-header');
        legend.appendChild(h('p', { text: t('desc.clock_speed_explanation'), style: 'color: #aaa; margin-bottom: 12px; text-align: center; width: fit-content; max-width: 100%; margin-left: auto; margin-right: auto;' }));
        legend.appendChild(h('p', { text: t('desc.somersloop_explanation'), style: 'color: #aaa; margin-bottom: 12px; text-align: center; width: fit-content; max-width: 100%; margin-left: auto; margin-right: auto;' }));
        var legendGrid = cardGrid(4);
        legendGrid.appendChild(card(t('label.100_default'), '', html(t('desc.legend_default')), '#9ca3af'));
        legendGrid.appendChild(card(t('label.below_100'), '', html(t('desc.legend_underclocked')), '#9ca3af'));
        legendGrid.appendChild(card(t('label.above_100'), '', html(t('desc.legend_overclocked')), '#9ca3af'));
        legendGrid.appendChild(card(t('label.somerslooped'), '', html(t('desc.legend_slooped')), '#9ca3af'));
        legend.appendChild(legendGrid);
        c.appendChild(legend);

        // A building type can appear in clock speed data, somersloop data, or both.
        // Merge keys from both so we don't miss types that only have sloop data.
        var allTypes = {};
        Object.keys(csd).forEach(function(t) { allTypes[t] = true; });
        Object.keys(pbd).forEach(function(t) { allTypes[t] = true; });

        var s1 = section(t('section.overclock_distribution'));
        s1.classList.add('sync-details-header');
        var detWrap = wrapRow();
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
            if (pbTotal > 0) summaryText += ' \u2014 ' + t('label.slooped_count', pbTotal);

            var det = h('details', { class: 'det-narrow' });
            var icon = iconFor(type);
            if (icon) {
                det.appendChild(h('summary', { html: '<img class="row-icon" src="' + icon + '" alt=""> ' + esc(summaryText) }));
            } else {
                det.appendChild(h('summary', { text: summaryText }));
            }

            // Clock speed table
            if (csTotal > 0) {
                det.appendChild(h('h4', { text: t('label.clock_speed'), style: 'margin: 12px 0 6px; color: #ccc;' }));
                var sloopDist = css[type] || {};
                var csRows = Object.keys(csDist).map(function(speed) {
                    var count = csDist[speed];
                    var pct = (count / csTotal * 100).toFixed(1);
                    // Color: red=overclocked, blue=underclocked, green=default 100%
                    var color = speed > 100 ? '#ff6464' : (speed < 100 ? '#6495ED' : '#00ff00');
                    var slooped = sloopDist[speed] || 0;
                    var sloopCell = slooped > 0
                        ? html('<span style="color:#a855f7;font-weight:bold;">' + t('label.slooped_count', slooped) + '</span>')
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
                    [{label: t('col.clock_speed'), width: '120px'},
                    {label: t('col.buildings'), type: 'number', width: '100px'},
                    {label: t('col.sloops'), type: 'number', width: '120px'},
                    {label: t('col.share_of_total'), width: '200px'}],
                    csRows
                ));
            }

            detWrap.appendChild(det);
        });
        s1.appendChild(detWrap);
        c.appendChild(s1);
    }