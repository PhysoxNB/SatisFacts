import { state, $, qsa, showToast, showInlineProgress, hideInlineProgress, validateSavePath, resolveChainIO, buildOutputPath, showPreview, friendlyError, callbacks } from '../state.js';
import { t } from '../i18n';

let currentMultipliers = [];

export function resetMultipliersUI() {
    hideInlineProgress('mult');
    currentMultipliers = [];
    const el = $('multiplier-list');
    if (el) el.innerHTML = '<p class="muted center">' + t('mult.click_load') + '</p>';
    // Don't touch btn-load-mult here - setSaveValid owns it.
    const saveBtn = $('btn-save-mult');
    if (saveBtn) saveBtn.disabled = true;
}

function renderMultiplierList() {
    const listEl = $('multiplier-list');
    listEl.innerHTML = '';
    if (currentMultipliers.length === 0) {
        listEl.innerHTML = '<p class="muted center">' + t('mult.no_items') + '</p>';
        return;
    }
    currentMultipliers.forEach((m, idx) => {
        const div = document.createElement('div');
        div.className = 'multiplier-item';
        const currentStr = m.current >= 0 ? m.current.toFixed(1) : t('mult.not_set');
        const notSet = m.current < 0;
        div.innerHTML = `
            <span class="label">${t('mult.label.' + m.name)}${notSet ? ' <span class="badge badge-new">' + t('mult.new_badge') + '</span>' : ''}</span>
            <span class="current">${t('mult.current_label')}: ${currentStr}</span>
            <input type="number" step="0.1" min="0" value="${m.target >= 0 ? m.target.toFixed(1) : '1.0'}" data-idx="${idx}" class="mult-input"/>
            <span class="ui-max">${t('mult.ui_max', { max: m.uiMax })}</span>
        `;
        listEl.appendChild(div);
    });
}

export function initMultipliers() {
    $('btn-load-mult').addEventListener('click', async () => {
        const validationError = validateSavePath(state.selectedSavePath);
        if (validationError) { showToast(validationError, 'error'); return; }
        try {
            showInlineProgress('mult', t('mult.reading'));
            currentMultipliers = await window.go.main.App.GetMultipliers(state.selectedSavePath);
            renderMultiplierList();
            $('btn-save-mult').disabled = false;
            showToast(t('mult.loaded'), 'success');
        } catch (err) {
            showToast(t('mult.error_loading', { err: friendlyError(err) }), 'error');
        } finally {
            hideInlineProgress('mult');
        }
    });

    $('btn-save-mult').addEventListener('click', async () => {
        const validationError = validateSavePath(state.selectedSavePath);
        if (validationError) { showToast(validationError, 'error'); return; }
        if (currentMultipliers.length === 0) return;

        const inputs = qsa('.mult-input');
        let changed = false;
        let invalidInputs = false;
        let previewHTML = '';
        inputs.forEach(input => {
            const idx = parseInt(input.dataset.idx);
            const m = currentMultipliers[idx];
            const newVal = parseFloat(input.value);
            // The backend treats negative targets as "leave this one alone".
            if (isNaN(newVal) || newVal < 0) {
                input.classList.add('input-error');
                invalidInputs = true;
                return;
            }
            input.classList.remove('input-error');
            const oldVal = m.current;
            const isInsert = oldVal < 0 && newVal >= 0;
            const isChanged = (oldVal >= 0 && Math.abs(newVal - oldVal) > 0.001) || isInsert;
            if (isChanged) changed = true;
            const oldStr = oldVal >= 0 ? oldVal.toFixed(1) : t('mult.not_set');
            if (isChanged) {
                previewHTML += `<div class="preview-row">
                    <span class="preview-label">${t('mult.label.' + m.name)}${isInsert ? ' <em>(' + t('mult.new_badge') + ')</em>' : ''}</span>
                    <span class="preview-old">${oldStr}</span>
                    <span class="preview-arrow">→</span>
                    <span class="preview-new">${newVal.toFixed(1)}</span>
                </div>`;
            } else {
                previewHTML += `<div class="preview-row">
                    <span class="preview-label">${t('mult.label.' + m.name)}</span>
                    <span class="preview-unchanged">${oldStr}</span>
                </div>`;
            }
        });
        if (invalidInputs) {
            showToast(t('mult.invalid_values'), 'error');
            return;
        }
        if (!changed) {
            previewHTML = '<p class="muted">' + t('mult.no_changes') + '</p>';
        }

        const confirmed = await showPreview(t('mult.preview_title'), previewHTML);
        if (!confirmed) return;

        const io = await resolveChainIO(t('chain.label.mult'), 'mult-output');
        if (!io) return;
        const { inputPath, outputDir } = io;
        try {
            showInlineProgress('mult', t('mult.patching'));
            inputs.forEach(input => {
                const idx = parseInt(input.dataset.idx);
                currentMultipliers[idx].target = parseFloat(input.value);
            });
            const outputPath = buildOutputPath(outputDir, inputPath, '_multipliers');
            await window.go.main.App.PatchMultipliers(inputPath, outputPath, currentMultipliers);
            state.lastOutputPath = outputPath;
            state.lastOutputAction = t('mult.action_label');
            showToast(t('mult.patched_to', { path: outputPath }), 'success');
        } catch (err) {
            showToast(t('mult.error_patching', { err: friendlyError(err) }), 'error');
        } finally {
            hideInlineProgress('mult');
        }
    });

    callbacks.onLanguageChange.push(() => {
        if (currentMultipliers.length > 0) {
            renderMultiplierList();
        } else {
            const el = $('multiplier-list');
            if (el) el.innerHTML = '<p class="muted center">' + t('mult.click_load') + '</p>';
        }
    });
}
