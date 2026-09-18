import { $, showToast, showInlineProgress, hideInlineProgress, validateSaveFileBackend, resolveOutputDir, buildOutputPath, showPreview, escapeHtml, formatBytes, infoDialog, friendlyError, callbacks } from '../state.js';
import { t } from '../i18n';

let bpSourcePath = '';
let bpTargetPath = '';
let bpPresetPath = '';
let bpSourceCategories = null;
let bpTargetCategories = null;
let bpSourceType = 'save'; // 'save' or 'preset'

async function loadSourceCategories() {
    if (!bpSourcePath) return;
    try {
        showInlineProgress('bp', t('bp.loading'));
        bpSourceCategories = await window.go.main.App.GetBlueprintCategories(bpSourcePath);
        renderBpCategories('bp-source-categories', bpSourceCategories);
        updateBpCopyButton();
        $('btn-bp-export').disabled = bpSourceCategories.length === 0;
        showToast(t('bp.loaded', { count: bpSourceCategories.length }), 'success');
    } catch (err) {
        bpSourceCategories = null;
        renderBpCategories('bp-source-categories', null);
        updateBpCopyButton();
        $('btn-bp-export').disabled = true;
        showToast(t('bp.error_loading', { err: friendlyError(err) }), 'error');
    } finally {
        hideInlineProgress('bp');
    }
}

export function resetBlueprintsUI() {
    bpSourcePath = '';
    bpTargetPath = '';
    bpPresetPath = '';
    bpSourceCategories = null;
    bpTargetCategories = null;
    bpSourceType = 'save';
    hideInlineProgress('bp');
    $('bp-source-path').value = '';
    $('bp-target-path').value = '';
    $('bp-preset-path').value = '';
    $('btn-bp-load-source').disabled = true;
    $('btn-bp-preset-load').disabled = true;
    $('btn-bp-export').disabled = true;
    $('btn-bp-copy').disabled = true;
    $('bp-source-save-list').classList.add('hidden');
    $('bp-target-save-list').classList.add('hidden');
    $('bp-source-categories').innerHTML = '<p class="muted center">' + t('bp.source_hint') + '</p>';
    $('bp-target-categories').innerHTML = '<p class="muted center">' + t('bp.target_hint') + '</p>';
    $('bp-source-summary').textContent = t('bp.source_categories');
    $('bp-target-summary').textContent = t('bp.target_categories');
    $('bp-source-details').open = false;
    $('bp-target-details').open = false;
    $('bp-result').classList.add('hidden');
    // Reset to save source type
    $('bp-source-type-save').checked = true;
    switchSourceType('save');
}

async function bpBrowseSave(inputId, isSource) {
    try {
        const filePath = await window.go.main.App.BrowseSaveFile();
        if (filePath) {
            if (isSource) {
                bpSourcePath = filePath;
                $('bp-source-path').value = filePath;
                $('btn-bp-load-source').disabled = false;
                $('btn-bp-export').disabled = true;
                bpSourceCategories = null;
                renderBpCategories('bp-source-categories', null);
                loadSourceCategories();
            } else {
                bpTargetPath = filePath;
                $('bp-target-path').value = filePath;
                bpTargetCategories = null;
                renderBpCategories('bp-target-categories', null);
                loadTargetCategories();
            }
            updateBpCopyButton();
        }
    } catch (err) {
        showToast(t('bp.error_browsing', { err: friendlyError(err) }), 'error');
    }
}

async function loadTargetCategories() {
    if (!bpTargetPath) return;
    try {
        bpTargetCategories = await window.go.main.App.GetBlueprintCategories(bpTargetPath);
        renderBpCategories('bp-target-categories', bpTargetCategories);
        updateBpCopyButton();
        if (bpTargetCategories.length === 0) {
            showToast(t('bp.target_no_blueprints'), 'info');
        }
    } catch (err) {
        bpTargetCategories = [];
        renderBpCategories('bp-target-categories', []);
        updateBpCopyButton();
        showToast(t('bp.error_target', { err: friendlyError(err) }), 'error');
    }
}

