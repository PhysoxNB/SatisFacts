import './style.css';
import { t } from './i18n';

// ---- Shared State ----
export const state = {
    selectedSavePath: '',
    selectedSaveDir: '',
    saveValid: true,
    appSettings: null,
    lastOutputPath: '',
    lastOutputAction: '',
    lastSummary: null,
};

// ---- Core Helpers ----

export function $(id) { return document.getElementById(id); }
export function qsa(sel) { return document.querySelectorAll(sel); }

export function applyZoom(factor) {
    if (factor && factor > 0) {
        document.body.style.zoom = String(factor);
    }
}

let toastTimeout = null;

export function showToast(msg, type) {
    const t = $('toast');
    if (toastTimeout) {
        clearTimeout(toastTimeout);
        toastTimeout = null;
    }
    t.className = 'toast' + (type ? ' ' + type : '');
    t.innerHTML = '';

    const isLong = msg.length > 120 || msg.includes('\n');

    const body = document.createElement('div');
    body.className = 'toast-body';

    const summary = document.createElement('span');
    summary.className = 'toast-summary';
    const singleLine = msg.replace(/\n/g, ' ');
    summary.textContent = singleLine.length > 120 ? singleLine.slice(0, 117) + '...' : singleLine;
    body.appendChild(summary);

    if (isLong) {
        const details = document.createElement('pre');
        details.className = 'toast-details';
        details.textContent = msg;
        body.appendChild(details);

        body.classList.add('expandable');
        summary.style.cursor = 'pointer';
        summary.addEventListener('click', (e) => {
            if (e.target.classList.contains('toast-close')) return;
            if (window.getSelection().toString()) return;
            t.classList.toggle('expanded');
        });
    }

    t.appendChild(body);

    const closeBtn = document.createElement('button');
    closeBtn.className = 'toast-close';
    closeBtn.textContent = '×';
    closeBtn.onclick = (e) => { e.stopPropagation(); t.className = 'toast hidden'; };
    t.appendChild(closeBtn);

    if (!isLong) {
        toastTimeout = setTimeout(() => { t.className = 'toast hidden'; }, 4000);
    }
}

// Persists a partial settings change and refreshes state.appSettings from the
// value the backend actually stored.
//
// Every call site used to hand-assemble the full twelve-field Settings object,
// so a stale or not-yet-loaded state.appSettings would quietly overwrite
// unrelated preferences (e.g. toggling the map checkbox blanking customOutputDir).
// Pass only the keys you are changing.
export async function persistSettings(patch) {
    state.appSettings = await window.go.main.App.UpdateSettings(patch);
    return state.appSettings;
}

export function validateSavePath(path) {
    if (!path) return t('validate.no_save');
    if (!path.toLowerCase().endsWith('.sav')) return t('validate.not_sav');
    return null;
}

// These just need a valid save file.
const ENTRY_BUTTONS = ['btn-analyze', 'btn-optimize', 'btn-load-depot', 'btn-load-mult', 'btn-load-belts'];

// These also need data loaded first, so we only ever switch them off here -
// each feature decides when they come back on.
const ACTION_BUTTONS = ['btn-save-depot', 'btn-save-mult', 'btn-upgrade-belts', 'btn-open-report', 'btn-session-fix'];

export function setSaveValid(valid) {
    state.saveValid = valid;
    ENTRY_BUTTONS.forEach(id => {
        const btn = $(id);
        if (btn) btn.disabled = !valid;
    });
    if (!valid) {
        ACTION_BUTTONS.forEach(id => {
            const btn = $(id);
            if (btn) btn.disabled = true;
        });
    }
    const banner = $('save-error-banner');
    if (banner) banner.classList.toggle('hidden', valid);
}

export async function validateSaveFileBackend(path) {
    try {
        await window.go.main.App.ValidateSaveFile(path);
        return null;
    } catch (err) {
        return friendlyError(err);
    }
}

export async function validateOutputDirBackend(dir) {
    try {
        await window.go.main.App.ValidateOutputDir(dir);
        return null;
    } catch (err) {
        return friendlyError(err);
    }
}

// --- Inline progress bar helpers ---
// Each page that needs progress has a <div id="{prefix}-progress"> with
// <div id="{prefix}-progress-fill"> and <span id="{prefix}-progress-phase">.

export function showInlineProgress(prefix, text) {
    const el = $(`${prefix}-progress`);
    if (!el) return;
    el.classList.remove('hidden');
    const fill = $(`${prefix}-progress-fill`);
    if (fill) {
        fill.classList.add('indeterminate');
        fill.style.width = '';
        fill.style.marginLeft = '';
        fill.style.background = '';
    }
    const phase = $(`${prefix}-progress-phase`);
    if (phase) phase.textContent = text || t('common.working');
}

