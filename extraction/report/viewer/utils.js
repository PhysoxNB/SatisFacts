    // Current UI language: 'en' or 'de'. Changed by the language toggle.
    var LANG = 'en';

    // t() looks up a UI string in the current language's UI strings table.
    // Supports {0}, {1}, ... placeholders for dynamic values.
    function t(key) {
        var table = (LANG === 'de' && DATA && DATA.uiStringsDe) ? DATA.uiStringsDe : (DATA && DATA.uiStrings ? DATA.uiStrings : {});
        var val = table[key];
        if (val === undefined) {
            // Fallback to English table if missing from DE
            if (LANG === 'de' && DATA && DATA.uiStrings) val = DATA.uiStrings[key];
            if (val === undefined) val = key;
        }
        // Replace {0}, {1}, ... with arguments
        if (arguments.length > 1) {
            for (var i = 1; i < arguments.length; i++) {
                val = val.replace('{' + (i - 1) + '}', arguments[i]);
            }
        }
        return val;
    }

    // translateReason() maps English reason strings from the Go backend to
    // t() keys so they get translated in the current language.
    var REASON_MAP = {
        'Not connected to power network': 'reason.not_connected',
        'Disabled/Paused': 'reason.disabled_paused',
        'No Fuel / Not Producing': 'reason.no_fuel',
        'Fuel-Starved': 'reason.fuel_starved',
        'Output full (waiting for space)': 'reason.output_full',
        'No output belt/pipe connected': 'reason.no_output',
        'Manually paused': 'reason.manually_paused',
        'Manually paused or no recipe': 'reason.manually_paused_no_recipe',
        'No input items (waiting for material)': 'reason.no_input',
        'Active': 'reason.active',
        'Standby (not actively extracting)': 'reason.standby_extracting',
        'Standby (not actively producing)': 'reason.standby_producing',
        'Low power consumption (waiting for input)': 'reason.low_power_waiting',
        'Actively producing': 'reason.actively_producing',
        'No recipe set': 'reason.no_recipe',
        'No power connection': 'reason.no_power_conn',
        'Unknown status': 'reason.unknown_status',
        'On a rail circuit with no power source': 'reason.rail_no_source',
        'Not connected to a power circuit or rail network': 'reason.rail_not_connected',
        'Transferring cargo': 'reason.transferring_cargo',
        'Powered, no cargo transfer': 'reason.rail_idle',
        'Train docked': 'reason.train_docked',
        'Powered, waiting for a train': 'reason.rail_waiting'
    };

    function translateReason(reason) {
        if (!reason) return '';
        // Check dynamic patterns first: "Producing at X% clock" / "Extracting at X% clock"
        var m = reason.match(/^(Producing at|Extracting at) (.*) clock$/);
        if (m) {
            var key = m[1] === 'Producing at' ? 'reason.producing_at' : 'reason.extracting_at';
            return t(key, m[2]);
        }
        // Static reasons
        var mapped = REASON_MAP[reason];
        return mapped ? t(mapped) : reason;
    }

    var STATUS_MAP = {
        'Running': 'status.running',
        'Standby': 'status.standby',
        'Idle': 'status.idle',
        'Paused': 'status.paused',
        'Blocked': 'status.blocked',
        'Starving': 'status.starving',
        'Unpowered': 'status.unpowered',
        'Unknown': 'status.unknown'
    };

    function translateStatus(status) {
        if (!status) return t('status.unknown');
        var key = STATUS_MAP[status];
        return key ? t(key) : status;
    }

    var BELT_LABEL_MAP = {
        'Belt': 'label.belt',
        'Lift': 'label.lift',
        'Pipe': 'label.pipe',
        'None': 'label.none'
    };

    function translateBeltLabel(label) {
        if (!label) return t('label.none');
        // Labels can be comma-separated: "Belt Mk3, Belt Mk2"
        return label.split(',').map(function(part) {
            part = part.trim();
            var m = part.match(/^(Belt|Lift|Pipe) Mk(\d+)$/);
            if (m) {
                var baseKey = m[1] === 'Belt' ? 'label.belt' : (m[1] === 'Lift' ? 'label.lift' : 'label.pipe');
                return t(baseKey) + ' Mk' + m[2];
            }
            var mapped = BELT_LABEL_MAP[part];
            return mapped ? t(mapped) : part;
        }).join(', ');
    }

    // itemDisplayNamesTable() returns the current language's class->name map.
    var DRONE_STATE_MAP = {
        'Docked': 'drone.docked',
        'Unloading': 'drone.unloading',
        'Loading': 'drone.loading',
        'Unload': 'drone.unload',
        'Load': 'drone.load'
    };

    function translateDroneState(stateText) {
        if (!stateText) return '';
        if (stateText === 'In Transit') return t('status.in_transit');
        var stripped = stateText.replace(/^DS_/, '').replace(/([A-Z])/g, ' $1').trim();
        var key = DRONE_STATE_MAP[stripped];
        return key ? t(key) : stripped;
    }

    var TRANSFER_STAGE_MAP = {
        'Unload': 'transfer.unload',
        'Load': 'transfer.load'
    };

    function translateTransferStage(stageText) {
        if (!stageText) return '';
        var stripped = stageText.replace(/^ITS_/, '').replace(/([A-Z])/g, ' $1').trim();
        var key = TRANSFER_STAGE_MAP[stripped];
        return key ? t(key) : stripped;
    }

    var VEHICLE_STATION_STATUS_MAP = {
        'Loading': 'vehicle.loading',
        'Unloading': 'vehicle.unloading',
        'Waiting': 'vehicle.waiting',
        'Docked': 'vehicle.docked'
    };

    function translateVehicleStationStatus(statusText) {
        if (!statusText) return t('status.idle');
        if (statusText.indexOf('NoPower') >= 0 || statusText.indexOf('No Power') >= 0) return t('status.no_power');
        if (statusText.indexOf('Idle') >= 0 || statusText === '') return t('status.idle');
        var stripped = statusText.replace(/^DSS_/, '').replace(/([A-Z])/g, ' $1').trim();
        var key = VEHICLE_STATION_STATUS_MAP[stripped];
        return key ? t(key) : stripped;
    }

    function itemDisplayNamesTable() {
        if (!DATA) return null;
        if (LANG === 'de' && DATA.itemDisplayNamesDe) return DATA.itemDisplayNamesDe;
        return DATA.itemDisplayNames;
    }

    function fmt(n) {
        if (n === null || n === undefined) return '0';
        return Number(n).toLocaleString('en-US');
    }

    function fmtKm(m) {
        if (!m) return '0 km';
        return (m / 1000).toFixed(2) + ' km';
    }

    function fmtMw(mw) {
        if (mw === null || mw === undefined) return '0 MW';
        // -1 is a sentinel from the backend meaning "power unknown" (e.g. a
        // building with no power connection and no recipe to estimate from)
        if (mw === -1) return t('status.unknown');
        return fmt(Math.round(mw * 10) / 10) + ' MW';
    }

    function fmtPct(p) {
        if (p === null || p === undefined) return '0%';
        return Number(p).toFixed(1) + '%';
    }

    // em dash for empty table cells
    var DASH = '\u2014';

    // location formatter. backend sends {X,Y,Z} objects on most tabs and
    // [x,y,z] arrays on transport. arrays get rounded, objects don't - leave
    // that alone, the column widths depend on it. '' if there's no location.
    function fmtLoc(loc) {
        if (!loc) return '';
        if (Array.isArray(loc)) {
            if (loc.length < 3) return '';
            return Math.round(loc[0]) + ', ' + Math.round(loc[1]) + ' (' + t('label.altitude', Math.round(loc[2])) + ')';
        }
        if (loc.X === undefined) return '';
        return loc.X + ', ' + loc.Y + ' (' + t('label.altitude', loc.Z) + ')';
    }

    // "biggest first" comparator for columns holding formatted numbers ("1,234")
    function byNumDesc(idx) {
        return function(a, b) {
            return parseInt(String(b[idx]).replace(/,/g, '')) - parseInt(String(a[idx]).replace(/,/g, ''));
        };
    }

    function fmtTime(seconds) {
        if (!seconds) return t('status.unknown');
        const s = parseInt(seconds);
        const d = Math.floor(s / 86400);
        const h = Math.floor((s % 86400) / 3600);
        const m = Math.floor((s % 3600) / 60);
        if (d > 0) return d + 'd ' + h + 'h ' + m + 'm';
        if (h > 0) return h + 'h ' + m + 'm';
        return m + 'm';
    }

    function gameVersion(saveVersion) {
        var v = parseInt(saveVersion);
        if (v >= 60) return '1.2';
        if (v >= 51) return '1.1';
        if (v >= 37) return '1.0';
        return 'Pre-1.0';
    }

    function fmtDate(ts) {
        if (!ts) return t('status.unknown');
        var d = new Date(typeof ts === 'string' ? parseInt(ts) : ts);
        if (isNaN(d.getTime())) return t('status.unknown');
        var yyyy = d.getFullYear();
        var mm = String(d.getMonth() + 1).padStart(2, '0');
        var dd = String(d.getDate()).padStart(2, '0');
        return yyyy + '-' + mm + '-' + dd;
    }

    function fmtClock(ts) {
        if (!ts) return '';
        var d = new Date(typeof ts === 'string' ? parseInt(ts) : ts);
        if (isNaN(d.getTime())) return '';
        var h12 = d.getHours() % 12 || 12;
        var mi = String(d.getMinutes()).padStart(2, '0');
        var ss = String(d.getSeconds()).padStart(2, '0');
        var ampm = d.getHours() < 12 ? 'AM' : 'PM';
        return h12 + ':' + mi + ':' + ss + ' ' + ampm;
    }

    var STARTING_LOCATION_MAP = {
        'Grass Fields': 'location.grass_fields',
        'Rocky Desert': 'location.rocky_desert',
        'Northern Forest': 'location.northern_forest',
        'Dune Desert': 'location.dune_desert'
    };

    function translateStartingLocation(loc) {
        if (!loc) return '';
        var key = STARTING_LOCATION_MAP[loc];
        return key ? t(key) : loc;
    }

    // Converts a UE class path like "/Game/FactoryGame/.../Build_ConstructorMk1.Build_ConstructorMk1_C"
    // into a readable name like "Constructor Mk1". Checks itemDisplayNames first
    // (class name -> in-game display name from the game's localization files)
    // before falling back to regex-based cleanup of the path's last segment.
    function displayName(typePath) {
        if (!typePath) return t('status.unknown');
        // Extract class name from path for lookups
        var lastSeg = typePath.split('/').pop();
        // UE paths look like "Build_Foo.Build_Foo_C" — the part before the dot
        // is the asset name, the part after is the class name with _C suffix.
        // The asset name casing can differ from the class name (e.g. MK2 vs Mk2),
        // so we try both for the lookup.
        var shortName = lastSeg;
        var afterDot = '';
        if (lastSeg.includes('.')) {
            var parts = lastSeg.split('.');
            shortName = parts[0];
            afterDot = parts[1];
        }
        // Check itemDisplayNames for current language (class name -> in-game display name)
        var namesTable = itemDisplayNamesTable();
        if (namesTable) {
            if (afterDot && namesTable[afterDot]) return namesTable[afterDot];
            if (namesTable[shortName]) return namesTable[shortName];
            if (namesTable[shortName + '_C']) return namesTable[shortName + '_C'];
            if (afterDot && namesTable[afterDot.replace(/_C$/, '')]) return namesTable[afterDot.replace(/_C$/, '')];
            // Some vehicle class names lack the BP_ prefix in the path
            // (e.g. Testa_BP_WB_C -> key is BP_Testa_BP_WB_C)
            if (namesTable['BP_' + shortName]) return namesTable['BP_' + shortName];
            if (namesTable['BP_' + shortName + '_C']) return namesTable['BP_' + shortName + '_C'];
        }
        let name = shortName;
        name = name.replace(/^Build_/, '').replace(/^Desc_/, '').replace(/^BP_/, '').replace(/_C$/, '');
        name = name.replace(/WalkwayTrun/i, 'Walkway Turn');
        name = name.replace(/([a-z0-9])([A-Z])/g, '$1 $2');
        name = name.replace(/_/g, ' ');
        name = name.replace(/Attachment /i, '');
        // Use in-game names for storage types that don't auto-format well
        name = name.replace(/^Storage Player$/, 'Personal Storage');
        name = name.replace(/^Storage Integrated$/, 'Hub Storage');
        name = name.replace(/^Oil Refinery$/, 'Refinery');
        name = name.replace(/Mk\s*(\d+)/gi, ' Mk$1');
        name = name.replace(/\s+/g, ' ').trim();
        // Strip ' Mk1' from buildings that only have one tier
        name = name.replace(/(Constructor|Foundry|Assembler|Smelter|Manufacturer|Power Storage) Mk1$/i, '$1');
        return name;
    }

    // Like displayName but for recipe paths. Checks itemDisplayNames for
    // localized recipe names (e.g. "Recipe_Alternate_HighSpeedConnector_C" ->
    // "Alternate: Silicon High-Speed Connector" / German equivalent).
    // Falls back to regex-based cleanup if no localization entry found.
    function recipeName(path) {
        if (!path) return t('status.unknown');
        // Extract class name from path for lookup
        var lastSeg = path.split('/').pop();
        var className = lastSeg.includes('.') ? lastSeg.split('.').pop() : lastSeg;
        if (!className.endsWith('_C')) className = className + '_C';
        // Check itemDisplayNames for current language
        var namesTable = itemDisplayNamesTable();
        if (namesTable && namesTable[className]) return namesTable[className];
        // Fallback: regex-based cleanup
        let name = lastSeg.replace(/^Recipe_/, '').replace(/_C$/, '');
        if (name.startsWith('Alternate_')) name = 'Alt: ' + name.replace(/^Alternate_/, '');
        name = name.replace(/([a-z0-9])([A-Z])/g, '$1 $2');
        name = name.replace(/_/g, ' ');
        return name.replace(/\s+/g, ' ').trim();
    }

    // Extract the short class name (e.g. Build_ConstructorMk1_C) from a UE type path.
    function classFromPath(typePath) {
        if (!typePath) return '';
        var name = typePath.split('/').pop();
        if (name.includes('.')) {
            // UE format: PackageName.ClassName — the part after the dot is the
            // actual class name and may differ in casing from the package name
            // (e.g. Build_PipelinePumpMK2.Build_PipelinePumpMk2_C).
            var afterDot = name.split('.')[1];
            if (afterDot) return afterDot;
            name = name.split('.')[0];
        }
        if (!name.endsWith('_C')) name = name + '_C';
        return name;
    }

    // Returns the base64 data URI for an icon, or '' if not found.
    // Accepts either a full type path or a bare class name.
    function modFallbackIcon() {
        return (DATA && DATA.icons && DATA.icons['Mod_Icon_C']) || '';
    }

    function iconFor(typePath) {
        if (!typePath || !DATA || !DATA.icons) return '';
        var cls = classFromPath(typePath);
        if (DATA.icons[cls]) return DATA.icons[cls];
        // Try without _C suffix
        var noC = cls.replace(/_C$/, '');
        if (DATA.icons[noC]) return DATA.icons[noC];
        // Some vehicle class names lack the BP_ prefix in the path
        if (DATA.icons['BP_' + cls]) return DATA.icons['BP_' + cls];
        if (DATA.icons['BP_' + noC]) return DATA.icons['BP_' + noC];
        // Blueprint storage uses the blueprint designer icon
        if (cls === 'Build_StorageBlueprint_C' && DATA.icons['Build_BlueprintDesigner_C']) return DATA.icons['Build_BlueprintDesigner_C'];
        return modFallbackIcon();
    }

    // Returns an HTML string with icon + display name, suitable for table cells via {__html: ...}.
    // If no icon is found, returns just the display name as escaped text.
    function iconName(typePath) {
        var name = displayName(typePath);
        var icon = iconFor(typePath);
        if (icon) {
            return '<img class="row-icon" src="' + icon + '" alt=""> ' + esc(name);
        }
        return esc(name);
    }

    // Reverse lookup cache: display name -> icon data URI.
    var _nameToIcon = null;

    // Returns the base64 data URI for an icon by display name, or '' if not found.
    // Use this when you have a human-readable name (e.g. "Iron Ore", "AI Limiter")
    // instead of a class path.
    function iconForName(name) {
        if (!name || !DATA || !DATA.icons) return '';
        if (!_nameToIcon) {
            _nameToIcon = {};
            // Build from itemDisplayNames (class -> display name) if available
            if (DATA.itemDisplayNames) {
                for (var cls in DATA.itemDisplayNames) {
                    var dn = DATA.itemDisplayNames[cls];
                    if (DATA.icons[cls] && !_nameToIcon[dn]) {
                        _nameToIcon[dn] = DATA.icons[cls];
                    }
                }
            }
            // Also build from German display names (class -> display name -> icon)
            if (DATA.itemDisplayNamesDe) {
                for (cls in DATA.itemDisplayNamesDe) {
                    dn = DATA.itemDisplayNamesDe[cls];
                    if (DATA.icons[cls] && !_nameToIcon[dn]) {
                        _nameToIcon[dn] = DATA.icons[cls];
                    }
                }
            }
            // Also try displayName() for each icon key (covers Build_ classes
            // not in itemDisplayNames, e.g. "Miner Mk1", "Pipeline Pump")
            for (var cls2 in DATA.icons) {
                var dn2 = displayName(cls2);
                if (!_nameToIcon[dn2]) _nameToIcon[dn2] = DATA.icons[cls2];
            }
            // Manual aliases for display names that don't match any
            // canonical display name but appear in the report data
            var aliases = {
                'Storage Container': 'Build_StorageContainerMk2_C',
                'Cyberwagon': 'Desc_CyberWagon_C',
                'Factory Cart': 'Desc_GolfCart_C',
                'Truck Station': 'Build_TruckStation_C',
                'Fluid Truck Station': 'Build_FluidTruckStation_C',
                'Train Station': 'Build_TrainStation_C',
                'Drone Station': 'Build_DroneStation_C',
                'Boombox': 'Desc_BoomBox_C',
                'Electric Locomotive': 'Desc_Locomotive_C',
                'Freight Car': 'Desc_FreightWagon_C',
                'Hard Drive': 'Desc_HardDrive_C',
                'Blue': 'Desc_Crystal_C',
                'Yellow': 'Desc_Crystal_mk2_C',
                'Purple': 'Desc_Crystal_mk3_C',
                'Somersloop': 'Desc_WAT1_C',
                'Mercer Sphere': 'Desc_WAT2_C',
                'Pipeline Pump Mk1': 'Build_PipelinePump_C',
                'Pipeline Pump Mk2': 'Build_PipelinePumpMk2_C',
                'Storage Container Mk.1': 'Build_StorageContainerMk1_C',
                'Storage Container Mk.2': 'Build_StorageContainerMk2_C',
                'Integrated Storage': 'Build_StorageIntegrated_C',
                'Personal Storage': 'Build_StoragePlayer_C',
                'Dimensional Depot': 'Build_CentralStorage_C'
            };
            for (var alias in aliases) {
                var cls3 = aliases[alias];
                if (DATA.icons[cls3] && !_nameToIcon[alias]) {
                    _nameToIcon[alias] = DATA.icons[cls3];
                }
            }
        }
        return _nameToIcon[name] || modFallbackIcon();
    }

    // Like iconName() but takes a display name instead of a type path.
    // Returns HTML string with icon + escaped name, or just escaped name.
    function nameWithIcon(name) {
        var icon = iconForName(name);
        if (icon) {
            return '<img class="row-icon" src="' + icon + '" alt=""> ' + esc(name);
        }
        return esc(name);
    }

    function safe(v, fallback) {
        return (v !== null && v !== undefined) ? v : (fallback || 0);
    }

    function esc(s) {
        if (s === null || s === undefined) return '';
        return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;');
    }