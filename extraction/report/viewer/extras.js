    function renderExtras() {
        var c = document.getElementById('tab-extras');
        c.innerHTML = '';
        var d = DATA;
        c.appendChild(tabDescription(t('desc.tab_extras')));
        var agp = (d.analytics && d.analytics.gameProgression) || {};

        // --- AWESOME Sink ---
        var sink = agp.awesome_sink || {};
        if (sink.coupons) {
            var sinkSection = section(t('section.awesome_sink'));
            sinkSection.classList.add('card-section');
            var sinkGrid = cardGrid(5);
            sinkGrid.appendChild(card(t('card.coupons_available'), fmt(sink.coupons), t('desc.unspent_coupons'), '#fbbf24'));
            sinkSection.appendChild(sinkGrid);
            c.appendChild(sinkSection);
        }

        // --- Game Progression ---
        var gp = d.gameProgression || {};
        if (gp.currentPhase || gp.targetPhase || (gp.purchasedSchematics && gp.purchasedSchematics.length > 0)) {
            var progSection = section(t('section.game_progression'));
            progSection.classList.add('card-section');
            var se = (agp.space_elevator) || {};
            let progGrid = cardGrid(5);
            if (se.current_phase_number !== undefined) {
                var phaseLabel = se.current_phase_number === 0 ? t('label.pre_space_elevator') : t('label.phase_n', se.current_phase_number);
                if (se.is_completed && se.current_phase_number >= 5) phaseLabel = t('label.phase_5_completed');
                progGrid.appendChild(card(t('card.current_phase'), phaseLabel, '', '#00d4ff'));
            }
            if (!se.is_completed && se.target_phase_number !== undefined) {
                var targetLabel = se.target_phase_number === 0 ? t('label.not_started') : t('label.phase_n', se.target_phase_number);
                progGrid.appendChild(card(t('card.target_phase'), targetLabel, '', '#f59e0b'));
            }
            progSection.appendChild(progGrid);

            // Hardcoded milestone totals per tier and schematic-to-tier mapping.
            // These are game constants - the save only stores which schematics
            // were purchased, not how many exist per tier.
            var schems = gp.purchasedSchematics || [];
            var tierTotals = {1:3, 2:5, 3:4, 4:5, 5:5, 6:5, 7:5, 8:5, 9:5};
            var schematicTier = {
                '1-1':1, '1-2':1, '1-3':1,
                '2-1':2, '2-2':2, '2-3':2, '2-5':2, '3-2':2,
                '3-1':3, '3-3':3, '3-4':3, '4-2':3,
                '4-1':4, '4-3':4, '4-4':4, '4-5':4, '5-3':4,
                '5-1':5, '5-4':5, '5-5':5, '6-1':5, '6-2':5,
                '5-2':6, '6-3':6, '6-5':6, '6-6':6, '6-7':6,
                '7-1':7, '7-2':7, '7-3':7, '7-5':7, '8-3':7,
                '7-4':8, '8-1':8, '8-2':8, '8-4':8, '8-5':8,
                '9-1':9, '9-2':9, '9-3':9, '9-4':9, '9-5':9
            };
            var tiers = {};
            schems.forEach(function(s) {
                var tier = schematicTier[s];
                if (tier) {
                    if (!tiers[tier]) tiers[tier] = {};
                    tiers[tier][s] = true;
                }
            });
            var tierNums = Object.keys(tiers).sort(function(a, b) { return parseInt(a) - parseInt(b); });
            if (tierNums.length > 0) {
                var tierGrid = cardGrid(5);
                tierNums.forEach(function(tier) {
                    var count = Object.keys(tiers[tier]).length;
                    var total = tierTotals[tier] || count;
                    if (count > total) count = total;
                    var color = count >= total ? '#22c55e' : '#00d4ff';
                    var desc = count + '/' + total + ' ' + t('label.milestones');
                    tierGrid.appendChild(progressCard(t('label.tier_n', tier), count, total, desc, color));
                });
                progSection.appendChild(tierGrid);
            }
            c.appendChild(progSection);
        }

        // --- MAM Research ---
        var mam = agp.mam;
        if (mam && mam.unlocked_count > 0) {
            var mamSection = section(t('section.mam_research'));
            mamSection.classList.add('card-section');
            var mamGrid = cardGrid(5);
            mamGrid.appendChild(card(t('card.research_activated'), mam.is_activated ? t('label.yes') : t('label.no'), '', '#22c55e'));
            mamGrid.appendChild(card(t('card.unlocked_trees'), fmt(mam.unlocked_count), t('desc.trees_discovered'), '#a855f7'));
            mamSection.appendChild(mamGrid);

            var progress = mam.progress || {};
            var treeNames = Object.keys(progress).sort();
            if (treeNames.length > 0) {
                let progGrid = cardGrid(5);
                treeNames.forEach(function(tree) {
                    var p = progress[tree] || {};
                    var completed = p.completed || 0;
                    var total = p.total || 0;
                    var pct = p.percentage || 0;
                    // total < 0 indicates a modded research tree where the
                    // total node count is unknown. We show '?' instead of a number.
                    if (total < 0) {
                        let color = completed > 0 ? '#00d4ff' : '#888';
                        let desc = completed > 0 ? completed + ' ' + t('label.nodes_unlocked_unknown') : t('label.modded_tree_unknown');
                        progGrid.appendChild(progressCard(tree, completed, '?', desc, color));
                    } else {
                        let color = pct >= 100 ? '#4caf50' : '#00d4ff';
                        progGrid.appendChild(progressCard(tree, completed, total, pct + '% ' + t('label.complete'), color));
                    }
                });
                mamSection.appendChild(progGrid);
            }
            c.appendChild(mamSection);
        }

        // --- Player Stats ---
        var players = (d.analytics && d.analytics.players) || [];
        if (players.length > 0) {
            var playerSection = section(t('section.players') + ' (' + players.length + ')');
            playerSection.classList.add('card-section');
            players.forEach(function(p) {
                var pGrid = cardGrid(5);
                pGrid.appendChild(card(t('card.player'), p.name, '', '#00d4ff'));
                if (p.inventorySlots) pGrid.appendChild(card(t('card.inventory_slots'), p.inventorySlots, t('desc.unlocked_slots'), '#3b82f6'));
                if (p.visitedAreas) pGrid.appendChild(card(t('card.areas_visited'), p.visitedAreas, t('desc.map_areas_explored'), '#22c55e'));
                if (p.playedMessages) pGrid.appendChild(card(t('card.messages_played'), p.playedMessages, t('desc.tutorial_story_messages'), '#a855f7'));
                playerSection.appendChild(pGrid);
            });
            c.appendChild(playerSection);
        }

        // --- Pets ---
        var pets = d.pets || {};
        if (pets.tamedDoggos > 0) {
            var petSection = section(t('section.pets'));
            petSection.classList.add('card-section');
            var petGrid = cardGrid(5);
            petGrid.appendChild(card(t('card.lizard_doggos_tamed'), fmt(pets.tamedDoggos || 0), t('desc.tamed_by_players'), '#f97316'));
            petSection.appendChild(petGrid);
            c.appendChild(petSection);
        }

        // --- Blueprints & Consumables side by side ---
        var blueprints = d.blueprints || {};
        var bpKeys = Object.keys(blueprints).filter(function(type) { return blueprints[type] > 0; });
        var hasBp = bpKeys.length > 0;
        var cc = d.consumablesConsumed || {};
        var hasCc = cc.total > 0;
        if (hasBp || hasCc) {
            var fr = wrapRow();
            if (hasBp) {
                var bpTotal = 0;
                bpKeys.forEach(function(type) { bpTotal += blueprints[type]; });
                var bpSection = section(t('section.blueprints_subsystems') + ' (' + fmt(bpTotal) + ')');
                bpSection.classList.add('auto-width');
                var bpRows = bpKeys.map(function(type) {
                    var lower = type.toLowerCase();
                    var label = t('status.unknown');
                    if (lower.indexOf('blueprintproxy') >= 0) {
                        label = t('label.blueprint_placed');
                    } else if (lower.indexOf('blueprintshortcut') >= 0) {
                        label = t('label.blueprint_shortcuts');
                    }
                    return [esc(label), fmt(blueprints[type])];
                }).sort(byNumDesc(1));
                bpSection.appendChild(sortableTable(
                    [{label: t('col.type'), width: '300px'},
                    {label: t('col.count'), type: 'number', width: '100px'}],
                    bpRows,
                    {scrollable: bpRows.length > 20, autoWidth: true}
                ));
                fr.appendChild(bpSection);
            }
            if (hasCc) {
                var ccSection = section(t('section.consumables_consumed') + ' (' + fmt(cc.total) + ')');
                ccSection.classList.add('auto-width');
                var ccByType = cc.byType || {};
                var ccKeys = Object.keys(ccByType).sort(function(a, b) {
                    return ccByType[b] - ccByType[a];
                });
                var ccRows = ccKeys.map(function(cls) {
                    var name = displayName(cls);
                    var icon = iconFor(cls);
                    return [icon ? html('<img class="row-icon" src="' + icon + '" alt=""> ' + esc(name)) : esc(name), fmt(ccByType[cls])];
                });
                ccSection.appendChild(sortableTable(
                    [{label: t('col.item'), width: '300px'},
                    {label: t('col.consumed'), type: 'number', width: '100px'}],
                    ccRows,
                    {scrollable: ccRows.length > 15, autoWidth: true}
                ));
                fr.appendChild(ccSection);
            }
            c.appendChild(fr);
        }

        // --- Creatures Killed ---
        var ck = d.creatureKills || {};
        if (ck.total > 0) {
            var ckSection = section(t('section.creatures_killed') + ' (' + fmt(ck.total) + ')');
            ckSection.classList.add('auto-width');
            var byType = ck.byType || {};
            var ckKeys = Object.keys(byType).sort(function(a, b) {
                return byType[b] - byType[a];
            });
            var ckRows = ckKeys.map(function(cls) {
                var name = displayName(cls);
                var icon = iconFor(cls);
                return [icon ? html('<img class="row-icon" src="' + icon + '" alt=""> ' + esc(name)) : esc(name), fmt(byType[cls])];
            });
            ckSection.appendChild(sortableTable(
                [{label: t('col.creature'), width: '300px'},
                {label: t('col.kills'), type: 'number', width: '100px'}],
                ckRows,
                {scrollable: ckRows.length > 15, autoWidth: true}
            ));
            c.appendChild(ckSection);
        }

        // Show a message if no extras data is available
        if (c.children.length === 0) {
            c.appendChild(emptyState(t('empty.no_extras_data')));
        }
    }