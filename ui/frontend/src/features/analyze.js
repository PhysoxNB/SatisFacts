import { state, $, showToast, showInlineProgress, setInlineProgressValue, finishInlineProgress, validateSavePath, validateSaveFileBackend, resolveOutputDir, friendlyError, persistSettings, escapeHtml } from '../state.js';
import { t } from '../i18n';

let lastReportPath = '';
let analyzeProgressListenerRegistered = false;

export function resetAnalyzeUI() {
    $('analyze-progress').classList.add('hidden');
    $('analyze-step-list').innerHTML = '';
    const fill = $('analyze-progress-fill');
    fill.classList.remove('indeterminate');
    fill.style.marginLeft = '';
    fill.style.width = '';
    fill.style.background = '';
    $('analyze-progress-phase').textContent = '';
    $('analyze-result').classList.add('hidden');
    $('btn-open-report').disabled = true;
    lastReportPath = '';
}

function ensureAnalyzeProgressListener() {
    if (analyzeProgressListenerRegistered) return;
    if (!window.runtime) return;
    analyzeProgressListenerRegistered = true;
    window.runtime.EventsOn('analyze:progress', (update) => {
        const fill = $('analyze-progress-fill');
        const phaseLabel = $('analyze-progress-phase');
        const stepList = $('analyze-step-list');

        const phaseText = update.phase ? t('analyze.phase.' + update.phase) : t('common.working');
        phaseLabel.textContent = phaseText;

        if (update.totalSteps > 0 && stepList.children.length === 0) {
            stepList.innerHTML = '';
            for (let i = 1; i <= update.totalSteps; i++) {
                const item = document.createElement('div');
                item.className = 'analyze-step pending';
                item.id = 'analyze-step-' + i;
                item.innerHTML = `<span class="step-icon"></span><span class="step-label">${t('analyze.step', { n: i })}</span>`;
                stepList.appendChild(item);
            }
        }

        if (update.totalSteps > 0) {
            for (let i = 1; i <= update.totalSteps; i++) {
                const el = $('analyze-step-' + i);
                if (!el) continue;
                el.classList.remove('pending', 'active', 'done');
                if (i < update.step) {
                    el.classList.add('done');
                    el.querySelector('.step-label').textContent = el.dataset.label || t('analyze.step', { n: i });
                } else if (i === update.step) {
                    el.classList.add('active');
                    el.querySelector('.step-label').textContent = phaseText;
                    el.dataset.label = phaseText;
                } else {
                    el.classList.add('pending');
                }
            }
        }

        if (update.total > 0) {
            setInlineProgressValue('analyze', update.current, update.total);
        } else {
            fill.classList.add('indeterminate');
            fill.style.width = '';
        }
    });
}

// Restores the map checkbox from persisted settings. Called by settings.js once
// state.appSettings is populated - initAnalyze runs before the first
// GetSettings() resolves, so reading appSettings there always saw null and the
// checkbox stayed on its HTML default of checked.
export function applyAnalyzeSettings() {
    const includeMapCheckbox = $('analyze-include-map');
    if (!includeMapCheckbox || !state.appSettings) return;
    includeMapCheckbox.checked = state.appSettings.includeMap !== false;
}

