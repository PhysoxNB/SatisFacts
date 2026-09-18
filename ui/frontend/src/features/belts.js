import { state, $, showToast, showInlineProgress, hideInlineProgress, validateSavePath, resolveChainIO, buildOutputPath, showPreview, friendlyError, escapeHtml, callbacks } from '../state.js';
import { t } from '../i18n';

let beltInventory = null;

export function resetBeltUpgradeUI() {
    hideInlineProgress('belt');
    beltInventory = null;
    const el = $('belt-inventory');
    if (el) el.innerHTML = '<p class="muted center">' + t('belts.click_scan') + '</p>';
    const btn = $('btn-upgrade-belts');
    if (btn) btn.disabled = true;
}

function groupBeltInventory() {
    const groups = {
        'ConveyorBeltMk': { label: t('belts.conveyor_belts'), max: beltInventory.maxBeltMk, counts: {} },
        'ConveyorLiftMk': { label: t('belts.conveyor_lifts'), max: beltInventory.maxLiftMk, counts: {} },
        'ConveyorPipelineMk': { label: t('belts.pipelines'), max: beltInventory.maxPipeMk, counts: {} },
    };
    const modded = [];
    beltInventory.items.forEach(item => {
        if (groups[item.type]) {
            groups[item.type].counts[item.mk] = item.count;
        } else if (item.type === 'ModdedBelt' || item.type === 'ModdedLift' || item.type === 'ModdedPipe') {
            modded.push(item);
        }
    });
    groups._modded = modded;
    return groups;
}

// Reads whatever the user has currently configured in the options section so a
// re-render (language switch) can put it back. Returns null before the section
// has ever been built.
function readUpgradeOptions() {
    const targetSelect = $('belt-target-mk');
    if (!targetSelect) return null;
    return {
        belts: $('belt-scope-belts')?.checked,
        lifts: $('belt-scope-lifts')?.checked,
        pipes: $('belt-scope-pipes')?.checked,
        targetMk: targetSelect.value,
        override: $('belt-override')?.checked,
    };
}

function restoreUpgradeOptions(opts) {
    if (!opts) return;
    const set = (id, val) => { const el = $(id); if (el && val !== undefined) el.checked = val; };
    set('belt-scope-belts', opts.belts);
    set('belt-scope-lifts', opts.lifts);
    set('belt-scope-pipes', opts.pipes);
    set('belt-override', opts.override);
    const targetSelect = $('belt-target-mk');
    if (targetSelect && opts.targetMk) targetSelect.value = opts.targetMk;
}

