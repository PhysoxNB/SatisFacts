    // h() is our DOM builder. Special attr keys: 'class', 'text', 'html', 'style'.
    // false/null attrs are skipped (useful for conditional attributes).
    // true sets a boolean attribute (e.g. disabled=true).
    // Any remaining arguments after attrs are appended as children.
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
        var labelStyle = color ? 'color: ' + color + ';' : '';
        var descEl = null;
        if (desc) {
            if (typeof desc === 'object' && desc.__html !== undefined) {
                descEl = h('div', { class: 'desc', html: desc.__html });
            } else {
                descEl = h('div', { class: 'desc', text: desc });
            }
        }
        return h('div', { class: 'card', style: style },
            h('div', { class: 'label', style: labelStyle, text: label }),
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
        var numericTotal = (typeof total === 'number' && total > 0) ? total : 0;
        var pct = numericTotal > 0 ? Math.min(100, (collected / numericTotal) * 100) : 0;
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

    function fillBadge(pct) {
        if (!pct || pct <= 0) return h('span', { text: DASH });
        var color = pct >= 80 ? 'green' : pct >= 50 ? 'yellow' : 'red';
        return badge(pct + '%', color);
    }

    function section(title) {
        return h('div', { class: 'section' }, h('h2', { text: title }));
    }

    // centered flex-wrap row, for putting sections or <details> side by side.
    // the flex rules have to stay inline: syncCardHeaders() finds these rows
    // via [style*="display: flex"]. move them into the CSS and the Power tab
    // header width sync breaks.
    function wrapRow() {
        return h('div', {
            class: 'auto-width-wrap',
            style: 'display: flex; flex-wrap: wrap; gap: 16px; justify-content: center; align-items: flex-start;'
        });
    }

    function emptyState(text) {
        return h('div', { class: 'empty-state', text: text });
    }

    // Creates a 📍 button cell for table rows. Clicking it calls showOnMap(x, y).
    // Handles both {X, Y, Z} object and [x, y, z] array location formats (in meters).
    // Returns {__el: buttonElement} for use in row arrays, or '' if no location.
    function mapLink(loc) {
        if (!loc) return '';
        var x, y;
        if (Array.isArray(loc)) {
            if (loc.length < 2) return '';
            x = loc[0]; y = loc[1];
        } else if (loc.X !== undefined) {
            x = loc.X; y = loc.Y;
        } else {
            return '';
        }
        if (!x && !y) return '';
        var btn = h('button', { class: 'map-link-btn', title: t('btn.show_on_map') }, '\u{1F4CD}');
        btn.addEventListener('click', function() { showOnMap(x, y); });
        return {__el: btn};
    }

    // card grid with optional fixed column count.
    // no cols = default auto-fill behaviour, cols = exact number of columns.
    // cards stay compact (max 230px) instead of stretching to fill the row.
    // The max-width formula prevents the grid from expanding beyond the
    // requested column count on wide screens.
    function cardGrid(cols) {
        if (cols) {
            // cols × 230px card + (cols-1) × 12px gap
            var maxW = cols * 230 + (cols - 1) * 12;
            return h('div', { class: 'card-grid', 'data-cols': cols, style: 'grid-template-columns: repeat(' + cols + ', minmax(0, 230px)); justify-content: start; max-width: ' + maxW + 'px;' });
        }
        return h('div', { class: 'card-grid' });
    }

    function tabDescription(text) {
        return h('div', { class: 'tab-description' },
            h('span', { class: 'icon', text: '\u24D8' }),
            h('span', { text: text })
        );
    }

    // Sync h2 header widths with content below so the underline matches the
    // content width exactly. Must be called after the tab is visible because
    // getBoundingClientRect() returns zeros for display:none elements.
    // - .card-section: h2 matches widest card row
    // - .sync-flex-header: h2 matches first flex row container width
    // - .sync-text-header: h2 matches widest <p> element
    // - .sync-details-header: h2 matches first row of <details> elements
    function syncCardHeaders(container) {
        // Flex header sections: h2 matches flex row width (cards + side panel)
        var flexSecs = container.querySelectorAll('.sync-flex-header');
        for (var fi = 0; fi < flexSecs.length; fi++) {
            var fh2 = flexSecs[fi].querySelector('h2');
            if (!fh2) continue;
            var flexRows = flexSecs[fi].querySelectorAll('[style*="display: flex"], [style*="display:flex"]');
            if (flexRows.length === 0) continue;
            var fRect = flexRows[0].getBoundingClientRect();
            if (fRect.width > 0) fh2.style.width = Math.round(fRect.width) + 'px';
        }

        // Card sections (skip if also sync-flex-header - already handled above)
        var sections = container.querySelectorAll('.card-section');
        for (var si = 0; si < sections.length; si++) {
            if (sections[si].classList.contains('sync-flex-header')) continue;
            var grids = sections[si].querySelectorAll('.card-grid');
            var h2 = sections[si].querySelector('h2');
            if (!h2 || grids.length === 0) continue;
            var maxWidth = 0;
            for (var gi = 0; gi < grids.length; gi++) {
                var cards = grids[gi].querySelectorAll('.card');
                if (cards.length === 0) continue;
                var firstRect = cards[0].getBoundingClientRect();
                var lastRect = firstRect;
                for (var ci = 1; ci < cards.length; ci++) {
                    var rect = cards[ci].getBoundingClientRect();
                    if (Math.abs(rect.top - firstRect.top) > 1) break;
                    lastRect = rect;
                }
                var width = Math.round(lastRect.right - firstRect.left);
                if (width > maxWidth) maxWidth = width;
            }
            if (maxWidth > 0) h2.style.width = maxWidth + 'px';
        }

        // Text header sections: h2 matches widest <p>
        var textSecs = container.querySelectorAll('.sync-text-header');
        for (var ti = 0; ti < textSecs.length; ti++) {
            var th2 = textSecs[ti].querySelector('h2');
            if (!th2) continue;
            var ps = textSecs[ti].querySelectorAll('p');
            var maxP = 0;
            for (var pi = 0; pi < ps.length; pi++) {
                var pRect = ps[pi].getBoundingClientRect();
                if (pRect.width > maxP) maxP = Math.round(pRect.width);
            }
            if (maxP > 0) th2.style.width = maxP + 'px';
        }

        // Details header sections: h2 matches first row of <details>
        var detSecs = container.querySelectorAll('.sync-details-header');
        for (var di = 0; di < detSecs.length; di++) {
            var dh2 = detSecs[di].querySelector('h2');
            if (!dh2) continue;
            var dets = detSecs[di].querySelectorAll('.auto-width-wrap > details');
            if (dets.length === 0) continue;
            var dFirstRect = dets[0].getBoundingClientRect();
            var dLastRect = dFirstRect;
            for (var dri = 1; dri < dets.length; dri++) {
                var dRect = dets[dri].getBoundingClientRect();
                if (Math.abs(dRect.top - dFirstRect.top) > 1) break;
                dLastRect = dRect;
            }
            var dWidth = Math.round(dLastRect.right - dFirstRect.left);
            if (dWidth > 0) dh2.style.width = dWidth + 'px';
        }
    }

    // paginatedTable creates a section with a paginated, filterable sortable table.
    // opts:
    //   rowBuilder: function(item) -> row array. If provided, items are mapped to rows.
    //   filterCols: array of column indices to search (default: all columns).
    //   extraFilters: [{ allLabel, values, labels, col, exact, match }] - dropdown filters.
    //     labels: optional translated display texts (defaults to values if omitted).
    //     match: function(row, selectedValue) -> bool. If present, used instead of col-based matching.
    //   extraButtons: [DOMElement] - extra buttons appended to the filter bar.
    //   container: DOMElement - use this instead of creating a section.
    //   deferRender: boolean - don't render the first page until .render() is called.
    //   (plus existing: sectionTitle, autoWidth, sectionAutoWidth,
    //    filterPlaceholder, scrollable, maxWidth, pageSize)
    function paginatedTable(items, headers, opts) {
        opts = opts || {};
        var pageSize = opts.pageSize || 100;

        // If rowBuilder is provided, pre-build rows from items.
        var rows = opts.rowBuilder ? items.map(opts.rowBuilder) : items;

        // Container: use provided element or create a section.
        var sec;
        if (opts.container) {
            sec = opts.container;
        } else {
            sec = section(opts.sectionTitle || t('section.details'));
        }
        if (opts.autoWidth || opts.sectionAutoWidth) sec.classList.add('auto-width');

        // sortCol -1 = backend order. keeping the sort state here instead of in
        // the DOM means it survives paging/filtering, and a header click sorts
        // everything rather than just the rows on screen.
        var state = { filtered: rows, page: 0, sortCol: -1, sortType: 'string', sortAsc: true };

        // Search/filter bar
        var filterBar = h('div', { class: 'filter-bar' });
        var inputWrap = h('div', { class: 'filter-input-wrap' });
        var searchInput = h('input', { type: 'text', placeholder: opts.filterPlaceholder || t('filter.placeholder') });
        var filterClearBtn = h('button', { class: 'filter-clear', title: t('filter.clear') }, '\u00d7');
        inputWrap.appendChild(searchInput);
        inputWrap.appendChild(filterClearBtn);
        filterBar.appendChild(h('span', {}, t('filter.label')));
        filterBar.appendChild(inputWrap);

        function updateFilterClear() {
            inputWrap.classList.toggle('has-text', searchInput.value.length > 0);
        }

        filterClearBtn.addEventListener('click', function(e) {
            e.stopPropagation();
            searchInput.value = '';
            updateFilterClear();
            applyFilter();
            searchInput.focus();
        });

        // Extra filter dropdowns (e.g. status select)
        var extraFilterEls = [];
        if (opts.extraFilters) {
            opts.extraFilters.forEach(function(ef) {
                var sel = h('select', {});
                sel.appendChild(h('option', { value: '' }, ef.allLabel || t('filter.all_statuses')));
                ef.values.forEach(function(v, i) {
                    var label = (ef.labels && ef.labels[i]) ? ef.labels[i] : v;
                    sel.appendChild(h('option', { value: label }, label));
                });
                filterBar.appendChild(sel);
                extraFilterEls.push({ select: sel, col: ef.col, exact: ef.exact, match: ef.match });
            });
        }

        // Extra buttons (e.g. locomotive info button)
        if (opts.extraButtons) {
            opts.extraButtons.forEach(function(btn) {
                filterBar.appendChild(btn);
            });
        }

        sec.appendChild(filterBar);

        var tableContainer = h('div', {});
        sec.appendChild(tableContainer);

        var pagerBar = h('div', { class: 'pager-bar' });
        var pageInfo = h('span', { class: 'pager-info' });
        var pagerButtons = h('div', { class: 'pager-buttons' });
        pagerBar.appendChild(pageInfo);
        pagerBar.appendChild(pagerButtons);
        sec.appendChild(pagerBar);

        function cellText(cell) {
            if (cell === null || cell === undefined) return '';
            if (typeof cell === 'string') return cell;
            if (cell && cell.__el) return cell.__el.textContent || '';
            if (cell && cell.__html !== undefined) return String(cell.__html).replace(/<[^>]*>/g, '');
            return String(cell);
        }

        // same rules as sortTable(): number columns get stripped of everything
        // non-numeric first, so "1,234 MW" compares as 1234.
        function applySort() {
            if (state.sortCol < 0) return;
            var col = state.sortCol;
            var numeric = state.sortType === 'number';
            var asc = state.sortAsc;
            state.filtered = state.filtered.slice().sort(function(ra, rb) {
                var va = cellText(ra[col]);
                var vb = cellText(rb[col]);
                if (numeric) {
                    va = parseFloat(va.replace(/[^0-9.-]/g, '')) || 0;
                    vb = parseFloat(vb.replace(/[^0-9.-]/g, '')) || 0;
                }
                if (va < vb) return asc ? -1 : 1;
                if (va > vb) return asc ? 1 : -1;
                return 0;
            });
        }

        // clicking the active column flips direction, a new column starts asc
        function onSort(colIdx, type) {
            if (state.sortCol === colIdx) {
                state.sortAsc = !state.sortAsc;
            } else {
                state.sortCol = colIdx;
                state.sortType = type;
                state.sortAsc = true;
            }
            applySort();
            state.page = 0;
            renderPage();
        }

        function renderPage() {
            tableContainer.innerHTML = '';
            var start = state.page * pageSize;
            var end = Math.min(start + pageSize, state.filtered.length);
            var pageRows = [];
            for (var i = start; i < end; i++) {
                pageRows.push(state.filtered[i]);
            }
            tableContainer.appendChild(sortableTable(
                headers,
                pageRows,
                {
                    scrollable: !!opts.scrollable,
                    maxWidth: opts.maxWidth,
                    autoWidth: !!opts.autoWidth,
                    onSort: onSort,
                    sortState: { col: state.sortCol, asc: state.sortAsc }
                }
            ));

            var totalPages = Math.ceil(state.filtered.length / pageSize);
            pageInfo.textContent = (start + 1) + '-' + end + ' / ' + state.filtered.length;
            pagerButtons.innerHTML = '';
            if (totalPages <= 1) {
                pagerBar.style.display = 'none';
            } else {
                pagerBar.style.display = '';
                var prevBtn = h('button', { disabled: state.page === 0 }, t('pager.prev'));
                prevBtn.addEventListener('click', function() { if (state.page > 0) { state.page--; renderPage(); } });
                pagerButtons.appendChild(prevBtn);

                var pageInput = h('span', { class: 'pager-page' }, (state.page + 1) + ' / ' + totalPages);
                pagerButtons.appendChild(pageInput);

                var nextBtn = h('button', { disabled: state.page >= totalPages - 1 }, t('pager.next'));
                nextBtn.addEventListener('click', function() { if (state.page < totalPages - 1) { state.page++; renderPage(); } });
                pagerButtons.appendChild(nextBtn);
            }
        }

        function applyFilter() {
            var q = searchInput.value.toLowerCase().trim();
            var hasExtraFilter = extraFilterEls.some(function(ef) { return ef.select.value !== ''; });

            if (!q && !hasExtraFilter) {
                state.filtered = rows;
            } else {
                state.filtered = rows.filter(function(row) {
                    // Check extra filters (exact or substring match on specific column)
                    for (var ei = 0; ei < extraFilterEls.length; ei++) {
                        var ef = extraFilterEls[ei];
                        if (ef.select.value === '') continue;
                        if (ef.match) {
                            if (!ef.match(row, ef.select.value)) return false;
                            continue;
                        }
                        var cellVal = cellText(row[ef.col]).toLowerCase();
                        if (ef.exact) {
                            if (cellVal !== ef.select.value.toLowerCase()) return false;
                        } else {
                            if (cellVal.indexOf(ef.select.value.toLowerCase()) < 0) return false;
                        }
                    }
                    // Text search
                    if (!q) return true;
                    if (opts.filterCols) {
                        for (var ci = 0; ci < opts.filterCols.length; ci++) {
                            if (cellText(row[opts.filterCols[ci]]).toLowerCase().indexOf(q) >= 0) return true;
                        }
                        return false;
                    } else {
                        return row.some(function(cell) { return cellText(cell).toLowerCase().indexOf(q) >= 0; });
                    }
                });
            }
            applySort();
            state.page = 0;
            renderPage();
        }

        searchInput.addEventListener('input', function() { updateFilterClear(); applyFilter(); });
        extraFilterEls.forEach(function(ef) {
            ef.select.addEventListener('change', applyFilter);
        });

        if (!opts.deferRender) {
            renderPage();
        }

        sec.render = renderPage;
        return sec;
    }

    function sortableTable(headers, rows, opts) {
        opts = opts || {};
        var table = document.createElement('table');
        table.setAttribute('data-sortable', 'true');
        if (opts.maxWidth) table.style.maxWidth = opts.maxWidth;
        if (opts.autoWidth) table.style.width = 'auto';
        var hasWidths = headers.some(function(h) { return h.width; });
        if (hasWidths) table.className = 'cols-fixed';

        var thead = document.createElement('thead');
        var tr = document.createElement('tr');
        headers.forEach(function(hdr, i) {
            var th = document.createElement('th');
            th.textContent = typeof hdr === 'string' ? hdr : (hdr.label || '');
            th.dataset.col = i;
            th.dataset.type = hdr.type || 'string';
            if (hdr.cls) th.className = hdr.cls;
            if (hdr.width) {
                th.style.width = hdr.width;
            }
            // paginatedTable passes onSort because it sorts its full dataset
            // itself. without it, just reorder the rows in the DOM - fine for
            // tables that aren't paginated.
            if (opts.onSort) {
                if (opts.sortState && opts.sortState.col === i) {
                    th.classList.add(opts.sortState.asc ? 'sort-asc' : 'sort-desc');
                }
                th.addEventListener('click', function() {
                    opts.onSort(i, hdr.type || 'string');
                });
            } else {
                th.addEventListener('click', function() {
                    sortTable(table, i, hdr.type || 'string', th);
                });
            }
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

    // Sorts table rows by a column. Clicking a sorted column toggles direction.
    // Number columns strip non-numeric characters (e.g. "123 MW", "4,567")
    // before parsing, so formatted values sort correctly.
    function sortTable(table, colIdx, type, th) {
        var tbody = table.querySelector('tbody');
        var rows = Array.from(tbody.querySelectorAll('tr'));
        var ascending = !th.classList.contains('sort-asc');

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