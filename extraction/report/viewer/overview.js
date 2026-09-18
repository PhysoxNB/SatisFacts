    function renderOverview() {
        var c = document.getElementById('tab-overview');
        c.innerHTML = '';
        var d = DATA;
        var hdr = d.header || {};
        var pb = (d.analytics && d.analytics.power && d.analytics.power.power_balance) || {};
        var tr = (d.analytics && d.analytics.transport) || {};
        var st = (d.structures && d.structures.totals) || {};
        var gp = d.gameProgression || {};
        var TAB_COLOR = '#9ca3af';

        var s1 = section(t('section.save_information'));
        s1.classList.add('card-section');
        var grid1 = cardGrid(4);
        grid1.appendChild(card(t('card.save_name'), hdr.SaveName || hdr.SessionName || t('status.unknown'), '', TAB_COLOR));

        grid1.appendChild(card(t('card.session'), hdr.SessionName || t('status.unknown'), t('desc.game_version', gameVersion(hdr.SaveVersion)), TAB_COLOR));

        grid1.appendChild(card(t('card.play_time'), fmtTime(hdr.PlayDurationSeconds), t('desc.hours', Math.round(hdr.PlayDurationSeconds / 3600).toLocaleString('en-US')), TAB_COLOR));
        grid1.appendChild(card(t('card.save_date'), fmtDate(hdr.SaveDateTime), {__html: '<span class="card-time">' + fmtClock(hdr.SaveDateTime) + '</span>'}, TAB_COLOR));
        if (gp.startingLocation) grid1.appendChild(card(t('card.starting_location'), translateStartingLocation(gp.startingLocation), t('desc.where_pioneers_landed'), TAB_COLOR));
        if (hdr.IsModdedSave) grid1.appendChild(card(t('card.modded'), t('desc.yes'), '', TAB_COLOR));
        if (hdr.CreativeModeEnabled) grid1.appendChild(card(t('card.creative_mode'), t('desc.yes'), t('desc.creative_mode'), TAB_COLOR));
        s1.appendChild(grid1);
        c.appendChild(s1);

        var s2 = section(t('section.colony_summary'));
        s2.classList.add('card-section');
        var totalStructures = 0;
        for (var k in st) totalStructures += st[k] || 0;

        // Row 1: objects, structures, buildings
        var grid2a = cardGrid(3);
        grid2a.appendChild(card(t('card.total_objects'), fmt(d.totalObjects), '', TAB_COLOR));
        if (totalStructures > 0) grid2a.appendChild(card(t('card.total_structures'), fmt(totalStructures), t('desc.foundations_walls'), TAB_COLOR));
        if (d.totalBuildings > 0) grid2a.appendChild(card(t('card.total_buildings'), fmt(d.totalBuildings), t('desc.unique_types', (d.buildings ? Object.keys(d.buildings).length : 0)), TAB_COLOR));
        s2.appendChild(grid2a);

        if (d.analytics) {
            // Row 2: power — only show if there is actual power data
            var hasPower = (pb.capacity_mw && pb.capacity_mw > 0) || (pb.consumption_mw && pb.consumption_mw > 0);
            if (hasPower) {
                var grid2b = cardGrid(2);
                grid2b.appendChild(card(t('card.power_capacity'), fmtMw(pb.capacity_mw), pb.surplus_mw >= 0 ? t('desc.surplus', fmtMw(pb.surplus_mw)) : '', TAB_COLOR));
                grid2b.appendChild(card(t('card.power_consumption'), fmtMw(pb.consumption_mw), (pb.surplus_mw < 0 ? t('desc.deficit', fmtMw(Math.abs(pb.surplus_mw))) : ''), TAB_COLOR));
                s2.appendChild(grid2b);
            }

            // Row 3: total infrastructure — only show if non-zero
            if (tr.total_infrastructure_km && tr.total_infrastructure_km > 0) {
                var grid2c = cardGrid(1);
                grid2c.appendChild(card(t('card.total_infrastructure'), tr.total_infrastructure_km.toFixed(2) + ' km', t('desc.belts_pipes_power'), TAB_COLOR));
                s2.appendChild(grid2c);
            }

            // Row 4: belts, lifts, pipes, power lines
            var ib = tr.infrastructure_breakdown || {};
            var grid2d = cardGrid(4);
            if (ib.belts_km) grid2d.appendChild(card(t('card.conveyor_belts'), ib.belts_km.toFixed(2) + ' km', '', TAB_COLOR));
            if (ib.lifts_km) grid2d.appendChild(card(t('card.conveyor_lifts'), ib.lifts_km.toFixed(2) + ' km', '', TAB_COLOR));
            if (ib.pipes_km) grid2d.appendChild(card(t('card.pipelines'), ib.pipes_km.toFixed(2) + ' km', '', TAB_COLOR));
            if (ib.power_lines_km) grid2d.appendChild(card(t('card.power_lines'), ib.power_lines_km.toFixed(2) + ' km', '', TAB_COLOR));
            if (grid2d.children.length) s2.appendChild(grid2d);

            // Row 5: hypertubes, railway, vehicle paths
            var grid2e = cardGrid(3);
            if (ib.hypertubes_km) grid2e.appendChild(card(t('card.hypertubes'), ib.hypertubes_km.toFixed(2) + ' km', '', TAB_COLOR));
            if (ib.trains_km) grid2e.appendChild(card(t('card.railway'), ib.trains_km.toFixed(2) + ' km', '', TAB_COLOR));
            if (ib.vehicle_paths_km) grid2e.appendChild(card(t('card.vehicle_paths'), ib.vehicle_paths_km.toFixed(2) + ' km', '', TAB_COLOR));
            if (grid2e.children.length) s2.appendChild(grid2e);
        }
        c.appendChild(s2);

        // Custom map settings only exist in 1.2+ saves. Older saves won't have
        // these fields, so the whole section is skipped if none are present.
        if (gp.nodePuritySetting || gp.nodeRandomization || gp.partsCostMultiplier || gp.spacePartsCostMultiplier || gp.powerConsumptionMultiplier) {
            var sMap = section(t('section.custom_map_settings'));
            sMap.classList.add('card-section');

            // Row 1: world generation settings
            var gridMapA = cardGrid(3);
            if (gp.nodePuritySetting) {
                // These map the raw enum strings from the save to human-readable labels
                var purityMap = { 'NoChange': t('purity.default'), 'AllPure': t('purity.all_pure'), 'Increase': t('purity.mostly_pure'), 'AllNormal': t('purity.average'), 'Decrease': t('purity.mostly_impure'), 'AllImpure': t('purity.all_impure'), 'AllRandom': t('purity.random') };
                gridMapA.appendChild(card(t('card.resource_node_purity'), purityMap[gp.nodePuritySetting] || gp.nodePuritySetting, t('card.resource_node_purity'), TAB_COLOR));
            }
            if (gp.nodeRandomization) {
                var randMap = { 'None': t('rand.default'), 'Strict': t('rand.random'), 'BasicRich': t('rand.basic_rich'), 'AdvancedRich': t('rand.advanced_rich'), 'FossilFuelRich': t('rand.fossil_rich') };
                gridMapA.appendChild(card(t('card.resource_node_randomization'), randMap[gp.nodeRandomization] || gp.nodeRandomization, t('card.resource_node_randomization'), TAB_COLOR));
            }
            if (gp.nodeRandomizationSeed) {
                gridMapA.appendChild(card(t('card.world_seed'), gp.nodeRandomizationSeed, gp.nodeRandomizationSeed === 0 ? t('desc.default_vanilla_seed') : t('desc.share_seed'), TAB_COLOR));
            }
            sMap.appendChild(gridMapA);

            // Row 2: cost multipliers
            var gridMapB = cardGrid(3);
            if (gp.partsCostMultiplier) {
                gridMapB.appendChild(card(t('card.recipe_parts_cost'), gp.partsCostMultiplier + 'x', t('desc.recipe_parts'), TAB_COLOR));
            }
            if (gp.spacePartsCostMultiplier) {
                gridMapB.appendChild(card(t('card.space_elevator_cost'), gp.spacePartsCostMultiplier + 'x', t('desc.space_elevator_costs'), TAB_COLOR));
            }
            if (gp.powerConsumptionMultiplier) {
                gridMapB.appendChild(card(t('card.power_consumption_multiplier'), gp.powerConsumptionMultiplier + 'x', t('desc.power_multiplier'), TAB_COLOR));
            }
            sMap.appendChild(gridMapB);

            c.appendChild(sMap);
        }

    }