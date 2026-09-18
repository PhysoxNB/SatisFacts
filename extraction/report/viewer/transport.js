    function renderTransport() {
        var c = document.getElementById('tab-transport');
        c.innerHTML = '';
        var d = DATA;
        c.appendChild(tabDescription(t('desc.tab_transport')));
        var TAB_COLOR = '#9ca3af';
        var tr = (d.analytics && d.analytics.transport) || {};

        var prod = d.production || {};
        var log = d.logistics || {};

        // Hypertubes
        if (prod.hypertubes && prod.hypertubes.length > 0) {
            var s6 = section(t('section.hypertubes'));
            s6.classList.add('card-section');
            var total = prod.hypertubes.reduce(function(a, b) { return a + b; }, 0);
            var grid6 = cardGrid(5);
            grid6.appendChild(card(t('card.total_segments'), fmt(prod.hypertubes.length), fmtKm(total), TAB_COLOR));
            if (prod.hypertubeJunctions) grid6.appendChild(card(t('card.junctions_entrances'), fmt(prod.hypertubeJunctions), '', TAB_COLOR));
            s6.appendChild(grid6);
            c.appendChild(s6);
        }

        // Trains summary
        var trains = prod.trains || {};
        var railLength = (prod.rails && prod.rails.TotalLength) ? prod.rails.TotalLength : 0;
        var railCount = (prod.rails && prod.rails.Count) ? prod.rails.Count : 0;
        var trainTotal = (trains.Locomotives || 0) + (trains.Wagons || 0) + (trains.Stations || 0) + (trains.FreightPlatforms || 0) + (trains.FluidFreightPlatforms || 0);
        if (trainTotal > 0 || railLength > 0) {
            var s7 = section(t('section.trains'));
            s7.classList.add('card-section');
            var grid7 = cardGrid(5);
            if (railLength > 0) grid7.appendChild(card(t('card.rail_length'), fmtKm(railLength), railCount + ' ' + t('label.segments'), TAB_COLOR));
            if (trains.Locomotives) grid7.appendChild(card(t('card.locomotives'), fmt(trains.Locomotives), '', TAB_COLOR));
            if (trains.Wagons) grid7.appendChild(card(t('card.freight_wagons'), fmt(trains.Wagons), '', TAB_COLOR));
            if (trains.Stations) grid7.appendChild(card(t('card.train_stations'), fmt(trains.Stations), '', TAB_COLOR));
            if (trains.FreightPlatforms) grid7.appendChild(card(t('card.freight_platforms'), fmt(trains.FreightPlatforms), '', TAB_COLOR));
            if (trains.FluidFreightPlatforms) grid7.appendChild(card(t('card.fluid_freight_platforms'), fmt(trains.FluidFreightPlatforms), '', TAB_COLOR));
            var blockSignals = (prod.rails && prod.rails.BlockSignals) ? prod.rails.BlockSignals : 0;
            var pathSignals = (prod.rails && prod.rails.PathSignals) ? prod.rails.PathSignals : 0;
            if (blockSignals) grid7.appendChild(card(t('card.block_signals'), fmt(blockSignals), '', TAB_COLOR));
            if (pathSignals) grid7.appendChild(card(t('card.path_signals'), fmt(pathSignals), '', TAB_COLOR));
            s7.appendChild(grid7);
            c.appendChild(s7);
        }

        // Train stations + details side by side (stations left, details right)
        var trainWrap = wrapRow();
        var hasTrainContent = false;

        // Train stations table (paginated)
        if (log.trainStations && log.trainStations.length > 0) {
            var tsRows = log.trainStations.map(function(s) {
                var warningStr = (s.warnings && s.warnings.length > 0) ? s.warnings.join(', ') : '';
                var stationIcon = iconForName('Train Station');
                var nameHTML = (stationIcon ? '<img class="row-icon" src="' + stationIcon + '" alt=""> ' : '') + esc(s.name);
                var nameCell;
                if (warningStr) {
                    nameCell = {__el: h('span', {},
                        h('span', { html: nameHTML + ' ' }),
                        badge(warningStr, 'red')
                    )};
                } else {
                    nameCell = html(nameHTML);
                }
                var locCell = mapLink(s.location);
                var fillCell = fillBadge(s.platformFillPct);
                return [nameCell, s.dockedTrain || DASH, s.platforms, {__el: fillCell}, locCell];
            });
            trainWrap.appendChild(paginatedTable(tsRows,
                [{label: t('col.station'), width: '150px'},
                {label: t('col.docked_train'), width: '120px'},
                {label: t('col.platforms'), type: 'number', width: '80px'},
                {label: t('col.platform_fill'), width: '110px'},
                {label: '', width: '40px'}],
                { sectionTitle: t('section.train_stations') + ' (' + log.trainStations.length + ')', scrollable: true, autoWidth: true, filterPlaceholder: t('filter.by_name_train') }
            ));
            hasTrainContent = true;
        }

        // Train details table (paginated)
        if (log.trains && log.trains.length > 0) {
            var trainRows = log.trains.map(function(tr) {
                var routeStr = (tr.route && tr.route.length > 0) ? tr.route.join(' \u2192 ') : '';
                var warnings = tr.warnings || [];
                if (warnings.length > 0) {
                    var warnBadge = badge(warnings.join(', '), 'red');
                    if (routeStr) {
                        routeStr = {__el: h('span', {},
                            h('span', { text: routeStr + ' ' }),
                            warnBadge
                        )};
                    } else {
                        routeStr = {__el: warnBadge};
                    }
                }
                if (!routeStr) routeStr = DASH;
                var stopStr = DASH;
                if (tr.route && tr.route.length > 0 && tr.currentStop >= 0 && tr.currentStop < tr.route.length) {
                    stopStr = '\u2192 ' + tr.route[tr.currentStop] + ' (' + (tr.currentStop + 1) + '/' + tr.route.length + ')';
                }
                var distStr = DASH;
                if (tr.totalDistance && tr.totalDistance > 0) {
                    distStr = tr.totalDistance.toFixed(1) + ' km';
                }
                var wagonFillCell = fillBadge(tr.wagonFillPct);
                var trainIcon = iconForName('Electric Locomotive');
                var trainName = tr.name || t('label.train');
                var trainHTML = (trainIcon ? '<img class="row-icon" src="' + trainIcon + '" alt=""> ' : '') + esc(trainName);
                return [html(trainHTML), tr.locomotives, tr.wagons, {__el: wagonFillCell}, stopStr, routeStr, distStr];
            });
            var trainDetSec = paginatedTable(trainRows,
                [{label: t('col.train'), width: '100px'},
                {label: t('col.locos'), type: 'number', width: '40px'},
                {label: t('col.wagons'), type: 'number', width: '50px'},
                {label: t('col.wagon_fill'), width: '100px'},
                {label: t('col.next_stop'), width: '180px'},
                {label: t('col.route'), width: '300px'},
                {label: t('col.distance'), type: 'number', width: '90px'}],
                { sectionTitle: t('section.train_details') + ' (' + log.trains.length + ')', scrollable: true, autoWidth: true, filterPlaceholder: t('filter.by_name_route') }
            );
            trainDetSec.classList.add('train-details');
            // Route column is hidden by default (can be very long for trains with
            // many stops). The toggle button shows/hides it via a CSS class.
            var trainFilterBar = trainDetSec.querySelector('.filter-bar');
            if (trainFilterBar) {
                var routeBtn = h('button', { class: 'train-route-btn', title: t('btn.show_routes') }, t('btn.show_routes'));
                routeBtn.addEventListener('click', function() {
                    var expanded = trainDetSec.classList.toggle('routes-expanded');
                    routeBtn.textContent = expanded ? t('btn.hide_routes') : t('btn.show_routes');
                });
                trainFilterBar.appendChild(routeBtn);
            }
            trainWrap.appendChild(trainDetSec);
            hasTrainContent = true;
        }

        if (hasTrainContent) c.appendChild(trainWrap);

        // Vehicles summary
        var vehicles = (d.analytics && d.analytics.vehicles) || {};
        var ib = tr.infrastructure_breakdown || {};
        var vehiclePathKm = ib.vehicle_paths_km || 0;
        var vehicleTotal = (vehicles.Trucks || 0) + (vehicles.FluidTrucks || 0) + (vehicles.Explorers || 0) + (vehicles.Tractors || 0) + (vehicles.Golfcarts || 0) + (vehicles.Cyberwagons || 0) + (vehicles.TruckStations || 0) + (vehicles.FluidTruckStations || 0);
        if (vehicleTotal > 0 || vehiclePathKm > 0) {
            var s9 = section(t('section.vehicles'));
            s9.classList.add('card-section');
            var grid9 = cardGrid(5);
            if (vehiclePathKm > 0) grid9.appendChild(card(t('card.vehicle_tracks'), vehiclePathKm.toFixed(2) + ' km', t('desc.total_path_length'), TAB_COLOR));
            if (vehicles.Trucks) grid9.appendChild(card(t('card.trucks'), fmt(vehicles.Trucks), '', TAB_COLOR));
            if (vehicles.FluidTrucks) grid9.appendChild(card(t('card.fluid_trucks'), fmt(vehicles.FluidTrucks), '', TAB_COLOR));
            if (vehicles.Explorers) grid9.appendChild(card(t('card.explorers'), fmt(vehicles.Explorers), '', TAB_COLOR));
            if (vehicles.Tractors) grid9.appendChild(card(t('card.tractors'), fmt(vehicles.Tractors), '', TAB_COLOR));
            if (vehicles.Golfcarts) grid9.appendChild(card(t('card.factory_carts'), fmt(vehicles.Golfcarts), '', TAB_COLOR));
            if (vehicles.Cyberwagons) grid9.appendChild(card(t('card.cyberwagons'), fmt(vehicles.Cyberwagons), '', TAB_COLOR));
            if (vehicles.TruckStations) grid9.appendChild(card(t('card.truck_stations'), fmt(vehicles.TruckStations), '', TAB_COLOR));
            if (vehicles.FluidTruckStations) grid9.appendChild(card(t('card.fluid_truck_stations'), fmt(vehicles.FluidTruckStations), '', TAB_COLOR));
            s9.appendChild(grid9);
            c.appendChild(s9);
        }

        // Vehicle stations + details side by side (stations left, details right)
        var vehWrap = wrapRow();
        var hasVehContent = false;

        // Vehicle stations table (paginated)
        if (log.vehicleStations && log.vehicleStations.length > 0) {
            var vsRows = log.vehicleStations.map(function(vs) {
                var rawStatus = vs.status || '';
                var statusColor = 'green';
                if (rawStatus.indexOf('NoPower') >= 0 || rawStatus.indexOf('No Power') >= 0) {
                    statusColor = 'red';
                } else if (rawStatus.indexOf('Idle') >= 0 || rawStatus === '') {
                    statusColor = 'blue';
                }
                var statusText = translateVehicleStationStatus(rawStatus);
                var statusBadge = badge(statusText, statusColor);
                var locCell = mapLink(vs.location);
                return [vs.name, html(nameWithIcon(vs.type)), {__el: statusBadge}, locCell];
            });
            vehWrap.appendChild(paginatedTable(vsRows,
                // widened from 150/120 - "Fluid Truck Station 1" and "Fluid
                // Truck Station" were both getting cut off (td is
                // overflow:hidden + ellipsis, so it truncates without warning).
                [{label: t('col.station'), width: '175px'},
                {label: t('col.type'), width: '160px'},
                {label: t('col.status'), width: '100px'},
                {label: '', width: '40px'}],
                { sectionTitle: t('section.vehicle_stations') + ' (' + log.vehicleStations.length + ')', scrollable: true, autoWidth: true, filterPlaceholder: t('filter.by_name_type_status') }
            ));
            hasVehContent = true;
        }

        // Vehicle details table (paginated)
        if (log.vehicles && log.vehicles.length > 0) {
            var vehRows = log.vehicles.map(function(v) {
                var fuelCell = '';
                if (v.fuel) {
                    fuelCell = html(iconName(v.fuel));
                }
                var warnings = v.warnings || [];
                if (warnings.length > 0) {
                    var warnBadge = badge(warnings.join(', '), 'red');
                    if (fuelCell) {
                        fuelCell = {__el: h('span', {},
                            h('span', { html: iconName(v.fuel) + ' ' }),
                            warnBadge
                        )};
                    } else {
                        fuelCell = {__el: warnBadge};
                    }
                }
                if (!fuelCell) fuelCell = DASH;
                var statusBadge = v.onRoute ? badge(t('label.on_route'), 'green') : badge(t('label.parked'), 'blue');
                var locCell = mapLink(v.location);
                var nameStr = v.name || displayName(v.type);
                return [esc(nameStr), html(iconName(v.type)), fuelCell, {__el: statusBadge}, locCell];
            });
            vehWrap.appendChild(paginatedTable(vehRows,
                [{label: t('col.name'), width: '120px'},
                {label: t('col.type'), width: '120px'},
                {label: t('col.fuel'), width: '100px'},
                {label: t('col.status'), width: '90px'},
                {label: '', width: '40px'}],
                { sectionTitle: t('section.vehicle_details') + ' (' + log.vehicles.length + ')', scrollable: true, autoWidth: true, filterPlaceholder: t('filter.by_name_type_fuel') }
            ));
            hasVehContent = true;
        }

        if (hasVehContent) c.appendChild(vehWrap);

        // Drones summary
        var drones = prod.drones || {};
        var droneTotal = (drones.Stations || 0) + (drones.FreightPlatforms || 0) + (drones.Drones || 0);
        if (droneTotal > 0) {
            var s8 = section(t('section.drones'));
            s8.classList.add('card-section');
            var grid8 = cardGrid(5);
            if (drones.Drones) grid8.appendChild(card(t('card.drones'), fmt(drones.Drones), '', TAB_COLOR));
            if (drones.Stations) grid8.appendChild(card(t('card.drone_stations'), fmt(drones.Stations), '', TAB_COLOR));
            if (drones.FreightPlatforms) grid8.appendChild(card(t('card.freight_platforms'), fmt(drones.FreightPlatforms), '', TAB_COLOR));
            s8.appendChild(grid8);
            c.appendChild(s8);
        }

        // Drone stations + details side by side (stations left, details right)
        var droneWrap = wrapRow();
        var hasDroneContent = false;

        // Drone stations table (paginated)
        if (log.droneStations && log.droneStations.length > 0) {
            var dsRows = log.droneStations.map(function(ds) {
                var stageText = translateTransferStage(ds.transferStage || '');
                var stageBadge = stageText ? badge(stageText, 'green') : DASH;
                var locCell = mapLink(ds.location);
                var droneIcon = iconForName('Drone Station');
                var dsNameHTML = (droneIcon ? '<img class="row-icon" src="' + droneIcon + '" alt=""> ' : '') + esc(ds.name);
                return [html(dsNameHTML), ds.pairedStation || DASH, {__el: stageBadge === DASH ? h('span', {text: DASH}) : stageBadge}, locCell];
            });
            droneWrap.appendChild(paginatedTable(dsRows,
                [{label: t('col.station'), width: '150px'},
                {label: t('col.paired_station'), width: '150px'},
                {label: t('col.transfer'), width: '100px'},
                {label: '', width: '40px'}],
                { sectionTitle: t('section.drone_stations') + ' (' + log.droneStations.length + ')', scrollable: true, autoWidth: true, filterPlaceholder: t('filter.by_name_paired') }
            ));
            hasDroneContent = true;
        }

        // Drone details table (paginated)
        if (log.drones && log.drones.length > 0) {
            var droneRows = log.drones.map(function(dr) {
                var fuelHTML = iconName(dr.fuel || DASH);
                var fuelCell = html(fuelHTML);
                var warnings = dr.warnings || [];
                if (warnings.length > 0) {
                    fuelCell = {__el: h('span', {},
                        h('span', { html: fuelHTML + ' ' }),
                        badge(warnings.join(', '), 'red')
                    )};
                }
                // Drone states: 'In Transit' is set by the backend when mHasBegunTrip=true.
                // Docked states come from mCurrentDockingState enum (DS_DOCKED, DS_UNLOADING, etc.)
                var rawState = dr.dockingState || '';
                var stateColor = (rawState === 'In Transit') ? 'blue' : 'green';
                var stateText = translateDroneState(rawState);
                var stateBadge = stateText ? badge(stateText, stateColor) : DASH;
                var destCell = DASH;
                // Destination cell shows paired station name, item count being
                // transported, and elapsed flight time for in-transit drones
                if (dr.destination) {
                    var destText = '\u2192 ' + dr.destination;
                    if (dr.tripItemCount > 0) {
                        destText += ' ' + t('label.items_count', dr.tripItemCount);
                    }
                    if (dr.tripDuration > 0) {
                        var mins = Math.floor(dr.tripDuration / 60);
                        var secs = Math.floor(dr.tripDuration % 60);
                        var timeStr = mins > 0 ? mins + 'm ' + secs + 's' : secs + 's';
                        destText += ' ' + t('label.flying_for', timeStr);
                    }
                    destCell = destText;
                }
                var hsIcon = iconForName('Drone Station');
                var hsHTML = (hsIcon ? '<img class="row-icon" src="' + hsIcon + '" alt=""> ' : '') + esc(dr.homeStation || DASH);
                return [fuelCell, {__el: stateBadge === DASH ? h('span', {text: DASH}) : stateBadge}, html(hsHTML), destCell];
            });
            droneWrap.appendChild(paginatedTable(droneRows,
                // 380px: an in-flight drone needs ~325px for the destination,
                // item count and flight time, and was cut off at 300px. if your
                // station names ever outgrow this, use cls:'wrap-cell' rather
                // than widening it again.
                [{label: t('col.fuel'), width: '100px'},
                {label: t('col.state'), width: '100px'},
                {label: t('col.home_station'), width: '150px'},
                {label: t('col.destination_trip'), width: '380px'}],
                { sectionTitle: t('section.drone_details') + ' (' + log.drones.length + ')', scrollable: true, autoWidth: true, filterPlaceholder: t('filter.by_fuel_state') }
            ));
            hasDroneContent = true;
        }

        if (hasDroneContent) c.appendChild(droneWrap);
    }