// Returns true when the upgrade options section was rendered, i.e. when there is
// actually something to act on. The caller uses this to gate the Upgrade button.
function renderBeltInventory() {
    const el = $('belt-inventory');
    const prevOptions = readUpgradeOptions();
    el.innerHTML = '';

    if (!beltInventory || !beltInventory.items || beltInventory.items.length === 0) {
        el.innerHTML = '<p class="muted center">' + t('belts.no_items') + '</p>';
        return false;
    }

    const groups = groupBeltInventory();

    const tierInfo = document.createElement('div');
    tierInfo.className = 'belt-tier-info';
    tierInfo.innerHTML = `<span class="muted">${t('belts.hub_tier', { tier: beltInventory.hubTier })}</span>`;
    el.appendChild(tierInfo);

    // Modded items warning
    if (groups._modded && groups._modded.length > 0) {
        const moddedDiv = document.createElement('div');
        moddedDiv.className = 'belt-modded-warning';
        let moddedHTML = `<div class="belt-modded-header">⚠️ ${t('belts.modded_detected')}</div>`;
        moddedHTML += `<p class="belt-modded-desc">${t('belts.modded_desc')}</p>`;
        moddedHTML += '<ul class="belt-modded-list">';
        groups._modded.forEach(item => {
            const typeLabel = item.type === 'ModdedBelt' ? t('belts.conveyor_belts') :
                item.type === 'ModdedLift' ? t('belts.conveyor_lifts') : t('belts.pipelines');
            const name = item.className ? item.className.split('/').pop() : item.type;
            moddedHTML += `<li><span class="belt-modded-type">${typeLabel}</span> — ${escapeHtml(name)} ×${item.count}</li>`;
        });
        moddedHTML += '</ul>';
        moddedDiv.innerHTML = moddedHTML;
        el.appendChild(moddedDiv);
    }

    for (const [typeKey, g] of Object.entries(groups)) {
        if (typeKey === '_modded') continue;
        const hasItems = Object.keys(g.counts).length > 0;
        if (!hasItems) continue;

        const card = document.createElement('div');
        card.className = 'belt-inventory-card';

        let html = `<h3>${g.label}</h3>`;
        html += `<div class="belt-mk-row">`;
        const maxMkCard = typeKey === 'ConveyorPipelineMk' ? 2 : 6;
        for (let mk = 1; mk <= maxMkCard; mk++) {
            const count = g.counts[mk] || 0;
            const locked = mk > g.max;
            const cls = locked ? 'belt-mk-cell locked' : 'belt-mk-cell';
            html += `<div class="${cls}"><span class="belt-mk-label">Mk${mk}</span><span class="belt-mk-count">${count}</span>${locked ? '<span class="belt-locked">🔒</span>' : ''}</div>`;
        }
        html += `</div>`;
        html += `<div class="belt-max-info muted">${t('belts.max_unlocked', { mk: g.max })}</div>`;
        card.innerHTML = html;
        el.appendChild(card);
    }

    const targetDiv = document.createElement('div');
    targetDiv.className = 'belt-target-section';
    let html = '<h3>' + t('belts.upgrade_options') + '</h3>';

    html += '<div class="belt-section-group">';
    html += '<div class="belt-section-title">' + t('belts.belts_lifts') + '</div>';
    html += '<div class="belt-scope-row">';
    html += '<label class="checkbox-toggle"><input type="checkbox" id="belt-scope-belts" checked/><span>' + t('belts.belts') + '</span></label>';
    html += '<label class="checkbox-toggle"><input type="checkbox" id="belt-scope-lifts" checked/><span>' + t('belts.lifts') + '</span></label>';
    html += '</div>';
    html += '<div class="belt-target-row">';
    html += '<label>' + t('belts.target_tier') + '</label>';
    html += '<select id="belt-target-mk" class="text-input">';
    for (let mk = 2; mk <= 6; mk++) {
        html += `<option value="${mk}">Mk${mk}</option>`;
    }
    html += '</select>';
    html += '</div>';
    html += '</div>';

    html += '<div class="belt-section-group">';
    html += '<div class="belt-section-title">' + t('belts.pipelines') + '</div>';
    html += '<div class="belt-scope-row">';
    html += '<label class="checkbox-toggle"><input type="checkbox" id="belt-scope-pipes"/><span>' + t('belts.upgrade_pipes') + '</span></label>';
    html += '</div>';
    html += '<div class="belt-pipe-info muted">' + t('belts.pipes_info') + '</div>';
    html += '</div>';

    html += '<div class="belt-override-row">';
    html += '<label class="checkbox-toggle"><input type="checkbox" id="belt-override"/><span>' + t('belts.override') + '</span></label>';
    html += '</div>';

    targetDiv.innerHTML = html;
    el.appendChild(targetDiv);

    // A re-render (e.g. language switch) rebuilds this section from scratch, so
    // without this the user's chosen target tier and scope silently reset.
    restoreUpgradeOptions(prevOptions);
    return true;
}

