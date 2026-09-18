import { state, $, qsa, showToast, setSaveValid, showInlineProgress, hideInlineProgress, formatBytes, baseName, escapeHtml, callbacks, friendlyError } from '../state.js';
import { t } from '../i18n';
import { resetAnalyzeUI } from './analyze.js';
import { resetBeltUpgradeUI } from './belts.js';
import { resetDepotUI } from './depot.js';
import { resetMultipliersUI } from './multipliers.js';
import { resetOptimizeUI } from './optimize.js';
import { resetBlueprintsUI } from './blueprints.js';

let lastSaveFileListDir = null;

function gameVersion(saveVersion) {
    const v = parseInt(saveVersion);
    if (v >= 60) return '1.2';
    if (v >= 51) return '1.1';
    if (v >= 37) return '1.0';
    return 'Pre-1.0';
}

function populateInfoGrid(gridId, items) {
    const grid = $(gridId);
    grid.innerHTML = '';
    items.forEach(([label, value]) => {
        const div = document.createElement('div');
        div.className = 'info-item';
        div.innerHTML = `<div class="label">${label}</div><div class="value" title="${escapeHtml(value)}">${escapeHtml(value)}</div>`;
        grid.appendChild(div);
    });
}

export function refreshSaveInfo() {
    if (!state.lastSummary) return;
    const summary = state.lastSummary;
    const items = [
        [t('home.info.file'), summary.fileName],
        [t('home.info.size'), formatBytes(summary.fileSize)],
        [t('home.info.session'), summary.sessionName],
        [t('home.info.save_version'), `${summary.saveVersion} (${gameVersion(summary.saveVersion)})`],
    ];
    populateInfoGrid('save-info-grid', items);
    populateInfoGrid('optimize-info', [
        [t('home.info.file'), summary.fileName],
        [t('home.info.size'), formatBytes(summary.fileSize)],
        [t('home.info.session'), summary.sessionName],
    ]);
    populateInfoGrid('analyze-info', items);
}

export async function selectSaveFile(file, el) {
    qsa('.save-file-item').forEach(s => s.classList.remove('selected'));
    if (el) el.classList.add('selected');
    state.selectedSavePath = file.path;
    $('save-file-display').textContent = file.name;

    // Everything below is a clean slate for the newly selected save: no chained
    // output yet, assume it parses until we find out otherwise, and clear out
    // whatever the previous save left on the other pages.
    state.lastOutputPath = '';
    state.lastOutputAction = '';

    setSaveValid(true);

    const sessionWarning = $('session-warning');
    sessionWarning.classList.add('hidden');
    sessionWarning.classList.remove('success');
    sessionWarning.querySelector('.session-warning-header span').textContent = t('session.issues_detected');
    $('session-warning-fix').style.display = '';

    resetAnalyzeUI();
    resetBeltUpgradeUI();
    resetDepotUI();
    resetMultipliersUI();
    resetOptimizeUI();
    resetBlueprintsUI();

    try {
        showInlineProgress('home', t('home.loading'));
        const summary = await window.go.main.App.LoadSaveSummary(file.path);
        state.lastSummary = summary;
        if (callbacks.loadSettings) callbacks.loadSettings();
        const panel = $('save-info-panel');
        panel.classList.remove('hidden');
        const items = [
            [t('home.info.file'), summary.fileName],
            [t('home.info.size'), formatBytes(summary.fileSize)],
            [t('home.info.session'), summary.sessionName],
            [t('home.info.save_version'), `${summary.saveVersion} (${gameVersion(summary.saveVersion)})`],
        ];
        populateInfoGrid('save-info-grid', items);

        populateInfoGrid('optimize-info', [
            [t('home.info.file'), summary.fileName],
            [t('home.info.size'), formatBytes(summary.fileSize)],
            [t('home.info.session'), summary.sessionName],
        ]);
        $('optimize-info').classList.remove('hidden');

        populateInfoGrid('analyze-info', items);
        $('analyze-info').classList.remove('hidden');
    } catch (err) {
        showToast(t('home.error_loading', { err: friendlyError(err) }), 'error');
        setSaveValid(false);
        // Don't keep showing the previously selected save's details.
        state.lastSummary = null;
        ['save-info-panel', 'optimize-info', 'analyze-info'].forEach(id => $(id).classList.add('hidden'));
    } finally {
        hideInlineProgress('home');
    }

    // After the info panel exists, so the warning card lands underneath it.
    if (callbacks.checkSessionNameIssues) callbacks.checkSessionNameIssues();
}

export function initHome() {
    $('btn-browse-file').addEventListener('click', async () => {
        try {
            const filePath = await window.go.main.App.BrowseSaveFile();
            if (filePath) {
                const fileObj = { name: baseName(filePath), path: filePath, size: 0, modTime: '' };
                $('dir-path').value = filePath;
                $('save-file-list').innerHTML = '';
                selectSaveFile(fileObj, null);
            }
        } catch (err) {
            showToast(t('home.error_browsing', { err: friendlyError(err) }), 'error');
        }
    });

    $('btn-sfsaves').addEventListener('click', async () => {
        const listEl = $('save-file-list');
        if (lastSaveFileListDir && !listEl.classList.contains('hidden')) {
            listEl.classList.add('hidden');
            return;
        }
        try {
            const dir = await window.go.main.App.GetSatisfactorySaveDir();
            if (dir) {
                await loadSaveFileList(dir);
                listEl.classList.remove('hidden');
                $('dir-path').value = dir;
            }
        } catch (err) {
            showToast(t('home.no_saves_dir', { err: friendlyError(err) }), 'error');
        }
    });

    // Intercept help contact links - Wails webview doesn't open target="_blank"
    document.querySelectorAll('.help-contact-link').forEach(link => {
        link.addEventListener('click', async (e) => {
            e.preventDefault();
            const url = link.getAttribute('href');
            if (url) {
                try {
                    await window.go.main.App.OpenFile(url);
                } catch (err) {
                    showToast(t('home.could_not_open_link', { err: friendlyError(err) }), 'error');
                }
            }
        });
    });

    callbacks.onLanguageChange.push(() => {
        if (lastSaveFileListDir) loadSaveFileList(lastSaveFileListDir);
    });
}

async function loadSaveFileList(dir) {
    lastSaveFileListDir = dir;
    try {
        const files = await window.go.main.App.ScanSaveDir(dir);
        const listEl = $('save-file-list');
        listEl.innerHTML = '';
        if (!files || files.length === 0) {
            listEl.innerHTML = '<p class="muted center">' + t('home.no_sav_files') + '</p>';
            return;
        }
        files.forEach(f => {
            const item = document.createElement('div');
            item.className = 'save-file-item';
            item.innerHTML = `
                <span class="name">${escapeHtml(f.name)}</span>
                <span class="size">${formatBytes(f.size)}</span>
                <span class="date">${escapeHtml(f.modTime)}</span>
            `;
            item.addEventListener('click', () => selectSaveFile(f, item));
            listEl.appendChild(item);
        });
    } catch (err) {
        showToast(t('home.error_scanning', { err: friendlyError(err) }), 'error');
    }
}