function switchSourceType(type) {
    bpSourceType = type;
    if (type === 'save') {
        $('bp-source-save-section').classList.remove('hidden');
        $('bp-source-preset-section').classList.add('hidden');
        $('bp-source-categories').innerHTML = '<p class="muted center">' + t('bp.source_hint') + '</p>';
        $('bp-source-summary').textContent = t('bp.source_categories');
        $('bp-source-details').open = false;
        bpSourceCategories = null;
        bpPresetPath = '';
        $('bp-preset-path').value = '';
    } else {
        $('bp-source-save-section').classList.add('hidden');
        $('bp-source-preset-section').classList.remove('hidden');
        $('bp-source-categories').innerHTML = '<p class="muted center">' + t('bp.preset_hint') + '</p>';
        $('bp-source-summary').textContent = t('bp.source_categories');
        $('bp-source-details').open = false;
        bpSourceCategories = null;
        bpSourcePath = '';
        $('bp-source-path').value = '';
        $('btn-bp-export').disabled = true;
    }
    updateBpCopyButton();
}

async function browsePresetFile() {
    try {
        const filePath = await window.go.main.App.BrowsePresetFile();
        if (filePath) {
            bpPresetPath = filePath;
            $('bp-preset-path').value = filePath;
            $('btn-bp-preset-load').disabled = false;
            bpSourceCategories = null;
            renderBpCategories('bp-source-categories', null);
            updateBpCopyButton();
            loadPreset();
        }
    } catch (err) {
        showToast(t('bp.error_browsing', { err: friendlyError(err) }), 'error');
    }
}

async function loadPreset() {
    if (!bpPresetPath) return;
    try {
        showInlineProgress('bp', t('bp.loading_preset'));
        // Read the preset file's JSON metadata to display categories
        const cats = await window.go.main.App.ReadPresetCategories(bpPresetPath);
        bpSourceCategories = cats;
        renderBpCategories('bp-source-categories', cats);
        updateBpCopyButton();
        showToast(t('bp.preset_loaded', { count: cats.length }), 'success');
    } catch (err) {
        bpSourceCategories = null;
        renderBpCategories('bp-source-categories', null);
        updateBpCopyButton();
        showToast(t('bp.error_loading_preset', { err: friendlyError(err) }), 'error');
    } finally {
        hideInlineProgress('bp');
    }
}

async function exportPreset() {
    if (!bpSourcePath || !bpSourceCategories || bpSourceCategories.length === 0) return;
    try {
        const { dir: outputDir, error: dirErr } = await resolveOutputDir('bp-output', bpSourcePath);
        if (dirErr) { showToast(dirErr, 'error'); return; }

        const stem = bpSourcePath.split(/[\\/]/).pop().replace(/\.sav$/i, '');
        const sep = outputDir.includes('\\') && !outputDir.includes('/') ? '\\' : '/';
        const outputPath = outputDir.replace(/[\\/]+$/, '') + sep + stem + '_blueprints.sfbcat';

        showInlineProgress('bp', t('bp.exporting'));
        const result = await window.go.main.App.ExportBlueprintPreset(bpSourcePath, outputPath);
        showToast(t('bp.exported_toast', { count: result.categories, path: outputPath }), 'success');
    } catch (err) {
        showToast(t('bp.error_exporting', { err: friendlyError(err) }), 'error');
    } finally {
        hideInlineProgress('bp');
    }
}

function renderBpCategories(containerId, categories) {
    const el = $(containerId);
    const summaryId = containerId === 'bp-source-categories' ? 'bp-source-summary' : 'bp-target-summary';
    const summary = $(summaryId);

    if (!categories || categories.length === 0) {
        el.innerHTML = '<p class="muted center">' + t('bp.no_categories') + '</p>';
        if (summary) summary.textContent = summary.dataset.i18n === 'bp.source_categories' ? t('bp.source_categories') : t('bp.target_categories');
        return;
    }

    let totalBps = 0;
    categories.forEach(cat => {
        if (cat.subCategories) {
            cat.subCategories.forEach(sub => {
                totalBps += sub.blueprintNames ? sub.blueprintNames.length : 0;
            });
        }
    });

    if (summary) {
        const label = summaryId === 'bp-source-summary' ? t('bp.source_categories') : t('bp.target_categories');
        summary.textContent = label + ' (' + categories.length + ' · ' + totalBps + ' ' + t('bp.blueprints_short') + ')';
    }

    let html = '';
    categories.forEach((cat) => {
        const subCount = cat.subCategories ? cat.subCategories.length : 0;
        let totalBps = 0;
        if (cat.subCategories) {
            cat.subCategories.forEach(sub => {
                totalBps += sub.blueprintNames ? sub.blueprintNames.length : 0;
            });
        }
        const iconLabel = cat.isUndefined ? ' <span class="muted">' + t('bp.default') + '</span>' : '';
        html += `<div class="bp-category-card">`;
        html += `<div class="bp-category-header">`;
        html += `<strong>${escapeHtml(cat.categoryName)}</strong>${iconLabel}`;
        html += `<span class="muted"> ${t('bp.sub_categories', { count: subCount, bps: totalBps })}</span>`;
        html += `</div>`;
        if (cat.subCategories && cat.subCategories.length > 0) {
            html += `<div class="bp-subcategory-list">`;
            cat.subCategories.forEach(sub => {
                const bpCount = sub.blueprintNames ? sub.blueprintNames.length : 0;
                const subIcon = sub.isUndefined ? ' <span class="muted">' + t('bp.default') + '</span>' : '';
                html += `<div class="bp-subcategory">`;
                html += `<span class="bp-sub-name">${escapeHtml(sub.subCategoryName)}</span>${subIcon}`;
                html += `<span class="muted"> ${t('bp.blueprints_count', { count: bpCount })}</span>`;
                if (sub.blueprintNames && sub.blueprintNames.length > 0) {
                    html += `<div class="bp-names">`;
                    html += sub.blueprintNames.map(n => `<span class="bp-name-tag">${escapeHtml(n)}</span>`).join('');
                    html += `</div>`;
                }
                html += `</div>`;
            });
            html += `</div>`;
        }
        html += `</div>`;
    });
    el.innerHTML = html;
}

