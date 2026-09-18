import { state, $, showToast, escapeHtml, callbacks, friendlyError, applyZoom, persistSettings } from '../state.js';
import { applyAnalyzeSettings } from './analyze.js';
import { t } from '../i18n';

let lastUpdateResult = null;
let lastUpdateError = null;

export async function loadSettings() {
    try {
        state.appSettings = await window.go.main.App.GetSettings();
        applySettingsToUI();
        renderRecentSaves();
    } catch (err) {
        // Settings not available yet (runtime not ready)
    }
}

function setZoomSelectValue(select, zf) {
    if (!select) return;
    const val = String(zf);
    // Find a matching option (String(1.0) returns "1", but option value is "1.0")
    for (const opt of select.options) {
        if (parseFloat(opt.value) === zf) {
            select.value = opt.value;
            return;
        }
    }
    select.value = val;
}

function applySettingsToUI() {
    if (!state.appSettings) return;

    const outputNames = ['opt-output', 'analyze-output', 'depot-output', 'mult-output', 'belt-output', 'session-output', 'bp-output'];
    outputNames.forEach(name => {
        const radio = document.querySelector(`input[name="${name}"][value="${state.appSettings.defaultOutputLoc}"]`);
        if (radio && !radio.disabled) radio.checked = true;
    });

    updateCustomOutputToggles();

    const settingsOutputRadio = document.querySelector(`input[name="settings-output"][value="${state.appSettings.defaultOutputLoc}"]`);
    if (settingsOutputRadio) settingsOutputRadio.checked = true;

    const customPathInput = $('settings-custom-output-path');
    if (customPathInput) customPathInput.value = state.appSettings.customOutputDir || '';

    updateCustomOutputRow();
    updateCustomOutputToggles();

    const autoDetect = $('settings-auto-detect');
    const autoAnalyze = $('settings-auto-analyze');
    if (autoDetect) autoDetect.checked = state.appSettings.autoDetectSave || false;
    if (autoAnalyze) autoAnalyze.checked = state.appSettings.autoAnalyze || false;
    updateAutoAnalyzeDisabled();

    const headerZoom = $('header-zoom');
    if (headerZoom) {
        setZoomSelectValue(headerZoom, state.appSettings.zoomFactor || 1.0);
    }

    // Owned by analyze.js, but it can only be applied once settings have
    // actually arrived from the backend - which is here, not at init time.
    applyAnalyzeSettings();

    populateAutoDetectPath();
}

function renderRecentSaves() {
    const recentSaves = $('recent-saves');
    const settingsList = $('settings-recent-list');
    if (!state.appSettings || !state.appSettings.recentSaves || state.appSettings.recentSaves.length === 0) {
        recentSaves.classList.add('hidden');
        settingsList.innerHTML = '<p class="muted">' + t('settings.no_recent') + '</p>';
        return;
    }

    recentSaves.classList.remove('hidden');
    const homeList = $('recent-saves-list');
    homeList.innerHTML = '';
    settingsList.innerHTML = '';

    state.appSettings.recentSaves.forEach((save, i) => {
        const item = document.createElement('div');
        item.className = 'save-file-item';
        item.innerHTML = `<span class="save-name">${escapeHtml(save.name)}</span><span class="path" title="${escapeHtml(save.path)}">${escapeHtml(save.path)}</span>`;
        item.addEventListener('click', () => {
            callbacks.selectSaveFile({ path: save.path, name: save.name });
        });
        homeList.appendChild(item);

        const sItem = document.createElement('div');
        sItem.className = 'save-file-item';
        sItem.innerHTML = `<span class="save-name">${escapeHtml(save.name)}</span><span class="path" title="${escapeHtml(save.path)}">${escapeHtml(save.path)}</span>`;
        const actions = document.createElement('div');
        actions.className = 'recent-save-actions';
        const loadBtn = document.createElement('button');
        loadBtn.className = 'btn';
        loadBtn.textContent = t('settings.load');
        loadBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            callbacks.selectSaveFile({ path: save.path, name: save.name });
        });
        const removeBtn = document.createElement('button');
        removeBtn.className = 'btn';
        removeBtn.textContent = '×';
        removeBtn.title = t('settings.remove_recent');
        removeBtn.addEventListener('click', async (e) => {
            e.stopPropagation();
            await window.go.main.App.RemoveRecentSave(i);
            state.appSettings = await window.go.main.App.GetSettings();
            renderRecentSaves();
        });
        actions.appendChild(loadBtn);
        actions.appendChild(removeBtn);
        sItem.appendChild(actions);
        settingsList.appendChild(sItem);
    });
}

