    let DATA = null;
    let currentTab = 'overview';
    let previousTab = null;
    // Remembered scroll position per tab, so going back returns to where the
    // user was instead of the top of the tab.
    let tabScrollPositions = {};

    var FULL_TABS  = ['overview', 'structures', 'buildings', 'clockspeeds', 'power',
                      'production', 'manufacturing', 'storage', 'logistics', 'transport',
                      'nuclear', 'collectibles', 'extras', 'map'];

    function getVisibleTabs() {
        return FULL_TABS;
    }

    var ALL_TABS = [
        {id: 'overview', labelKey: 'tab.overview', render: renderOverview},
        {id: 'structures', labelKey: 'tab.structures', render: renderStructures},
        {id: 'buildings', labelKey: 'tab.buildings', render: renderBuildings},
        {id: 'clockspeeds', labelKey: 'tab.clockspeeds', render: renderClockSpeeds},
        {id: 'power', labelKey: 'tab.power', render: renderPower},
        {id: 'production', labelKey: 'tab.production', render: renderExtraction},
        {id: 'manufacturing', labelKey: 'tab.manufacturing', render: renderManufacturing},
        {id: 'storage', labelKey: 'tab.storage', render: renderStorage},
        {id: 'logistics', labelKey: 'tab.logistics', render: renderLogistics},
        {id: 'transport', labelKey: 'tab.transport', render: renderTransport},
        {id: 'nuclear', labelKey: 'tab.nuclear', render: renderNuclear},
        {id: 'collectibles', labelKey: 'tab.collectibles', render: renderCollectibles},
        {id: 'extras', labelKey: 'tab.extras', render: renderExtras},
        {id: 'map', labelKey: 'tab.map', render: renderMap}
    ];

    var TABS = ALL_TABS;

    function setupTabs() {
        var nav = document.getElementById('tabs');
        var visible = getVisibleTabs();
        TABS = ALL_TABS.filter(function(tab) {
            return visible.indexOf(tab.id) >= 0;
        });
        TABS.forEach(function(tab) {
            var btn = h('button', { class: 'tab-btn', 'data-tab': tab.id, text: t(tab.labelKey) });
            btn.addEventListener('click', function() { switchTab(tab.id); });
            nav.appendChild(btn);
        });
    }

    function switchTab(tabId) {
        if (currentTab !== tabId) {
            // Remember where we were before leaving this tab
            tabScrollPositions[currentTab] = window.scrollY || 0;
            previousTab = currentTab;
        }
        currentTab = tabId;
        document.querySelectorAll('.tab-btn').forEach(function(b) {
            b.classList.toggle('active', b.dataset.tab === tabId);
        });
        document.querySelectorAll('.tab-content').forEach(function(c) {
            c.classList.toggle('active', c.id === 'tab-' + tabId);
        });
        // Update map back button visibility
        if (tabId === 'map') {
            var backBtn = document.getElementById('mapBackBtn');
            if (backBtn) {
                if (previousTab && previousTab !== 'map') {
                    backBtn.style.display = '';
                    backBtn.onclick = function() { switchTab(previousTab); };
                } else {
                    backBtn.style.display = 'none';
                }
            }
        }
        // Re-apply current global search filter to the newly-active tab
        var search = document.getElementById('search');
        var q = search ? search.value.toLowerCase().trim() : '';
        filterRows(q);
        // Restore remembered scroll position (e.g. after going back),
        // otherwise start at top of tab
        var targetY = tabScrollPositions[tabId];
        if (typeof targetY === 'number' && targetY > 0) {
            window.scrollTo(0, targetY);
        } else {
            window.scrollTo(0, 0);
        }
        // syncCardHeaders needs the tab to be visible to measure element rects.
        // The 50ms delay lets the browser apply the 'active' class and lay out
        // the content before we measure.
        var active = document.getElementById('tab-' + tabId);
        if (active) setTimeout(function() { syncCardHeaders(active); }, 50);
    }

    // ===== Global Search =====

    var searchDebounceTimer = null;

    function setupSearch() {
        var input = document.getElementById('search');
        var dropdown = document.getElementById('search-dropdown');
        var clearBtn = document.getElementById('search-clear');
        var wrap = document.querySelector('.search-wrap');
        if (!input || !dropdown) return;

        function updateClearVisibility() {
            if (wrap) wrap.classList.toggle('has-text', input.value.length > 0);
        }

        input.addEventListener('input', function() {
            updateClearVisibility();
            if (searchDebounceTimer) clearTimeout(searchDebounceTimer);
            searchDebounceTimer = setTimeout(function() {
                runGlobalSearch(input.value);
            }, 300);
        });

        input.addEventListener('keydown', function(e) {
            if (e.key === 'Enter') {
                if (searchDebounceTimer) clearTimeout(searchDebounceTimer);
                runGlobalSearch(input.value);
            } else if (e.key === 'Escape') {
                input.value = '';
                updateClearVisibility();
                dropdown.classList.remove('visible');
                filterRows('');
                input.blur();
            }
        });

        if (clearBtn) {
            clearBtn.addEventListener('click', function(e) {
                e.stopPropagation();
                input.value = '';
                updateClearVisibility();
                dropdown.classList.remove('visible');
                filterRows('');
                input.focus();
            });
        }

        // Close dropdown on outside click
        document.addEventListener('click', function(e) {
            if (!e.target.closest('.search-wrap')) {
                dropdown.classList.remove('visible');
            }
        });

        // Re-open dropdown on focus if there's a query
        input.addEventListener('focus', function() {
            if (input.value.trim()) runGlobalSearch(input.value);
        });
    }

    function runGlobalSearch(rawQuery) {
        var input = document.getElementById('search');
        var dropdown = document.getElementById('search-dropdown');
        if (!input || !dropdown) return;

        var q = rawQuery.toLowerCase().trim();
        dropdown.innerHTML = '';

        if (!q) {
            dropdown.classList.remove('visible');
            filterRows('');
            return;
        }

        // Collect results across all visible tabs
        var results = [];
        var totalMatches = 0;

        TABS.forEach(function(tab) {
            var content = document.getElementById('tab-' + tab.id);
            if (!content) return;

            var tier1 = []; // section titles, card titles+values, badge text
            var rowMatchCount = 0;

            // Tier 1: section titles
            content.querySelectorAll('.section h2').forEach(function(h2) {
                var text = h2.textContent.toLowerCase();
                if (text.includes(q)) {
                    var sec = h2.closest('.section');
                    tier1.push({
                        label: h2.textContent,
                        snippet: t('search.section'),
                        tabId: tab.id,
                        tabLabel: t(tab.labelKey),
                        element: sec || h2
                    });
                }
            });

            // Tier 1: card values + titles
            content.querySelectorAll('.card-grid > .card').forEach(function(card) {
                var text = card.textContent.toLowerCase();
                if (text.includes(q)) {
                    // Extract a meaningful snippet
                    var val = card.querySelector('.value');
                    var label = card.querySelector('.label');
                    var snippet = (label ? label.textContent : '') + (val ? ': ' + val.textContent : '');
                    tier1.push({
                        label: (label ? label.textContent : card.textContent.substring(0, 60)),
                        snippet: snippet || t('search.card'),
                        tabId: tab.id,
                        tabLabel: t(tab.labelKey),
                        element: card
                    });
                }
            });

            // Tier 1: tab description text
            content.querySelectorAll('.tab-description').forEach(function(td) {
                var text = td.textContent.toLowerCase();
                if (text.includes(q)) {
                    tier1.push({
                        label: td.textContent.substring(0, 80),
                        snippet: t('search.tab_description'),
                        tabId: tab.id,
                        tabLabel: t(tab.labelKey),
                        element: td
                    });
                }
            });

            // Tier 2: count table row matches (don't list them individually)
            content.querySelectorAll('tbody tr').forEach(function(tr) {
                if (tr.textContent.toLowerCase().includes(q)) rowMatchCount++;
            });

            if (tier1.length > 0 || rowMatchCount > 0) {
                results.push({
                    tabId: tab.id,
                    tabLabel: t(tab.labelKey),
                    tier1: tier1,
                    rowCount: rowMatchCount
                });
                totalMatches += tier1.length + rowMatchCount;
            }
        });

        if (totalMatches === 0) {
            dropdown.innerHTML = '<div class="sd-empty">' + t('search.no_results', esc(rawQuery.trim())) + '</div>';
            dropdown.classList.add('visible');
            return;
        }

        // Build dropdown content
        results.forEach(function(grp) {
            var group = h('div', { class: 'sd-group' });
            var header = h('div', { class: 'sd-group-header' },
                grp.tabLabel,
                h('span', { class: 'sd-group-count' },
                  grp.tier1.length + ' ' + (grp.tier1.length !== 1 ? t('search.results') : t('search.result')) +
                  (grp.rowCount > 0 ? ' · ' + grp.rowCount + ' ' + (grp.rowCount !== 1 ? t('search.row_matches') : t('search.row_match')) : '')));

            group.appendChild(header);

            // Tier 1 results (max 10 per tab to keep dropdown readable)
            var shown = grp.tier1.slice(0, 10);
            shown.forEach(function(r) {
                var item = h('div', { class: 'sd-result' });
                item.appendChild(h('div', {}, r.label));
                if (r.snippet && r.snippet !== r.label) {
                    item.appendChild(h('div', { class: 'sd-snippet' }, r.snippet));
                }
                item.addEventListener('click', function() {
                    jumpToResult(r);
                });
                group.appendChild(item);
            });

            if (grp.tier1.length > 10) {
                group.appendChild(h('div', { class: 'sd-row-count' },
                    t('search.more_results', grp.tier1.length - 10)));
            }

            // Tier 2: row count
            if (grp.rowCount > 0 && grp.tier1.length === 0) {
                // Only tab with row matches - make it clickable
                var item = h('div', { class: 'sd-result' },
                    h('div', {}, grp.rowCount + ' ' + (grp.rowCount !== 1 ? t('search.row_matches') : t('search.row_match'))),
                    h('div', { class: 'sd-snippet' }, t('search.click_to_filter')));
                item.addEventListener('click', function() {
                    switchTab(grp.tabId);
                    filterRows(q);
                    dropdown.classList.remove('visible');
                });
                group.appendChild(item);
            } else if (grp.rowCount > 0) {
                group.appendChild(h('div', { class: 'sd-row-count' },
                    grp.rowCount + ' ' + (grp.rowCount !== 1 ? t('search.row_matches') : t('search.row_match')) + ' ' + t('search.shown_when_visit')));
            }

            dropdown.appendChild(group);
        });

        dropdown.classList.add('visible');
    }

    function jumpToResult(r) {
        var dropdown = document.getElementById('search-dropdown');
        var input = document.getElementById('search');
        var q = input ? input.value.toLowerCase().trim() : '';

        // Switch to the tab
        switchTab(r.tabId);

        // Filter the tab so user sees exactly where matches are
        filterRows(q);

        // Close dropdown
        dropdown.classList.remove('visible');

        // Scroll to the element and highlight it briefly
        setTimeout(function() {
            if (r.element && r.element.scrollIntoView) {
                r.element.scrollIntoView({ behavior: 'smooth', block: 'center' });
                // Brief highlight
                var orig = r.element.style.transition;
                var origBg = r.element.style.backgroundColor;
                r.element.style.transition = 'background-color 0.5s';
                r.element.style.backgroundColor = 'rgba(0, 212, 255, 0.2)';
                setTimeout(function() {
                    r.element.style.backgroundColor = origBg || '';
                    setTimeout(function() { r.element.style.transition = orig || ''; }, 500);
                }, 1500);
            }
        }, 100);
    }

    function filterRows(query) {
        var active = document.querySelector('.tab-content.active');
        if (!active) return;

        if (!query) {
            // Restore everything
            active.querySelectorAll('tbody tr').forEach(function(tr) { tr.style.display = ''; });
            active.querySelectorAll('div.section, details').forEach(function(el) { el.style.display = ''; });
            return;
        }

        // Filter table rows
        active.querySelectorAll('tbody tr').forEach(function(tr) {
            var text = tr.textContent.toLowerCase();
            tr.style.display = text.includes(query) ? '' : 'none';
        });

        // Hide sections (div.section) with no visible rows
        active.querySelectorAll('div.section').forEach(function(sec) {
            var hasVisible = false;
            // Check tables
            sec.querySelectorAll('table').forEach(function(tbl) {
                var rows = tbl.querySelectorAll('tbody tr');
                for (let i = 0; i < rows.length; i++) {
                    if (rows[i].style.display !== 'none') { hasVisible = true; break; }
                }
            });
            // Check card-grid content
            if (!hasVisible) {
                var cards = sec.querySelectorAll('.card-grid > *');
                for (let i = 0; i < cards.length; i++) {
                    if (cards[i].textContent.toLowerCase().includes(query)) { hasVisible = true; break; }
                }
            }
            // Check nested details with visible rows
            if (!hasVisible) {
                var dets = sec.querySelectorAll('details');
                for (let i = 0; i < dets.length; i++) {
                    var detRows = dets[i].querySelectorAll('tbody tr');
                    for (var j = 0; j < detRows.length; j++) {
                        if (detRows[j].style.display !== 'none') { hasVisible = true; break; }
                    }
                    if (hasVisible) break;
                }
            }
            sec.style.display = hasVisible ? '' : 'none';
        });

        // Hide standalone details elements with no visible rows
        active.querySelectorAll('details').forEach(function(det) {
            var rows = det.querySelectorAll('tbody tr');
            var hasVisible = false;
            for (var i = 0; i < rows.length; i++) {
                if (rows[i].style.display !== 'none') { hasVisible = true; break; }
            }
            // Also check summary text for non-table details (e.g. tapes)
            if (!hasVisible && rows.length === 0) {
                hasVisible = det.textContent.toLowerCase().includes(query);
            }
            det.style.display = hasVisible ? '' : 'none';
        });
    }

    // ===== Empty Tab/Section Hiding =====

    // a section always renders its h2, so emptiness has to be judged from what's
    // below the heading. used by both hideEmptyTabs() and hideEmptySections() so
    // they can't disagree.
    var ZERO_VALUES = ['0', '0 MW', '0 km', '0 / 0', '0.0 km', '0.00 km'];

    function isZeroValue(text) {
        if (!text) return false;
        // Strip leading + or - so "+0 MW" and "-0 MW" are caught
        var normalized = text.replace(/^[+-]/, '');
        return ZERO_VALUES.indexOf(normalized) >= 0;
    }

    function sectionHasContent(sec) {
        // any table row counts
        if (sec.querySelector('table tbody tr')) return true;

        // a card counts if its value isn't just a zero placeholder. some cards
        // have no value at all and put everything in the description (the Clock
        // Speeds legend) - those count too.
        var cards = sec.querySelectorAll('.card-grid > .card');
        for (var i = 0; i < cards.length; i++) {
            var val = cards[i].querySelector('.value');
            var text = val ? val.textContent.trim() : '';
            if (text && !isZeroValue(text)) return true;
            if (!text) {
                var desc = cards[i].querySelector('.desc');
                if (desc && desc.textContent.trim()) return true;
            }
        }

        if (sec.querySelector('details')) return true;

        // a bar that's actually filled in
        var fills = sec.querySelectorAll('.bar-fill');
        for (var f = 0; f < fills.length; f++) {
            var w = fills[f].style.width;
            if (w && w !== '0%') return true;
        }

        // text-only sections (like the Power tab's locomotive note) have none of
        // the above but still count. only checked when there's no card grid or
        // table around, so an all-zero grid can't pass on its labels alone.
        if (!sec.querySelector('.card-grid, table, details')) {
            var heading = sec.querySelector('h2');
            var body = sec.textContent;
            if (heading) body = body.replace(heading.textContent, '');
            if (body.trim()) return true;
        }

        return false;
    }

    // hide the tab button when none of the tab's sections hold data
    function hideEmptyTabs() {
        TABS.forEach(function(tab) {
            if (tab.id === 'overview') return;
            if (tab.id === 'map') {
                // Hide map tab when no map data was embedded in the report
                var mapEl = document.getElementById('map-objects');
                if (!DATA.mapData || !mapEl || !mapEl.textContent.trim()) {
                    var mapBtn = document.querySelector('.tab-btn[data-tab="map"]');
                    if (mapBtn) mapBtn.style.display = 'none';
                }
                return;
            }
            var content = document.getElementById('tab-' + tab.id);
            if (!content) return;
            var hasContent = false;
            content.querySelectorAll('.section').forEach(function(sec) {
                if (!hasContent && sectionHasContent(sec)) hasContent = true;
            });
            if (!hasContent) {
                var btn = document.querySelector('.tab-btn[data-tab="' + tab.id + '"]');
                if (btn) btn.style.display = 'none';
            }
        });
    }

    // tag empty sections with .is-empty. has to be a class, not an inline style:
    // filterRows() wipes inline display values when the search box is cleared,
    // which would pop these sections back into view.
    function hideEmptySections(container) {
        if (!container) return;
        container.querySelectorAll('.section').forEach(function(sec) {
            if (!sectionHasContent(sec)) sec.classList.add('is-empty');
        });
    }

    // ===== Language Toggle =====

    function setupLanguageToggle() {
        var flags = document.querySelectorAll('.lang-flag');
        if (!flags.length) return;
        flags.forEach(function(flag) {
            flag.addEventListener('click', function() {
                switchLanguage(flag.getAttribute('data-lang'));
            });
        });
        updateFlagActive();
    }

    function updateFlagActive() {
        var flags = document.querySelectorAll('.lang-flag');
        flags.forEach(function(flag) {
            var lang = flag.getAttribute('data-lang');
            flag.classList.toggle('active', lang === LANG);
        });
    }

    function applyI18n() {
        // Translate elements with data-i18n attributes
        document.querySelectorAll('[data-i18n]').forEach(function(el) {
            el.textContent = t(el.getAttribute('data-i18n'));
        });
        document.querySelectorAll('[data-i18n-placeholder]').forEach(function(el) {
            el.placeholder = t(el.getAttribute('data-i18n-placeholder'));
        });
        document.querySelectorAll('[data-i18n-title]').forEach(function(el) {
            el.title = t(el.getAttribute('data-i18n-title'));
        });
    }

    function switchLanguage(newLang) {
        if (newLang !== 'en' && newLang !== 'de') return;
        if (newLang === LANG) return;
        LANG = newLang;
        updateFlagActive();

        // Translate static HTML elements
        applyI18n();

        // Update title
        var hdr = DATA.header || {};
        document.getElementById('title').textContent = hdr.SaveName || hdr.SessionName || t('header.title');

        // Rebuild tab buttons with new labels
        var nav = document.getElementById('tabs');
        nav.innerHTML = '';
        setupTabs();

        // Re-render all tabs
        TABS.forEach(function(tab) { tab.render(); });

        // Hide empty tabs and sections
        hideEmptyTabs();
        TABS.forEach(function(tab) {
            hideEmptySections(document.getElementById('tab-' + tab.id));
        });

        // Re-activate current tab
        switchTab(currentTab);

        // Re-apply search filter if any
        if (search && search.value.trim()) {
            runGlobalSearch(search.value);
        }
    }

    // ===== Init =====

    async function init() {
        var loading = document.getElementById('loading');
        var app = document.getElementById('app');
        try {
            DATA = await loadData();
            loading.style.display = 'none';
            app.style.display = 'block';

            // Set title
            var hdr = DATA.header || {};
            document.getElementById('title').textContent = hdr.SaveName || hdr.SessionName || t('header.title');

            // Translate static HTML elements (footer, search, etc.)
            applyI18n();

            // Setup tabs
            setupTabs();

            // Render all tabs
            TABS.forEach(function(tab) { tab.render(); });

            // Hide empty tabs and sections
            hideEmptyTabs();
            TABS.forEach(function(tab) {
                hideEmptySections(document.getElementById('tab-' + tab.id));
            });

            // Activate first tab
            switchTab('overview');

            // Setup search
            setupSearch();

            // Setup language toggle
            setupLanguageToggle();

        } catch (err) {
            loading.style.display = 'block';
            loading.innerHTML = '<p style="color: #ff6464; font-size: 1.2em;">' + t('search.error_loading', esc(err.message)) + '</p>';
            console.error('SatisFacts:', err);
        }
    }

    init();