function updateBpCopyButton() {
    const btn = $('btn-bp-copy');
    const hasSource = bpSourceType === 'save'
        ? (bpSourcePath && bpSourceCategories && bpSourceCategories.length > 0)
        : (bpPresetPath && bpSourceCategories && bpSourceCategories.length > 0);
    const canCopy = hasSource && bpTargetPath;
    btn.disabled = !canCopy;
}

function renderBpSaveList(containerId, files, isSource) {
    const el = $(containerId);
    if (!files || files.length === 0) {
        el.innerHTML = '<p class="muted">' + t('bp.no_files') + '</p>';
        el.classList.remove('hidden');
        return;
    }
    el.innerHTML = '';
    files.forEach(file => {
        const item = document.createElement('div');
        item.className = 'save-file-item';
        item.innerHTML = `<span class="save-name">${escapeHtml(file.name)}</span><span class="size">${formatBytes(file.size)}</span><span class="date">${file.modTime || ''}</span>`;
        item.addEventListener('click', () => {
            if (isSource) {
                bpSourcePath = file.path;
                $('bp-source-path').value = file.path;
                $('btn-bp-load-source').disabled = false;
                bpSourceCategories = null;
                renderBpCategories('bp-source-categories', null);
                loadSourceCategories();
            } else {
                bpTargetPath = file.path;
                $('bp-target-path').value = file.path;
                bpTargetCategories = null;
                renderBpCategories('bp-target-categories', null);
                loadTargetCategories();
            }
            updateBpCopyButton();
            el.classList.add('hidden');
        });
        el.appendChild(item);
    });
    el.classList.remove('hidden');
}