function updateAutoAnalyzeDisabled() {
    const autoDetect = $('settings-auto-detect');
    const autoAnalyze = $('settings-auto-analyze');
    if (autoDetect && autoAnalyze) {
        autoAnalyze.disabled = !autoDetect.checked;
    }
}

async function syncAutoDetect() {
    if (!state.appSettings?.autoDetectSave) {
        await window.go.main.App.StopAutoDetect();
        return;
    }
    try {
        const saveDir = await window.go.main.App.GetAutoDetectPath();
        await window.go.main.App.StartAutoDetect(saveDir);
    } catch (err) {
        showToast(t('settings.autodetect_failed', { err: friendlyError(err) }), 'error');
    }
}

async function populateAutoDetectPath() {
    const pathInput = $('settings-autodetect-path');
    if (!pathInput) return;
    try {
        const dir = await window.go.main.App.GetAutoDetectPath();
        if (dir) {
            pathInput.value = dir;
            pathInput.placeholder = '';
        }
    } catch (err) {
        // Directory not found
    }
}

// Only the preferences this page actually owns are sent. Language, map colors,
// includeMap and the window size are left untouched by omitting them.
async function autoSaveSettings() {
    const patch = {
        defaultOutputLoc: document.querySelector('input[name="settings-output"]:checked')?.value || 'exe',
        autoDetectSave: $('settings-auto-detect').checked,
        autoAnalyze: $('settings-auto-analyze').checked,
        zoomFactor: parseFloat($('header-zoom')?.value || '1.0'),
    };
    // These live in state because they're set by a directory picker, not a
    // control on the page. Only send them if we actually have a value.
    if (state.appSettings?.customOutputDir !== undefined) {
        patch.customOutputDir = state.appSettings.customOutputDir;
    }
    if (state.appSettings?.customSaveDir !== undefined) {
        patch.customSaveDir = state.appSettings.customSaveDir;
    }
    try {
        await persistSettings(patch);
        await syncAutoDetect();
        showToast(t('settings.saved'), 'success');
    } catch (err) {
        showToast(t('settings.save_failed', { err: friendlyError(err) }), 'error');
    }
}

function updateCustomOutputToggles() {
    const hasCustomPath = !!(state.appSettings && state.appSettings.customOutputDir);
    const tip = hasCustomPath
        ? t('settings.custom_output_tooltip_set', { path: state.appSettings.customOutputDir })
        : t('settings.custom_output_tooltip_unset');
    document.querySelectorAll('.custom-output-toggle').forEach(label => {
        const radio = label.querySelector('input[type="radio"]');
        if (radio) radio.disabled = !hasCustomPath;
        label.title = tip;
    });
}

function updateCustomOutputRow() {
    const customRow = $('settings-custom-output-row');
    const pathInput = $('settings-custom-output-path');
    if (!customRow || !pathInput) return;
    // Always show the custom path row so users can see/set the path.
    const hasCustomPath = !!(state.appSettings && state.appSettings.customOutputDir);
    if (hasCustomPath) {
        pathInput.value = state.appSettings.customOutputDir;
        pathInput.classList.remove('input-warning');
        pathInput.placeholder = '';
    } else {
        pathInput.value = '';
        pathInput.classList.add('input-warning');
        pathInput.placeholder = t('settings.choose_dir');
    }
}

