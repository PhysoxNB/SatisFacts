    function renderBuildings() {
        var c = document.getElementById('tab-buildings');
        c.innerHTML = '';
        var d = DATA;
        c.appendChild(tabDescription(t('desc.tab_buildings')));
        var buildings = d.buildings || {};

        var buildingKeys = Object.keys(buildings).filter(function(type) { return buildings[type] > 0; });

        // Categorize by class name. Power is checked before Production
        // because 'generator' would also match the production check.
        var categories = {
            'Production': {},
            'Power': {},
            'Mining': {},
            'Storage': {},
            'Transport': {},
            'Utility': {},
            'Other': {}
        };
        var catLabelKeys = {
            'Production': 'cat.production',
            'Power': 'cat.power',
            'Mining': 'cat.mining',
            'Storage': 'cat.storage',
            'Transport': 'cat.transport',
            'Utility': 'cat.utility',
            'Other': 'cat.other'
        };

        // Pipe/belt parts are in d.buildings because other tabs need them,
        // but they're not buildings – keep them out of the Other bucket.
        function isNotABuilding(lower) {
            return lower.includes('flowindicator');
        }

        buildingKeys.forEach(function(type) {
            var lower = type.toLowerCase();
            if (isNotABuilding(lower)) return;
            var cat = null;

            // Power first – 'generator' would also match Production
            if (lower.includes('generator') || lower.includes('powerstorage') || lower.includes('alienpower')) {
                cat = 'Power';
            }
            // Mining
            else if (lower.includes('miner') || lower.includes('fracking') || lower.includes('oilpump') || lower.includes('waterpump') || lower.includes('resourcecollector')) {
                cat = 'Mining';
            }
            // Storage
            else if (lower.includes('storage') || lower.includes('container') || lower.includes('centralstorage') || lower.includes('industrialtank') || lower.includes('pipestoragetank')) {
                cat = 'Storage';
            }
            // Transport (trains, signals, stations, tracks)
            else if (lower.includes('train') || lower.includes('railroad')) {
                cat = 'Transport';
            }
            // Production
            else if (lower.includes('assembler') || lower.includes('smelter') || lower.includes('foundry') ||
                lower.includes('constructor') || lower.includes('manufacturer') || lower.includes('refinery') ||
                lower.includes('blender') || lower.includes('packager') || lower.includes('converter') ||
                lower.includes('hadron') || lower.includes('collider') ||
                lower.includes('portal') || lower.includes('accelerator') || lower.includes('quantum') ||
                lower.includes('encoder')) {
                cat = 'Production';
            }

            // Utility (blueprint designers, workstations)
            else if (lower.includes('blueprintdesigner')) {
                cat = 'Utility';
            }

            // Anything unrecognised goes in Other instead of vanishing
            categories[cat || 'Other'][type] = buildings[type];
        });

        var catOrder = ['Production', 'Power', 'Mining', 'Storage', 'Transport', 'Utility', 'Other'];

        // build sections for non-empty categories
        var sections = [];
        catOrder.forEach(function(catName) {
            var catBuildings = categories[catName];
            var catKeys = Object.keys(catBuildings);
            if (catKeys.length === 0) return;

            var catTotal = 0;
            catKeys.forEach(function(type) { catTotal += catBuildings[type]; });

            var s = section(t(catLabelKeys[catName]) + ' (' + fmt(catTotal) + ')');
            s.classList.add('auto-width');
            var rows = catKeys.map(function(type) {
                return [html(iconName(type)), fmt(catBuildings[type])];
            }).sort(byNumDesc(1));
            s.appendChild(sortableTable(
                [{label: t('col.building'), width: '280px'},
                {label: t('col.count'), type: 'number', width: '80px'}],
                rows,
                {scrollable: rows.length > 20, autoWidth: true}
            ));
            sections.push(s);
        });

        // auto-width sections wrap side-by-side naturally
        var wrap = wrapRow();
        sections.forEach(function(s) { wrap.appendChild(s); });
        c.appendChild(wrap);
    }