export function initBlueprints() {
    $('btn-bp-source-browse').addEventListener('click', () => bpBrowseSave('bp-source-path', true));
    $('btn-bp-target-browse').addEventListener('click', () => bpBrowseSave('bp-target-path', false));

    $('btn-bp-source-saves').addEventListener('click', async () => {
        try {
            const saveDir = await window.go.main.App.GetSatisfactorySaveDir();
            const files = await window.go.main.App.ScanSaveDir(saveDir);
            renderBpSaveList('bp-source-save-list', files, true);
        } catch (err) {
            showToast(t('bp.error_loading_saves', { err: friendlyError(err) }), 'error');
        }
    });

    $('btn-bp-target-saves').addEventListener('click', async () => {
        try {
            const saveDir = await window.go.main.App.GetSatisfactorySaveDir();
            const files = await window.go.main.App.ScanSaveDir(saveDir);
            renderBpSaveList('bp-target-save-list', files, false);
        } catch (err) {
            showToast(t('bp.error_loading_saves', { err: friendlyError(err) }), 'error');
        }
    });

    $('btn-bp-load-source').addEventListener('click', loadSourceCategories);
    $('btn-bp-reset').addEventListener('click', resetBlueprintsUI);

    // Source type toggle
    $('bp-source-type-save').addEventListener('change', () => switchSourceType('save'));
    $('bp-source-type-preset').addEventListener('change', () => switchSourceType('preset'));

    // Preset file browsing
    $('btn-bp-preset-browse').addEventListener('click', browsePresetFile);
    $('btn-bp-preset-load').addEventListener('click', loadPreset);

    // Export preset
    $('btn-bp-export').addEventListener('click', exportPreset);

    $('btn-bp-copy').addEventListener('click', async () => {
        const hasSource = bpSourceType === 'save'
            ? (bpSourcePath && bpSourceCategories)
            : (bpPresetPath && bpSourceCategories);
        if (!hasSource || !bpTargetPath || !bpSourceCategories) {
            showToast(t('bp.select_both'), 'error');
            return;
        }

        const mode = document.querySelector('input[name="bp-mode"]:checked')?.value || 'replace';

        let previewHTML = '<div class="preview-belt-list">';
        previewHTML += `<p>${mode === 'replace' ? t('bp.preview_mode_replace') : t('bp.preview_mode_merge')}</p>`;
        previewHTML += `<p>${t('bp.preview_source', { count: bpSourceCategories.length })}</p>`;
        if (bpTargetCategories) {
            previewHTML += `<p>${t('bp.preview_target', { count: bpTargetCategories.length })}</p>`;
        }
        if (mode === 'merge' && bpTargetCategories) {
            const targetNames = new Set(bpTargetCategories.map(c => c.categoryName.toLowerCase()));
            const newCats = bpSourceCategories.filter(c => !targetNames.has(c.categoryName.toLowerCase()));
            previewHTML += `<p>${t('bp.preview_to_add', { count: newCats.length })}</p>`;
            if (newCats.length > 0) {
                previewHTML += '<ul>';
                newCats.forEach(c => {
                    previewHTML += `<li>${escapeHtml(c.categoryName)}</li>`;
                });
                previewHTML += '</ul>';
            } else {
                previewHTML += '<p class="muted">' + t('bp.preview_all_exist') + '</p>';
            }
        }
        previewHTML += '</div>';

        const confirmed = await showPreview(t('bp.preview_title'), previewHTML);
        if (!confirmed) return;

        try {
            const saveErr = await validateSaveFileBackend(bpTargetPath);
            if (saveErr) { showToast(saveErr, 'error'); return; }

            const { dir: outputDir, error: dirErr } = await resolveOutputDir('bp-output', bpTargetPath);
            if (dirErr) { showToast(dirErr, 'error'); return; }

            showInlineProgress('bp', t('bp.copying'));
            const outputPath = buildOutputPath(outputDir, bpTargetPath, '_blueprints');

            let result;
            if (bpSourceType === 'preset') {
                result = await window.go.main.App.ImportBlueprintPreset(bpPresetPath, bpTargetPath, outputPath, mode);
            } else {
                result = await window.go.main.App.CopyBlueprintCategories(bpSourcePath, bpTargetPath, outputPath, mode);
            }

            const bpResult = $('bp-result');
            bpResult.classList.remove('hidden');
            bpResult.innerHTML = `
                <div class="result-success">
                    <p>${t('bp.result', { source: result.sourceCategories, final: result.finalCategories, mode: result.mode })}</p>
                    <p class="muted">${t('bp.result_output', { path: escapeHtml(outputPath) })}</p>
                    <button id="btn-bp-open-dir" class="btn" style="margin-top: 10px;">${t('bp.open_blueprint_dir')}</button>
                </div>
            `;
            $('btn-bp-open-dir').addEventListener('click', async () => {
                try {
                    await window.go.main.App.OpenBlueprintDir(bpTargetPath);
                } catch (err) {
                    showToast(t('bp.error_opening_dir', { err }), 'error');
                }
            });
            showToast(t('bp.copied_toast', { count: result.sourceCategories, mode, path: outputPath }), 'success');
            hideInlineProgress('bp');
            await infoDialog(
                t('bp.info_title'),
                t('bp.info_msg')
            );
        } catch (err) {
            showToast(t('bp.error_copying', { err: friendlyError(err) }), 'error');
        } finally {
            hideInlineProgress('bp');
        }
    });

    callbacks.onLanguageChange.push(() => {
        if (!bpSourcePath && !bpPresetPath) {
            $('bp-source-categories').innerHTML = '<p class="muted center">' + t('bp.source_hint') + '</p>';
            $('bp-target-categories').innerHTML = '<p class="muted center">' + t('bp.target_hint') + '</p>';
            $('bp-source-summary').textContent = t('bp.source_categories');
            $('bp-target-summary').textContent = t('bp.target_categories');
        }
    });
}