export function hideInlineProgress(prefix) {
    const el = $(`${prefix}-progress`);
    if (!el) return;
    el.classList.add('hidden');
}

export function setInlineProgressPhase(prefix, text) {
    const phase = $(`${prefix}-progress-phase`);
    if (phase) phase.textContent = text;
}

export function setInlineProgressValue(prefix, current, total) {
    const fill = $(`${prefix}-progress-fill`);
    if (!fill) return;
    if (total > 0) {
        fill.classList.remove('indeterminate');
        fill.style.marginLeft = '';
        const pct = Math.min(100, (current / total) * 100);
        fill.style.width = pct + '%';
    } else {
        fill.classList.add('indeterminate');
        fill.style.width = '';
    }
}

export function finishInlineProgress(prefix, text, success) {
    const fill = $(`${prefix}-progress-fill`);
    if (fill) {
        fill.classList.remove('indeterminate');
        fill.style.marginLeft = '';
        fill.style.width = success ? '100%' : '0%';
        fill.style.background = success ? 'var(--green)' : 'var(--red)';
    }
    const phase = $(`${prefix}-progress-phase`);
    if (phase) phase.textContent = text;
}

export function formatBytes(bytes) {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
    return (bytes / (1024 * 1024 * 1024)).toFixed(2) + ' GB';
}

export function shortenItemClass(path) {
    if (!path) return 'Unknown';
    const parts = path.split('.');
    return parts[parts.length - 1];
}

export function baseName(path) {
    return path.split(/[\\/]/).pop();
}

export function dirName(path) {
    const parts = path.split(/[\\/]/);
    parts.pop();
    const sep = path.includes('\\') && !path.includes('/') ? '\\' : '/';
    return parts.join(sep);
}

// basePath is what "next to save" means for this feature. Defaults to whatever
// is selected on Home; Blueprints has to pass its own target.
export async function getOutputDir(toggleName, basePath) {
    const choice = document.querySelector(`input[name="${toggleName}"]:checked`)?.value || 'exe';
    if (choice === 'save') {
        const src = basePath || state.selectedSavePath;
        if (!src) {
            throw new Error(t('validate.cannot_place'));
        }
        const dir = dirName(src);
        if (!dir) {
            throw new Error(t('validate.cannot_determine', { name: baseName(src) }));
        }
        return dir;
    }
    if (choice === 'custom') {
        const dir = state.appSettings?.customOutputDir;
        if (!dir) {
            throw new Error(t('validate.no_custom'));
        }
        return dir;
    }
    return await window.go.main.App.GetExeDir();
}

export function buildOutputPath(dir, inputPath, suffix) {
    const stem = baseName(inputPath).replace(/\.sav$/i, '');
    const sep = dir.includes('\\') && !dir.includes('/') ? '\\' : '/';
    return dir.replace(/[\\/]+$/, '') + sep + stem + suffix + '.sav';
}

// getOutputDir plus validation. Returns { dir, error } rather than throwing so a
// bad output path doesn't get reported as a corrupt save.
export async function resolveOutputDir(toggleName, basePath) {
    let dir;
    try {
        dir = await getOutputDir(toggleName, basePath);
    } catch (err) {
        return { dir: null, error: String(err && err.message ? err.message : err) };
    }
    const dirErr = await validateOutputDirBackend(dir);
    if (dirErr) return { dir: null, error: dirErr };
    return { dir, error: null };
}

// Combines askChainChoice + validateSaveFileBackend + resolveOutputDir into one call.
// Returns { inputPath, outputDir } on success, or null if cancelled or validation failed
// (toast is already shown in the error case).
export async function resolveChainIO(actionLabel, outputToggleName) {
    const choice = await askChainChoice(actionLabel);
    if (choice === null) return null;
    const inputPath = choice === 'previous' ? state.lastOutputPath : state.selectedSavePath;
    const saveErr = await validateSaveFileBackend(inputPath);
    if (saveErr) { showToast(saveErr, 'error'); return null; }
    const { dir: outputDir, error: dirErr } = await resolveOutputDir(outputToggleName);
    if (dirErr) { showToast(dirErr, 'error'); return null; }
    return { inputPath, outputDir };
}

export function askChainChoice(actionLabel) {
    return new Promise((resolve) => {
        if (!state.lastOutputPath) {
            resolve('original');
            return;
        }
        const modal = $('chain-modal');
        const text = $('chain-modal-text');
        const prevFile = baseName(state.lastOutputPath);
        const origFile = baseName(state.selectedSavePath);
        text.innerHTML = t('chain.text', { prev: escapeHtml(prevFile), action: state.lastOutputAction, label: actionLabel, orig: escapeHtml(origFile) });
        modal.classList.remove('hidden');

        const cleanup = () => {
            modal.classList.add('hidden');
            $('btn-chain-previous').onclick = null;
            $('btn-chain-original').onclick = null;
            $('btn-chain-cancel').onclick = null;
        };
        $('btn-chain-previous').onclick = () => { cleanup(); resolve('previous'); };
        $('btn-chain-original').onclick = () => { cleanup(); resolve('original'); };
        $('btn-chain-cancel').onclick = () => { cleanup(); resolve(null); };
    });
}

