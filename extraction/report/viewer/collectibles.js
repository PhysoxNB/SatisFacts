    function renderCollectibles() {
        var c = document.getElementById('tab-collectibles');
        c.innerHTML = '';
        var d = DATA;
        var col = (d.analytics && d.analytics.collectibles) || {};

        c.appendChild(tabDescription(t('desc.tab_collectibles')));

        // Map collectible display names to icon class names for icon lookup.
        var collectibleIconMap = {
            'Power Slugs': 'Desc_Crystal_C',
            'Blue Slugs': 'Desc_Crystal_C',
            'Yellow Slugs': 'Desc_Crystal_mk2_C',
            'Purple Slugs': 'Desc_Crystal_mk3_C',
            'Blue': 'Desc_Crystal_C',
            'Yellow': 'Desc_Crystal_mk2_C',
            'Purple': 'Desc_Crystal_mk3_C',
            'Somersloops': 'Desc_WAT1_C',
            'Somersloop': 'Desc_WAT1_C',
            'Mercer Spheres': 'Desc_WAT2_C',
            'Mercer Sphere': 'Desc_WAT2_C',
            'Crash Sites': 'Desc_HardDrive_C',
            'Crash Site': 'Desc_HardDrive_C'
        };
        function collectibleIcon(name) {
            var cls = collectibleIconMap[name];
            if (cls && DATA.icons && DATA.icons[cls]) return DATA.icons[cls];
            return iconForName(name);
        }
        function collectibleIconHtml(name) {
            var icon = collectibleIcon(name);
            if (icon) return '<img class="row-icon" src="' + icon + '" alt=""> ';
            return '';
        }

        // World collectibles (slugs, sloops, spheres, crash sites) are tracked
        // via the collectables list embedded in the save. This list records what
        // the player has collected/dismantled, so we show progress as collected/total.
        var wc = col.world_collectibles;
        if (wc) {
            var ws = section(t('section.world_collectibles'));
            ws.classList.add('card-section');

            // Power Slugs
            if (wc.power_slugs) {
                var ps = wc.power_slugs;
                var slugGrid = cardGrid(5);
                slugGrid.appendChild(progressCard(t('card.power_slugs'), ps.collected, ps.total, t('desc.remaining', ps.remaining)));
                if (ps.blue) slugGrid.appendChild(progressCard(t('card.blue_slugs'), ps.blue.collected, ps.blue.total, t('desc.remaining', ps.blue.remaining), '#4a9eff'));
                if (ps.yellow) slugGrid.appendChild(progressCard(t('card.yellow_slugs'), ps.yellow.collected, ps.yellow.total, t('desc.remaining', ps.yellow.remaining), '#f5a623'));
                if (ps.purple) slugGrid.appendChild(progressCard(t('card.purple_slugs'), ps.purple.collected, ps.purple.total, t('desc.remaining', ps.purple.remaining), '#9b59b6'));
                ws.appendChild(slugGrid);
            }

            // Somersloops, Mercer Spheres, Crash Sites - combined in one row
            var otherGrid = cardGrid(5);
            if (wc.somersloops) {
                var sl = wc.somersloops;
                otherGrid.appendChild(progressCard(t('card.somersloops'), sl.collected, sl.total, t('desc.remaining', sl.remaining), '#e91e63'));
            }
            if (wc.mercer_spheres) {
                var ms = wc.mercer_spheres;
                otherGrid.appendChild(progressCard(t('card.mercer_spheres'), ms.collected, ms.total, t('desc.remaining', ms.remaining), '#a855f7'));
            }
            if (wc.crash_sites) {
                var cs = wc.crash_sites;
                var csSub = t('desc.crash_sites_status', (cs.dismantled || 0), (cs.opened || 0), (cs.unopened || 0));
                otherGrid.appendChild(progressCard(t('card.crash_sites'), cs.collected, cs.total, csSub, '#e67e22'));
            }
            if (otherGrid.children.length > 0) ws.appendChild(otherGrid);

            c.appendChild(ws);

            // Collectible location dropdowns
            var locSection = section(t('section.remaining_collectibles'));
            locSection.classList.add('sync-details-header');
            var locWrap = wrapRow();

            if (wc.power_slugs && wc.power_slugs.locations && wc.power_slugs.locations.length > 0) {
                var slugDet = h('details', { class: 'det-compact' });
                slugDet.appendChild(h('summary', { text: t('card.power_slugs') + ' (' + wc.power_slugs.locations.length + ' ' + t('label.remaining') + ')' }));
                var slugRows = wc.power_slugs.locations.map(function(item) {
                    var colorKey = item.color ? item.color.toLowerCase() : '';
                    var colorLabel = t('color.' + colorKey) || item.color;
                    return [html(collectibleIconHtml(item.color) + esc(colorLabel)), mapLink(item.location)];
                });
                paginatedTable(slugRows,
                    [{label: t('col.color'), width: '100px'},
                     {label: '', width: '40px'}],
                    {
                        container: slugDet,
                        filterPlaceholder: t('filter.by_color'),
                        scrollable: true,
                        pageSize: 100
                    }
                );
                locWrap.appendChild(slugDet);
            }

            if (wc.crash_sites && wc.crash_sites.locations && wc.crash_sites.locations.length > 0) {
                var crashDet = h('details', { class: 'det-compact' });
                var crashRemaining = wc.crash_sites.remaining || 0;
                var crashLooted = wc.crash_sites.locations.length - crashRemaining;
                var crashLabel = t('card.crash_sites') + ' (' + crashRemaining + ' ' + t('label.remaining');
                if (crashLooted > 0) crashLabel += ', ' + crashLooted + ' ' + t('label.looted');
                crashLabel += ')';
                crashDet.appendChild(h('summary', { text: crashLabel }));

                var crashInfoBtn = h('button', { class: 'loc-toggle-btn', title: t('crash.statuses_title'), style: 'color: #e67e22; font-weight: bold;' }, t('btn.info_status'));
                var crashPopover = null;
                crashInfoBtn.addEventListener('click', function(e) {
                    e.stopPropagation();
                    if (crashPopover) {
                        crashPopover.remove();
                        crashPopover = null;
                        return;
                    }
                    crashPopover = h('div', {
                        style: 'position: fixed; z-index: 1000; padding: 14px 16px; background: #1a2332; border-left: 4px solid #e67e22; border-radius: 6px; box-shadow: 0 8px 24px rgba(0,0,0,0.5); max-width: 420px; font-size: 0.9em; line-height: 1.5;'
                    });
                    crashPopover.appendChild(h('div', { style: 'margin-bottom: 8px;' },
                        h('strong', { style: 'color: #e67e22; font-size: 1.05em;', text: t('crash.statuses_title') })));
                    crashPopover.appendChild(h('div', { style: 'color: #ccc; margin-bottom: 6px;',
                        text: t('crash.status_unopened') }));
                    crashPopover.appendChild(h('div', { style: 'color: #ccc; margin-bottom: 6px;',
                        text: t('crash.status_looted') }));
                    crashPopover.appendChild(h('div', { style: 'color: #fbbf24;',
                        text: t('crash.status_dismantled_warn') }));
                    var rect = crashInfoBtn.getBoundingClientRect();
                    crashPopover.style.top = (rect.bottom + 8) + 'px';
                    crashPopover.style.right = (window.innerWidth - rect.right) + 'px';
                    document.body.appendChild(crashPopover);
                    setTimeout(function() {
                        var closeHandler = function(ev) {
                            if (crashPopover && !crashPopover.contains(ev.target) && ev.target !== crashInfoBtn) {
                                crashPopover.remove();
                                crashPopover = null;
                                document.removeEventListener('click', closeHandler);
                            }
                        };
                        document.addEventListener('click', closeHandler);
                    }, 0);
                });

                var crashRows = wc.crash_sites.locations.map(function(item) {
                    var statusKey = item.status ? item.status.toLowerCase() : '';
                    var statusLabel = t('crash.' + statusKey) || item.status;
                    return [html(collectibleIconHtml('Crash Sites') + esc(statusLabel)), mapLink(item.location)];
                });
                paginatedTable(crashRows,
                    [{label: t('col.status'), width: '110px'},
                     {label: '', width: '40px'}],
                    {
                        container: crashDet,
                        filterPlaceholder: t('filter.by_status'),
                        scrollable: true,
                        pageSize: 100,
                        extraButtons: [crashInfoBtn]
                    }
                );
                locWrap.appendChild(crashDet);
            }

            if (wc.somersloops && wc.somersloops.locations && wc.somersloops.locations.length > 0) {
                var sloopDet = h('details', { class: 'det-compact' });
                sloopDet.appendChild(h('summary', { text: t('card.somersloops') + ' (' + wc.somersloops.locations.length + ' ' + t('label.remaining') + ')' }));
                var sloopRows = wc.somersloops.locations.map(function(item) {
                    return [html(collectibleIconHtml('Somersloop') + esc(t('card.somersloops'))), mapLink(item.location)];
                });
                paginatedTable(sloopRows,
                    [{label: t('col.type'), width: '130px'},
                     {label: '', width: '40px'}],
                    {
                        container: sloopDet,
                        scrollable: true,
                        pageSize: 100
                    }
                );
                locWrap.appendChild(sloopDet);
            }

            if (wc.mercer_spheres && wc.mercer_spheres.locations && wc.mercer_spheres.locations.length > 0) {
                var sphereDet = h('details', { class: 'det-compact' });
                sphereDet.appendChild(h('summary', { text: t('card.mercer_spheres') + ' (' + wc.mercer_spheres.locations.length + ' ' + t('label.remaining') + ')' }));
                var sphereRows = wc.mercer_spheres.locations.map(function(item) {
                    return [html(collectibleIconHtml('Mercer Sphere') + t('card.mercer_spheres')), mapLink(item.location)];
                });
                paginatedTable(sphereRows,
                    [{label: t('col.type'), width: '150px'},
                     {label: '', width: '40px'}],
                    {
                        container: sphereDet,
                        scrollable: true,
                        pageSize: 100
                    }
                );
                locWrap.appendChild(sphereDet);
            }

            if (locWrap.children.length > 0) {
                locSection.appendChild(locWrap);
                c.appendChild(locSection);
            }
        }

        var s1 = section(t('section.other_collectibles'));
        s1.classList.add('card-section');
        var grid = cardGrid(5);
        var TAB_COLOR = '#9ca3af';
        grid.appendChild(card(t('card.total_pickups'), fmt(col.total_pickups || 0), t('desc.pickups_destroyed'), TAB_COLOR));
        if (col.tapes) grid.appendChild(card(t('card.tapes'), fmt(col.tapes.total_collected || 0), t('label.collected'), TAB_COLOR));

        // Item pickups are objects still sitting on the map (not yet collected).
        // These come from scanning for pickup actors, not from the collectables list.
        // Only pickups in discovered areas are visible - the save file doesn't
        // contain objects from unexplored parts of the map.
        var ip = col.item_pickups || {};
        if (ip.available_count) {
            grid.appendChild(card(t('card.item_pickups'), fmt(ip.available_count || 0), t('desc.pickups_on_map'), TAB_COLOR));
            grid.appendChild(card(t('card.total_items'), fmt(ip.total_items || 0), t('desc.items_from_pickups'), TAB_COLOR));
        }
        s1.appendChild(grid);
        c.appendChild(s1);

        // Item pickups table
        if (ip.by_type && Object.keys(ip.by_type).length > 0) {
            var s2 = section(t('section.item_pickups_by_type'));
            s2.classList.add('sync-details-header');
            var detWrap = wrapRow();
            var byType = ip.by_type;
            var types = Object.keys(byType).sort(function(a, b) {
                return (byType[b].total_items || 0) - (byType[a].total_items || 0);
            });

            types.forEach(function(itemName) {
                var info = byType[itemName];
                var det = h('details', { class: 'det-narrow' });
                // Use item_class for localized display name and icon
                var localizedName = info.item_class ? displayName(info.item_class) : itemName;
                var itemIcon = iconForName(itemName);
                // Try item_class for icon lookup (e.g. statue pickups)
                if (!itemIcon && info.item_class) {
                    itemIcon = iconFor(info.item_class);
                }
                var summaryLabel = localizedName + ' (' + info.count + ' ' + t('label.pickups') + ', ' + fmt(info.total_items) + ' ' + t('label.items') + ')';
                if (itemIcon) {
                    det.appendChild(h('summary', { html: '<img class="row-icon" src="' + itemIcon + '" alt=""> ' + esc(summaryLabel) }));
                } else {
                    det.appendChild(h('summary', { text: summaryLabel }));
                }

                var itemHtml = info.item_class ? iconName(info.item_class) : nameWithIcon(itemName);
                if (!itemIcon) {
                    // Try item_class for the table rows too
                    var clsIcon = info.item_class ? iconFor(info.item_class) : '';
                    if (clsIcon) {
                        itemHtml = '<img class="row-icon" src="' + clsIcon + '" alt=""> ' + esc(localizedName);
                    }
                }
                var rows = (info.pickups || []).map(function(p) {
                    return [html(itemHtml), fmt(p.num_items || 0), mapLink(p.position)];
                });

                det.appendChild(sortableTable(
                    [{label: t('col.item'), width: '200px'},
                    {label: t('col.quantity'), type: 'number', width: '100px'},
                    {label: '', width: '40px'}],
                    rows,
                    {scrollable: true}
                ));
                detWrap.appendChild(det);
            });
            s2.appendChild(detWrap);
            c.appendChild(s2);
        }

        // Tapes list
        if (col.tapes && col.tapes.collected && col.tapes.collected.length > 0) {
            var s3 = section(t('section.collected_tapes'));
            s3.classList.add('auto-width');
            var rows = col.tapes.collected.map(function(tape) {
                var tapeName = tape.replace(/Tape_/, '').replace(/_/g, ' ');
                var tapeIcon = iconForName('Boombox');
                var tapeHTML = (tapeIcon ? '<img class="row-icon" src="' + tapeIcon + '" alt=""> ' : '') + esc(tapeName);
                return [html(tapeHTML)];
            });
            s3.appendChild(sortableTable([{label: t('col.tape'), width: '300px'}], rows, {autoWidth: true}));
            c.appendChild(s3);
        }
    }