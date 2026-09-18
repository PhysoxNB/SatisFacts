    function renderStorage() {
        var c = document.getElementById('tab-storage');
        c.innerHTML = '';
        var d = DATA;
        c.appendChild(tabDescription(t('desc.tab_storage')));
        var st = (d.analytics && d.analytics.storage) || {};
        var inv = (d.analytics && d.analytics.inventory) || {};
        var prod = d.production || {};
        var iit = prod.itemsInTransit || {};
        var transitTotal = 0, transitTypes = 0;
        for (var k in iit) { transitTotal += iit[k]; transitTypes++; }

        // Container summary
        var s1 = section(t('section.storage_containers'));
        s1.classList.add('card-section');
        var grid = cardGrid(5);
        var TAB_COLOR = '#9ca3af';
        grid.appendChild(card(t('card.total_slots'), fmt(st.total_slots || 0), '', TAB_COLOR));
        var containers = st.containers || {};
        Object.keys(containers).forEach(function(type) {
            var ct = containers[type];
            if (!ct.count || ct.count === 0) return;
            grid.appendChild(card(t('container.' + type), fmt(ct.count), t('desc.slots_each', fmt(ct.slots)), TAB_COLOR));
        });
        s1.appendChild(grid);
        c.appendChild(s1);

        // Items in Storage (in containers) and Items on Belts (in transit)
        // are shown side by side since they answer "where are my items?"
        var hasStorageItems = inv.items_by_count && Object.keys(inv.items_by_count).length > 0;
        var hasTransitItems = transitTypes > 0;

        if (hasStorageItems || hasTransitItems) {
            var row = wrapRow();
            var headers = [{label: t('col.item'), width: '280px'},
            {label: t('col.count'), type: 'number', width: '80px'}];

            if (hasStorageItems) {
                var storageRows = Object.keys(inv.items_by_count).map(function(item) {
                    return [html(iconName(item)), fmt(inv.items_by_count[item])];
                }).sort(byNumDesc(1));
                row.appendChild(paginatedTable(storageRows, headers, {
                    sectionTitle: t('section.items_in_storage') + ' (' + inv.unique_item_types + ' / ' + fmt(inv.total_items) + ')',
                    filterPlaceholder: t('filter.search_items'),
                    scrollable: true,
                    autoWidth: true
                }));
            }

            if (hasTransitItems) {
                var transitRows = Object.keys(iit).map(function(item) {
                    return [html(iconName(item)), fmt(iit[item])];
                }).sort(byNumDesc(1));
                row.appendChild(paginatedTable(transitRows, headers, {
                    sectionTitle: t('section.items_on_belts') + ' (' + transitTypes + ' / ' + fmt(transitTotal) + ')',
                    filterPlaceholder: t('filter.search_items'),
                    scrollable: true,
                    autoWidth: true
                }));
            }

            c.appendChild(row);
        }

        // Storage Breakdown - per-container details
        var cd = st.container_details || [];
        if (cd.length > 0) {
            var cdRows = cd.map(function(c) {
                var locCell = mapLink(c.location);
                var itemStr = '';
                if (c.items && c.items.length > 0) {
                    var parts = c.items.map(function(it) {
                        return iconName(it.item) + ' \u00d7' + fmt(it.count);
                    });
                    itemStr = parts.join(', ');
                }
                if (!itemStr) itemStr = DASH;
                return [html(iconName(c.type)), html(itemStr), locCell];
            });
            c.appendChild(paginatedTable(cdRows,
                [{label: t('col.container'), width: '200px'},
                {label: t('col.contents'), width: '600px', cls: 'wrap-cell'},
                {label: '', width: '40px'}],
                { sectionTitle: t('section.storage_breakdown') + ' (' + cd.length + ')', scrollable: true, maxWidth: '920px', sectionAutoWidth: true, filterPlaceholder: t('filter.by_type_item_location') }
            ));
        }
    }