export function showPreview(title, contentHTML) {
    return new Promise((resolve) => {
        $('preview-title').textContent = title || t('preview.title');
        $('preview-content').innerHTML = contentHTML;
        $('preview-modal').classList.remove('hidden');

        const cleanup = () => {
            $('preview-modal').classList.add('hidden');
            $('btn-preview-confirm').onclick = null;
            $('btn-preview-cancel').onclick = null;
        };
        $('btn-preview-confirm').onclick = () => { cleanup(); resolve(true); };
        $('btn-preview-cancel').onclick = () => { cleanup(); resolve(false); };
    });
}

export function confirmDialog(title, message) {
    return new Promise((resolve) => {
        $('confirm-title').textContent = title;
        $('confirm-message').textContent = message;
        $('confirm-modal').classList.remove('hidden');

        const cleanup = () => {
            $('confirm-modal').classList.add('hidden');
            $('btn-confirm-yes').onclick = null;
            $('btn-confirm-no').onclick = null;
        };
        $('btn-confirm-yes').onclick = () => { cleanup(); resolve(true); };
        $('btn-confirm-no').onclick = () => { cleanup(); resolve(false); };
    });
}

export function infoDialog(title, message) {
    return new Promise((resolve) => {
        $('info-title').textContent = title;
        $('info-message').textContent = message;
        $('info-modal').classList.remove('hidden');

        const cleanup = () => {
            $('info-modal').classList.add('hidden');
            $('btn-info-ok').onclick = null;
        };
        $('btn-info-ok').onclick = () => { cleanup(); resolve(); };
    });
}

// Quotes are escaped too: the result also goes into title="..." attributes, and a
// session name read from a save could otherwise break out of them.
export function escapeHtml(s) {
    return String(s ?? '')
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
}

// Backend errors a user can run into, as [pattern, translation key]. The pattern's
// first capture group (a file name or path) is passed along as {detail}.
const ERROR_MESSAGES = [
    [/no save file selected/, 'validate.no_save'],
    [/selected file is not a \.sav file/, 'validate.not_sav'],
    [/file not found: (.*)/, 'error.file_not_found'],
    [/file is empty \(0 bytes\): (.*)/, 'error.file_empty'],
    [/selected path is a directory, not a file: (.*)/, 'error.path_is_dir'],
    [/no output directory specified/, 'validate.no_custom'],
    [/output directory does not exist: (.*)/, 'error.output_missing'],
    [/output path is not a directory: (.*)/, 'error.output_not_dir'],
    [/output directory is not writable: (.*)/, 'error.output_not_writable'],
    [/an analysis is already running/, 'error.analyze_busy'],
    [/an optimization is already running/, 'error.optimize_busy'],
    [/new session name contains non-ASCII characters/, 'error.session_non_ascii'],
    [/new session name is longer than the original/, 'error.session_too_long'],
    [/was not found in the save body/, 'error.session_not_in_body'],
    [/save directory not accessible/, 'error.watch_dir_missing'],
];

// Maps known backend error strings to user-friendly translated messages.
// Falls back to the raw error string if no match is found.
export function friendlyError(err) {
    const raw = String(err && err.message ? err.message : err);

    // Depot: mStoredItems not found -> depot not unlocked
    if (raw.includes('mStoredItems property not found')) {
        return t('error.depot_not_unlocked');
    }
    // Blueprints: subsystem not found
    if (raw.includes('BlueprintSubsystem not found')) {
        return t('error.no_blueprint_subsystem');
    }
    // Blueprints: category records property not found
    if (raw.includes('mBlueprintCategoryRecords property not found')) {
        return t('error.no_blueprint_categories');
    }
    // Multipliers: GameState not found
    if (raw.includes('GameState object not found')) {
        return t('error.no_gamestate');
    }
    // Generic: corrupt save (from recover panics)
    if (raw.includes('the file may be corrupt')) {
        return t('error.corrupt_save', { detail: raw });
    }

    for (const [pattern, key] of ERROR_MESSAGES) {
        const match = raw.match(pattern);
        if (match) return t(key, { detail: match[1] ?? '' });
    }

    return raw;
}

// ---- Late binding for circular imports ----
// home.js and session.js/settings.js need each other. main.js fills these in
// after every module has loaded, which sidesteps the import cycle.
export const callbacks = {
    selectSaveFile: null,
    checkSessionNameIssues: null,
    loadSettings: null,
    onLanguageChange: [],
};