async function browseCustomOutput() {
    try {
        const dir = await window.go.main.App.BrowseDir();
        if (!dir) return;
        state.appSettings = state.appSettings || {};
        state.appSettings.customOutputDir = dir;
        // Check the Custom radio BEFORE saving so autoSaveSettings picks up 'custom'
        const customRadio = $('settings-output-custom');
        if (customRadio) customRadio.checked = true;
        const pathInput = $('settings-custom-output-path');
        if (pathInput) pathInput.value = dir;
        updateCustomOutputToggles();
        updateCustomOutputRow();
        await autoSaveSettings();
        showToast(t('settings.custom_path_set'), 'success');
    } catch (err) {
        showToast(t('settings.custom_path_failed', { err: friendlyError(err) }), 'error');
    }
}

export function initSettings() {
    // No explicit save button on this page - radios persist as you click them.
    document.querySelectorAll('input[name="settings-output"]').forEach(radio => {
        radio.addEventListener('change', async () => {
            if (radio.name === 'settings-output' && radio.value === 'custom' && !state.appSettings?.customOutputDir) {
                // No path set yet - save the selection but let the visual indicator show
                await autoSaveSettings();
                updateCustomOutputRow();
                return;
            }
            await autoSaveSettings();
            applySettingsToUI();
        });
    });

    $('btn-settings-custom-browse').addEventListener('click', browseCustomOutput);

    $('btn-clear-recent').addEventListener('click', async () => {
        try {
            await window.go.main.App.ClearRecentSaves();
            state.appSettings = await window.go.main.App.GetSettings();
            renderRecentSaves();
            showToast(t('settings.recent_cleared'), 'success');
        } catch (err) {
            showToast(t('settings.clear_failed', { err: friendlyError(err) }), 'error');
        }
    });

    $('settings-auto-detect').addEventListener('change', () => {
        updateAutoAnalyzeDisabled();
        autoSaveSettings();
    });

    $('settings-auto-analyze').addEventListener('change', () => {
        autoSaveSettings();
    });

    $('btn-autodetect-browse').addEventListener('click', async () => {
        try {
            const dir = await window.go.main.App.BrowseDir();
            if (!dir) return;
            state.appSettings = state.appSettings || {};
            state.appSettings.customSaveDir = dir;
            $('settings-autodetect-path').value = dir;
            await autoSaveSettings();
            showToast(t('settings.watch_updated'), 'success');
        } catch (err) {
            showToast(t('settings.watch_failed', { err: friendlyError(err) }), 'error');
        }
    });

    $('btn-autodetect-reset').addEventListener('click', async () => {
        state.appSettings = state.appSettings || {};
        state.appSettings.customSaveDir = '';
        await populateAutoDetectPath();
        await autoSaveSettings();
        showToast(t('settings.reset_to_default'), 'success');
    });

    const headerZoom = $('header-zoom');
    if (headerZoom) {
        headerZoom.addEventListener('change', () => {
            applyZoom(parseFloat(headerZoom.value));
            autoSaveSettings();
        });
    }

    $('btn-check-updates').addEventListener('click', checkForUpdates);

    // Display current version on load
    function initVersionDisplay() {
        if (window.go?.main?.App) {
            window.go.main.App.GetAppVersion().then(v => {
                const el = $('app-version');
                if (el) el.textContent = 'v' + v;
            }).catch(() => {});
        } else {
            setTimeout(initVersionDisplay, 200);
        }
    }
    initVersionDisplay();

    callbacks.onLanguageChange.push(() => {
        renderRecentSaves();
        updateCustomOutputToggles();
        updateCustomOutputRow();
        if (lastUpdateResult || lastUpdateError) renderUpdateStatus();
        const btn = $('btn-check-updates');
        if (btn && !btn.disabled) btn.textContent = t('settings.check_updates');
    });

    // The Wails runtime isn't there on first paint, so keep retrying.
    function initSettingsRetry() {
        if (window.go?.main?.App) {
            loadSettings().then(() => {
                syncAutoDetect();
            });
        } else {
            setTimeout(initSettingsRetry, 200);
        }
    }
    initSettingsRetry();
}

async function checkForUpdates() {
    const btn = $('btn-check-updates');
    const status = $('update-status');
    if (!btn || !status) return;

    btn.disabled = true;
    btn.textContent = t('settings.checking');
    status.classList.add('hidden');

    try {
        lastUpdateResult = await window.go.main.App.CheckForUpdates();
        lastUpdateError = null;
        renderUpdateStatus();
    } catch (err) {
        lastUpdateResult = null;
        lastUpdateError = err;
        renderUpdateStatus();
    } finally {
        btn.disabled = false;
        btn.textContent = t('settings.check_updates');
    }
}