export function initBelts() {
    $('btn-load-belts').addEventListener('click', async () => {
        const validationError = validateSavePath(state.selectedSavePath);
        if (validationError) { showToast(validationError, 'error'); return; }
        try {
            showInlineProgress('belt', t('belts.scanning'));
            beltInventory = await window.go.main.App.GetBeltInventory(state.selectedSavePath);
            // Only enable Upgrade if the options section actually rendered.
            // A save with no belts/lifts/pipes leaves those controls absent, and
            // the handler below would then throw on a null .checked read.
            const hasOptions = renderBeltInventory();
            $('btn-upgrade-belts').disabled = !hasOptions;
            if (hasOptions) {
                showToast(t('belts.inventory_loaded'), 'success');
            } else {
                showToast(t('belts.no_items'), 'info');
            }
        } catch (err) {
            showToast(t('belts.error_scanning', { err: friendlyError(err) }), 'error');
        } finally {
            hideInlineProgress('belt');
        }
    });

    $('btn-upgrade-belts').addEventListener('click', async () => {
        const validationError = validateSavePath(state.selectedSavePath);
        if (validationError) { showToast(validationError, 'error'); return; }
        if (!beltInventory) return;

        const opts = readUpgradeOptions();
        if (!opts) { showToast(t('belts.no_items'), 'error'); return; }

        const upgradeBelts = !!opts.belts;
        const upgradeLifts = !!opts.lifts;
        const upgradePipes = !!opts.pipes;
        const targetMk = parseInt(opts.targetMk, 10);
        const override = !!opts.override;

        if (!upgradeBelts && !upgradeLifts && !upgradePipes) {
            showToast(t('belts.select_type'), 'error');
            return;
        }

        if (!override) {
            if (upgradeBelts && targetMk > beltInventory.maxBeltMk) {
                showToast(t('belts.target_exceeds', { type: t('belts.conveyor_belts').toLowerCase(), target: targetMk, max: beltInventory.maxBeltMk }) + ' ' + t('belts.enable_override'), 'error');
                return;
            }
            if (upgradeLifts && targetMk > beltInventory.maxLiftMk) {
                showToast(t('belts.target_exceeds', { type: t('belts.conveyor_lifts').toLowerCase(), target: targetMk, max: beltInventory.maxLiftMk }) + ' ' + t('belts.enable_override'), 'error');
                return;
            }
        }

        let previewHTML = '<div class="preview-belt-list">';
        const groups = groupBeltInventory();
        groups['ConveyorBeltMk'].enabled = upgradeBelts;
        groups['ConveyorLiftMk'].enabled = upgradeLifts;
        groups['ConveyorPipelineMk'].enabled = upgradePipes;

        let totalUpgraded = 0;
        for (const [typeKey, g] of Object.entries(groups)) {
            if (typeKey === '_modded') continue;
            if (!g.enabled) continue;
            let typeTotal = 0;
            const targetForType = typeKey === 'ConveyorPipelineMk' ? 2 : targetMk;
            let typeHTML = `<div class="preview-belt-type"><strong>${g.label}:</strong> `;
            for (let mk = 1; mk < targetForType; mk++) {
                const count = g.counts[mk] || 0;
                if (count > 0) {
                    typeHTML += `Mk${mk}→Mk${targetForType}: ${count}, `;
                    typeTotal += count;
                }
            }
            typeHTML = typeHTML.replace(/, $/, '');
            if (typeTotal === 0) {
                typeHTML += ' <span class="muted">(' + t('belts.nothing_to_upgrade') + ')</span>';
            }
            typeHTML += '</div>';
            previewHTML += typeHTML;
            totalUpgraded += typeTotal;
        }
        previewHTML += `</div>`;

        // Add modded items warning to preview
        if (groups._modded && groups._modded.length > 0) {
            previewHTML += `<div class="preview-modded-warning">⚠️ ${t('belts.modded_preview')}</div>`;
        }

        if (totalUpgraded === 0 && (!groups._modded || groups._modded.length === 0)) {
            previewHTML = '<p class="muted">' + t('belts.nothing_found') + '</p>';
        } else {
            const pipeNote = upgradePipes ? t('belts.pipe_note') : '';
            previewHTML += `<p><strong>${t('belts.total_items', { count: totalUpgraded, mk: targetMk, pipeNote: pipeNote })}</strong></p>`;
        }

        const confirmed = await showPreview(t('belts.preview_title'), previewHTML);
        if (!confirmed) return;

        const io = await resolveChainIO(t('chain.label.belts'), 'belt-output');
        if (!io) return;
        const { inputPath, outputDir } = io;

        try {
            showInlineProgress('belt', t('belts.upgrading_writing'));
            const outputPath = buildOutputPath(outputDir, inputPath, '_upgraded');
            await window.go.main.App.UpgradeBeltsSave(inputPath, outputPath, {
                upgradeBelts,
                upgradeLifts,
                upgradePipes,
                targetMk,
                targetPipeMk: 2,
            });
            state.lastOutputPath = outputPath;
            state.lastOutputAction = t('belts.action_label');
            showToast(t('belts.upgraded_toast', { count: totalUpgraded, path: outputPath }), 'success');
        } catch (err) {
            showToast(t('belts.error_upgrading', { err: friendlyError(err) }), 'error');
        } finally {
            hideInlineProgress('belt');
        }
    });

    callbacks.onLanguageChange.push(() => {
        if (beltInventory) {
            $('btn-upgrade-belts').disabled = !renderBeltInventory();
        } else {
            const el = $('belt-inventory');
            if (el) el.innerHTML = '<p class="muted center">' + t('belts.click_scan') + '</p>';
        }
    });
}
