import { $, showToast, infoDialog, friendlyError, callbacks } from '../state.js';
import { t } from '../i18n';

let configDir = '';
let currentStatus = null;

export function initGameConfig() {
    // Auto-detect on first load
    detectConfigDir();

    $('btn-gameconfig-detect').addEventListener('click', () => {
        detectConfigDir();
    });

    $('btn-gameconfig-browse').addEventListener('click', async () => {
        try {
            const dir = await window.go.main.App.BrowseDir();
            if (dir) {
                configDir = dir;
                $('gameconfig-dir').value = dir;
                $('gameconfig-dir-hint').textContent = '';
                await refreshStatus();
            }
        } catch (err) {
            showToast(t('gameconfig.error_browsing', { err: friendlyError(err) }), 'error');
        }
    });

    $('btn-gameconfig-engine-apply').addEventListener('click', async () => {
        if (!configDir) { showToast(t('gameconfig.no_dir'), 'error'); return; }
        try {
            await window.go.main.App.ApplyEngineIniPatch(configDir);
            await infoDialog(t('gameconfig.engine_updated_title'), t('gameconfig.engine_updated_msg'));
            showToast(t('gameconfig.uobject_success'), 'success');
            await refreshStatus();
        } catch (err) {
            showToast(t('gameconfig.error_engine', { err: friendlyError(err) }), 'error');
        }
    });

    $('btn-gameconfig-engine-restore').addEventListener('click', async () => {
        if (!configDir) return;
        try {
            await window.go.main.App.RestoreConfigFile(configDir, 'Engine.ini');
            showToast(t('gameconfig.engine_restored'), 'success');
            await refreshStatus();
        } catch (err) {
            showToast(t('gameconfig.error_restoring', { err: friendlyError(err) }), 'error');
        }
    });

    $('btn-gameconfig-input-apply').addEventListener('click', async () => {
        if (!configDir) { showToast(t('gameconfig.no_dir'), 'error'); return; }
        const key = $('gameconfig-console-key').value;
        try {
            await window.go.main.App.ApplyInputIniPatch(configDir, key);
            await window.go.main.App.SetInputIniReadOnly(configDir, true);
            await infoDialog(t('gameconfig.input_updated_title'), t('gameconfig.input_updated_msg', { key }));
            showToast(t('gameconfig.console_key_toast', { key }), 'success');
            await refreshStatus();
        } catch (err) {
            showToast(t('gameconfig.error_input', { err: friendlyError(err) }), 'error');
        }
    });

    $('btn-gameconfig-input-restore').addEventListener('click', async () => {
        if (!configDir) return;
        try {
            await window.go.main.App.RestoreConfigFile(configDir, 'Input.ini');
            showToast(t('gameconfig.input_restored'), 'success');
            await refreshStatus();
        } catch (err) {
            showToast(t('gameconfig.error_restoring', { err: friendlyError(err) }), 'error');
        }
    });

    callbacks.onLanguageChange.push(() => {
        if (configDir) {
            if (currentStatus) {
                updateEngineStatus(currentStatus.engineIni);
                updateInputStatus(currentStatus.inputIni);
            }
            const hint = $('gameconfig-dir-hint');
            if (hint) hint.textContent = t('gameconfig.auto_detected');
        } else {
            const hint = $('gameconfig-dir-hint');
            if (hint) hint.textContent = t('gameconfig.not_detected');
        }
    });
}

async function detectConfigDir() {
    try {
        const dir = await window.go.main.App.DetectGameConfigDir();
        if (dir) {
            configDir = dir;
            $('gameconfig-dir').value = dir;
            $('gameconfig-dir-hint').textContent = t('gameconfig.auto_detected');
            await refreshStatus();
        } else {
            $('gameconfig-dir').value = '';
            $('gameconfig-dir-hint').textContent = t('gameconfig.not_detected');
            updateEngineStatus(null);
            updateInputStatus(null);
        }
    } catch (err) {
        showToast(t('gameconfig.error_detecting', { err: friendlyError(err) }), 'error');
    }
}

async function refreshStatus() {
    if (!configDir) return;
    try {
        currentStatus = await window.go.main.App.GetGameConfigStatus(configDir);
        updateEngineStatus(currentStatus.engineIni);
        updateInputStatus(currentStatus.inputIni);
    } catch (err) {
        showToast(t('gameconfig.error_reading', { err: friendlyError(err) }), 'error');
    }
}

function updateEngineStatus(info) {
    const el = $('gameconfig-engine-status');
    const applyBtn = $('btn-gameconfig-engine-apply');
    const restoreBtn = $('btn-gameconfig-engine-restore');
    const contentEl = $('gameconfig-engine-content');

    if (!info || !info.exists) {
        el.innerHTML = '<span class="config-status-badge config-status-unknown">' + t('gameconfig.file_not_found') + '</span>';
        applyBtn.disabled = !configDir;
        restoreBtn.disabled = true;
        contentEl.textContent = info ? info.content || t('gameconfig.file_not_exist') : t('gameconfig.no_config_dir');
        return;
    }

    if (info.hasEntry) {
        el.innerHTML = '<span class="config-status-badge config-status-applied">' + t('gameconfig.uobject_applied') + '</span>';
        applyBtn.disabled = false;
        applyBtn.textContent = t('gameconfig.uobject_reapply');
    } else {
        el.innerHTML = '<span class="config-status-badge config-status-default">' + t('gameconfig.uobject_default') + '</span>';
        applyBtn.disabled = false;
        applyBtn.textContent = t('gameconfig.uobject_increase');
    }

    restoreBtn.disabled = !info.hasBackup;
    restoreBtn.title = info.hasBackup ? t('gameconfig.restore_engine_hint') : t('gameconfig.no_backup');
    contentEl.textContent = info.content || t('gameconfig.empty_file');
}

function updateInputStatus(info) {
    const el = $('gameconfig-input-status');
    const applyBtn = $('btn-gameconfig-input-apply');
    const restoreBtn = $('btn-gameconfig-input-restore');
    const contentEl = $('gameconfig-input-content');

    if (!info || !info.exists) {
        el.innerHTML = '<span class="config-status-badge config-status-unknown">' + t('gameconfig.file_not_found') + '</span>';
        applyBtn.disabled = !configDir;
        restoreBtn.disabled = true;
        contentEl.textContent = info ? info.content || t('gameconfig.file_not_exist') : t('gameconfig.no_config_dir');
        return;
    }

    if (info.hasEntry) {
        let badge = '<span class="config-status-badge config-status-applied">' + t('gameconfig.console_set') + '</span>';
        if (!info.readOnly) {
            badge += ' <span class="config-status-badge config-status-warning">' + t('gameconfig.not_readonly') + '</span>';
        }
        el.innerHTML = badge;
        applyBtn.disabled = false;
        applyBtn.textContent = t('gameconfig.uobject_reapply');
    } else {
        el.innerHTML = '<span class="config-status-badge config-status-default">' + t('gameconfig.no_console_key') + '</span>';
        applyBtn.disabled = false;
        applyBtn.textContent = t('gameconfig.set_console_key');
    }

    restoreBtn.disabled = !info.hasBackup;
    restoreBtn.title = info.hasBackup ? t('gameconfig.restore_input_hint') : t('gameconfig.no_backup');
    contentEl.textContent = info.content || t('gameconfig.empty_file');
}