function renderUpdateStatus() {
    const status = $('update-status');
    if (!status) return;

    if (lastUpdateError) {
        status.className = 'update-status update-error';
        status.classList.remove('hidden');
        status.innerHTML = `<strong>${t('settings.check_failed')}</strong><p class="muted" style="margin: 4px 0 0;">${escapeHtml(friendlyError(lastUpdateError))}</p>`;
        return;
    }

    if (!lastUpdateResult) return;
    const result = lastUpdateResult;
    status.classList.remove('hidden');

    if (result.updateAvailable) {
        status.className = 'update-status update-available';
        status.innerHTML = `
            <div class="update-status-header">
                <span class="update-icon">↑</span>
                <strong>${t('settings.update_available', { version: result.latestVersion })}</strong>
            </div>
            <p class="muted" style="margin: 6px 0 10px;">${t('settings.current_is', { version: result.currentVersion })}</p>
            <button class="btn btn-primary" id="btn-download-update">${t('settings.download')}</button>
        `;
        const dlBtn = $('btn-download-update');
        if (dlBtn) {
            dlBtn.addEventListener('click', () => {
                window.go.main.App.OpenFile(result.releaseURL);
            });
        }
    } else {
        status.className = 'update-status update-uptodate';
        status.innerHTML = `
            <div class="update-status-header">
                <span class="update-icon">✓</span>
                <strong>${t('settings.up_to_date', { version: result.currentVersion })}</strong>
            </div>
        `;
    }
}

// ---- Auto-Detect Listener ----

let autoDetectListenerRegistered = false;

export function initAutoDetectListener() {
    function retry() {
        if (window.runtime) {
            ensureAutoDetectListener();
        } else {
            setTimeout(retry, 200);
        }
    }
    retry();
}

function ensureAutoDetectListener() {
    if (autoDetectListenerRegistered) return;
    if (!window.runtime) return;
    autoDetectListenerRegistered = true;
    window.runtime.EventsOn('save:auto-detected', async (path) => {
        const fileName = path.split('/').pop().split('\\').pop();
        await callbacks.selectSaveFile({ path, name: fileName });
        if (state.appSettings?.autoAnalyze && state.saveValid) {
            document.querySelector('.nav-item[data-page="analyze"]').click();
            $('btn-analyze').click();
        }
    });
}

// ---- Drag and Drop ----

export function initDragDrop() {
    let dragCounter = 0;

    window.addEventListener('dragenter', (e) => {
        if (e.dataTransfer && e.dataTransfer.types.includes('Files')) {
            e.preventDefault();
            dragCounter++;
            $('drop-overlay').classList.remove('hidden');
        }
    });
    window.addEventListener('dragleave', (e) => {
        if (e.dataTransfer && e.dataTransfer.types.includes('Files')) {
            e.preventDefault();
            dragCounter--;
            if (dragCounter <= 0) {
                dragCounter = 0;
                $('drop-overlay').classList.add('hidden');
            }
        }
    });
    window.addEventListener('dragover', (e) => {
        if (e.dataTransfer && e.dataTransfer.types.includes('Files')) {
            e.preventDefault();
        }
    });
    window.addEventListener('drop', (e) => {
        if (e.dataTransfer && e.dataTransfer.types.includes('Files')) {
            e.preventDefault();
            dragCounter = 0;
            $('drop-overlay').classList.add('hidden');
        }
    });

    function initFileDrop() {
        if (!window.runtime) {
            setTimeout(initFileDrop, 200);
            return;
        }
        window.runtime.EventsOn('file:drop', (path) => {
            if (!path) return;
            const name = path.split(/[\\/]/).pop();
            document.querySelector('.nav-item[data-page="home"]').click();
            callbacks.selectSaveFile({ path, name });
            showToast(t('settings.loaded_toast', { name }), 'success');
        });
    }
    initFileDrop();
}
