import Sortable from 'sortablejs';
import { state, $, showToast, showInlineProgress, hideInlineProgress, validateSavePath, resolveChainIO, buildOutputPath, showPreview, escapeHtml, shortenItemClass, friendlyError, callbacks } from '../state.js';
import { t } from '../i18n';

let depotItems = [];
let originalDepotItems = [];
let depotSortable = null;

export function resetDepotUI() {
    hideInlineProgress('depot');
    depotItems = [];
    originalDepotItems = [];
    if (depotSortable) { depotSortable.destroy(); depotSortable = null; }
    const el = $('depot-list');
    if (el) el.innerHTML = '<p class="muted center">' + t('depot.click_load') + '</p>';
    // Don't touch btn-load-depot here - setSaveValid owns it.
    const saveBtn = $('btn-save-depot');
    if (saveBtn) saveBtn.disabled = true;
    const sortWrap = $('depot-sort-wrapper');
    if (sortWrap) sortWrap.classList.add('hidden');
}

function renderDepotList() {
    const listEl = $('depot-list');
    listEl.innerHTML = '';
    if (depotItems.length === 0) {
        listEl.innerHTML = '<p class="muted center">' + t('depot.no_items') + '</p>';
        return;
    }
    depotItems.forEach((item) => {
        const div = document.createElement('div');
        div.className = 'depot-item';
        // Backend index, not display position. Sortable reads these back after
        // every drag, and positions go stale as soon as the array is reordered.
        div.dataset.depotIndex = item.index;
        const name = item.displayName || shortenItemClass(item.itemClass);
        div.innerHTML = `
            <span class="drag-handle">⠿</span>
            <span class="item-name">${escapeHtml(name)}</span>
            <span class="item-amount">${item.amount.toLocaleString()}</span>
        `;
        listEl.appendChild(div);
    });
    if (depotSortable) depotSortable.destroy();
    depotSortable = Sortable.create(listEl, {
        animation: 150,
        ghostClass: 'dragging',
        onEnd: () => {
            const byIndex = new Map(depotItems.map(it => [it.index, it]));
            const newOrder = [];
            listEl.querySelectorAll('.depot-item').forEach(el => {
                const item = byIndex.get(parseInt(el.dataset.depotIndex, 10));
                if (item) newOrder.push(item);
            });
            depotItems = newOrder;
        }
    });
}

// Re-fetches the depot so item names come back in the newly selected language.
// The display order the user dragged into place is preserved - a plain reload
// would silently throw away their unsaved reordering.
export async function reloadDepotItems() {
    if (!state.selectedSavePath || depotItems.length === 0) return;
    const previousOrder = depotItems.map(it => it.index);
    try {
        showInlineProgress('depot', t('depot.loading'));
        const fresh = await window.go.main.App.LoadDepotItems(state.selectedSavePath);
        originalDepotItems = [...fresh];
        const byIndex = new Map(fresh.map(it => [it.index, it]));
        const reordered = [];
        previousOrder.forEach(i => {
            const item = byIndex.get(i);
            if (item) { reordered.push(item); byIndex.delete(i); }
        });
        // Anything the previous list didn't know about (shouldn't happen for the
        // same file, but don't drop items if it does) goes on the end.
        byIndex.forEach(item => reordered.push(item));
        depotItems = reordered;
        renderDepotList();
    } catch (err) {
        // silent - don't toast on background reload
    } finally {
        hideInlineProgress('depot');
    }
}

// Item indices are positions within one specific file. When chaining onto an
// earlier output, whose items may be in a different order, the desired order has
// to be translated into that file's positions. Returns null if the file doesn't
// hold the same items.
export function orderForFile(desired, fileItems) {
    if (desired.length !== fileItems.length) return null;
    const positions = new Map();
    fileItems.forEach(item => {
        if (!positions.has(item.itemClass)) positions.set(item.itemClass, []);
        positions.get(item.itemClass).push(item.index);
    });
    const order = [];
    for (const item of desired) {
        const available = positions.get(item.itemClass);
        if (!available || available.length === 0) return null;
        order.push(available.shift());
    }
    return order;
}

