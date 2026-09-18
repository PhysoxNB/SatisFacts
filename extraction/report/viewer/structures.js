    function renderStructures() {
        var c = document.getElementById('tab-structures');
        c.innerHTML = '';
        var d = DATA;
        c.appendChild(tabDescription(t('desc.tab_structures')));
        var totals = (d.structures && d.structures.totals) || {};
        var details = (d.structures && d.structures.details) || {};

        // Summary cards
        var s1 = section(t('section.structure_summary'));
        s1.classList.add('card-section');
        var grid = cardGrid();
        var TAB_COLOR = '#9ca3af';
        var catOrder = ['statues', 'foundations', 'walls', 'attachments', 'beams', 'walkways', 'roofs',
                        'signs', 'pillars', 'lights'];
        catOrder.forEach(function(cat) {
            if (totals[cat] && totals[cat] > 0) {
                grid.appendChild(card(t('struct.' + cat), fmt(totals[cat]), '', TAB_COLOR));
            }
        });
        // any categories not in the fixed order, sorted by count
        var remaining = Object.keys(totals).filter(function(cat) {
            return catOrder.indexOf(cat) < 0 && cat !== 'powerSwitches' && totals[cat] > 0;
        }).sort(function(a, b) { return totals[b] - totals[a]; });
        remaining.forEach(function(cat) {
            grid.appendChild(card(t('struct.' + cat), fmt(totals[cat]), '', TAB_COLOR));
        });
        s1.appendChild(grid);
        c.appendChild(s1);

        // Statues get special treatment: gold-colored cards in a 6-column grid
        // right under the summary, since they're collectibles rather than buildables
        if (details['statues']) {
            var sStat = section(t('section.statues') + ' (' + fmt(totals['statues']) + ')');
            sStat.classList.add('card-section');
            var gridStat = cardGrid(6);
            Object.keys(details['statues']).filter(function(type) { return details['statues'][type] > 0; })
                .sort(function(a, b) { return details['statues'][b] - details['statues'][a]; })
                .forEach(function(type) {
                    var statCard = card(displayName(type), fmt(details['statues'][type]), '', '#fbbf24');
                    var statIcon = iconFor(type);
                    if (statIcon) {
                        var iconImg = document.createElement('img');
                        iconImg.src = statIcon;
                        iconImg.style.cssText = 'width: 28px; height: 28px; display: block; margin: 0 auto 4px;';
                        statCard.insertBefore(iconImg, statCard.firstChild);
                    }
                    var labelEl = statCard.querySelector('.label');
                    if (labelEl) labelEl.style.textAlign = 'center';
                    var valueEl = statCard.querySelector('.value');
                    if (valueEl) valueEl.style.textAlign = 'center';
                    gridStat.appendChild(statCard);
                });
            sStat.appendChild(gridStat);
            c.appendChild(sStat);
        }

        // helper: build a detail section for a category
        function detailSection(cat) {
            if (!details[cat] || Object.keys(details[cat]).length === 0) return null;
            var types = details[cat];
            var rows = Object.keys(types).filter(function(type) { return types[type] > 0; }).map(function(type) {
                return [html(iconName(type)), fmt(types[type])];
            }).sort(byNumDesc(1));
            if (rows.length === 0) return null;
            var s = section(t('struct.' + cat) + ' (' + fmt(totals[cat]) + ')');
            s.classList.add('auto-width');
            s.appendChild(sortableTable(
                [{label: t('col.type'), width: '280px'},
                {label: t('col.count'), type: 'number', width: '80px'}],
                rows,
                {scrollable: rows.length > 20, autoWidth: true}
            ));
            return s;
        }

        // Detail sections use auto-width so they shrink to table content and
        // sit side-by-side in a flex-wrap container. Categories not in the
        // fixed order go in a separate wrap below.
        var detailOrder = ['foundations', 'walls', 'attachments', 'beams', 'walkways', 'roofs',
                           'signs', 'pillars', 'lights'];
        var available = detailOrder.filter(function(cat) { return detailSection(cat) !== null; });

        var wrap = wrapRow();
        available.forEach(function(cat) {
            wrap.appendChild(detailSection(cat));
        });
        c.appendChild(wrap);

        // any categories not in the fixed order or statues
        var known = detailOrder.concat(['statues', 'powerSwitches']);
        var otherWrap = wrapRow();
        var hasOther = false;
        Object.keys(totals).sort(function(a, b) { return totals[b] - totals[a]; }).forEach(function(cat) {
            if (known.indexOf(cat) >= 0) return;
            var s = detailSection(cat);
            if (s) { otherWrap.appendChild(s); hasOther = true; }
        });
        if (hasOther) c.appendChild(otherWrap);
    }