export function initAnalyze() {
    const includeMapCheckbox = $('analyze-include-map');

    $('btn-analyze').addEventListener('click', async () => {
        const validationError = validateSavePath(state.selectedSavePath);
        if (validationError) { showToast(validationError, 'error'); return; }
        ensureAnalyzeProgressListener();

        // Checked up front, outside the try - otherwise a bad output directory
        // comes back as "Analysis failed" and disables every button.
        const saveErr = await validateSaveFileBackend(state.selectedSavePath);
        if (saveErr) { showToast(saveErr, 'error'); return; }
        const { dir: outputDir, error: dirErr } = await resolveOutputDir('analyze-output');
        if (dirErr) { showToast(dirErr, 'error'); return; }

        const btnAnalyze = $('btn-analyze');
        const btnOpenReport = $('btn-open-report');
        try {
            btnAnalyze.disabled = true;
            btnOpenReport.disabled = true;
            $('btn-analyze-cancel').classList.remove('hidden');
            showInlineProgress('analyze', t('analyze.starting'));
            $('analyze-step-list').innerHTML = '';
            const analyzeResult = $('analyze-result');
            analyzeResult.classList.add('hidden');

            const includeMap = includeMapCheckbox.checked;
            const result = await window.go.main.App.AnalyzeSave(state.selectedSavePath, outputDir, includeMap);

            finishInlineProgress('analyze', t('analyze.report_saved'), true);
            document.querySelectorAll('.analyze-step').forEach(step => {
                step.classList.remove('pending', 'active');
                step.classList.add('done');
            });
            lastReportPath = result.htmlPath;

            analyzeResult.classList.remove('hidden');
            let optimizeHint = '';
            if (result.canOptimize) {
                optimizeHint = `
                <div class="optimize-hint">
                    <span>${t('analyze.optimize_hint')}</span>
                    <button class="btn btn-small" id="btn-hint-optimize">${t('analyze.optimize_go')}</button>
                </div>`;
            }
            const reportDir = result.htmlPath.replace(/[/\\][^/\\]+$/, '');
            analyzeResult.innerHTML = `
                <div class="savings">${t('analyze.report_generated')}</div>
                <p><strong>${result.totalObjects.toLocaleString()}</strong> ${t('analyze.objects')} · <strong>${result.totalBuildings.toLocaleString()}</strong> ${t('analyze.buildings')} · <strong>${result.htmlSizeKB.toFixed(0)}</strong> KB HTML</p>
                <p class="report-path">📄 ${t('analyze.saved_to')} <span title="${escapeHtml(result.htmlPath)}">${escapeHtml(result.htmlPath)}</span></p>
                <p class="muted">${t('analyze.completed_in', { time: result.elapsed })}</p>
                ${optimizeHint}
            `;
            btnOpenReport.disabled = false;
            const hintBtn = $('btn-hint-optimize');
            if (hintBtn) {
                hintBtn.addEventListener('click', () => {
                    document.querySelector('.nav-item[data-page="optimize"]').click();
                });
            }
            // This link is embedded in the analyze.completed_in translation
            // string above, so it only exists after the innerHTML assignment.
            const openFolderBtn = $('btn-open-folder');
            if (openFolderBtn) {
                openFolderBtn.addEventListener('click', async (e) => {
                    e.preventDefault();
                    try {
                        await window.go.main.App.OpenFile(reportDir);
                    } catch (err) {
                        showToast(t('analyze.could_not_open_folder', { err: friendlyError(err) }), 'error');
                    }
                });
            }
            showToast(t('analyze.report_saved_to', { path: result.htmlPath }), 'success');
        } catch (err) {
            const msg = String(err);
            if (msg.includes('cancelled')) {
                showToast(t('analyze.cancelled'), 'info');
                finishInlineProgress('analyze', t('common.cancelled'), false);
            } else {
                showToast(t('analyze.failed', { err: friendlyError(err) }), 'error');
                const activeStep = $('analyze-step-list').querySelector('.analyze-step.active');
                if (activeStep) {
                    activeStep.classList.remove('active');
                    activeStep.classList.add('failed');
                    const label = activeStep.querySelector('.step-label');
                    if (label) label.textContent = (label.textContent || t('analyze.step_short')) + ' — ' + t('common.failed');
                }
                finishInlineProgress('analyze', t('common.failed'), false);
            }
        } finally {
            $('btn-analyze-cancel').classList.add('hidden');
            if (state.saveValid) btnAnalyze.disabled = false;
        }
    });

    $('btn-analyze-cancel').addEventListener('click', () => {
        window.go.main.App.CancelAnalyze();
    });

    $('btn-open-report').addEventListener('click', async () => {
        if (!lastReportPath) return;
        try {
            await window.go.main.App.OpenFile(lastReportPath);
        } catch (err) {
            showToast(t('analyze.could_not_open_report', { err }), 'error');
        }
    });

    includeMapCheckbox.addEventListener('change', async () => {
        try {
            await persistSettings({ includeMap: includeMapCheckbox.checked });
        } catch (err) {
            // non-fatal — checkbox still works for this session
        }
    });
}