export function initDepot() {
    $('btn-load-depot').addEventListener('click', async () => {
        const validationError = validateSavePath(state.selectedSavePath);
        if (validationError) { showToast(validationError, 'error'); return; }
        try {
            showInlineProgress('depot', t('depot.loading'));
            depotItems = await window.go.main.App.LoadDepotItems(state.selectedSavePath);
            originalDepotItems = [...depotItems];
            renderDepotList();
            $('btn-save-depot').disabled = false;
            $('depot-sort-wrapper').classList.remove('hidden');
            showToast(t('depot.loaded', { count: depotItems.length }), 'success');
        } catch (err) {
            showToast(t('depot.error_loading', { err: friendlyError(err) }), 'error');
        } finally {
            hideInlineProgress('depot');
        }
    });

    $('btn-save-depot').addEventListener('click', async () => {
        const validationError = validateSavePath(state.selectedSavePath);
        if (validationError) { showToast(validationError, 'error'); return; }
        if (depotItems.length === 0) return;

        let changed = false;
        let previewHTML = '';
        const origPos = new Map(originalDepotItems.map((o, i) => [o.index, i]));
        depotItems.forEach((item, i) => {
            const origIdx = origPos.get(item.index) ?? -1;
            const moved = origIdx !== -1 && origIdx !== i;
            if (moved) changed = true;
            previewHTML += `<div class="preview-row">
                <span class="preview-label">${escapeHtml(item.displayName || shortenItemClass(item.itemClass))}</span>
                <span class="preview-old">#${origIdx + 1}</span>
                <span class="preview-arrow">→</span>
                <span class="preview-new">#${i + 1}</span>
            </div>`;
        });
        if (!changed) {
            previewHTML = '<p class="muted">' + t('depot.no_changes') + '</p>';
        }

        const confirmed = await showPreview(t('depot.preview_title'), previewHTML);
        if (!confirmed) return;

        const io = await resolveChainIO(t('chain.label.depot'), 'depot-output');
        if (!io) return;
        const { inputPath, outputDir } = io;
        try {
            showInlineProgress('depot', t('depot.writing'));
            // Original indices in their new order. Sending [0,1,2,...] here just
            // rewrites the file unchanged.
            let newOrder = depotItems.map(it => it.index);
            if (inputPath !== state.selectedSavePath) {
                const fileItems = await window.go.main.App.LoadDepotItems(inputPath);
                newOrder = orderForFile(depotItems, fileItems);
                if (!newOrder) {
                    showToast(t('depot.chain_mismatch'), 'error');
                    return;
                }
            }
            const outputPath = buildOutputPath(outputDir, inputPath, '_reordered');
            await window.go.main.App.ReorderDepot(inputPath, outputPath, newOrder);
            state.lastOutputPath = outputPath;
            state.lastOutputAction = t('depot.action_label');
            showToast(t('depot.saved_to', { path: outputPath }), 'success');
        } catch (err) {
            showToast(t('depot.error_saving', { err: friendlyError(err) }), 'error');
        } finally {
            hideInlineProgress('depot');
        }
    });

    // Auto-sort buttons
    $('btn-depot-sort-az').addEventListener('click', () => {
        if (depotItems.length === 0) return;
        depotItems.sort((a, b) => {
            const na = (a.displayName || '').toLowerCase();
            const nb = (b.displayName || '').toLowerCase();
            return na.localeCompare(nb);
        });
        renderDepotList();
    });

    $('btn-depot-sort-most').addEventListener('click', () => {
        if (depotItems.length === 0) return;
        depotItems.sort((a, b) => {
            if (b.stackSize !== a.stackSize) return b.stackSize - a.stackSize;
            return b.amount - a.amount;
        });
        renderDepotList();
    });

    $('btn-depot-sort-least').addEventListener('click', () => {
        if (depotItems.length === 0) return;
        depotItems.sort((a, b) => {
            if (a.stackSize !== b.stackSize) return a.stackSize - b.stackSize;
            return a.amount - b.amount;
        });
        renderDepotList();
    });

    callbacks.onLanguageChange.push(() => {
        if (depotItems.length > 0) {
            renderDepotList();
        } else {
            const el = $('depot-list');
            if (el) el.innerHTML = '<p class="muted center">' + t('depot.click_load') + '</p>';
        }
    });
}
