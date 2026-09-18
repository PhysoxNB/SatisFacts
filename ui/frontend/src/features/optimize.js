import { state, $, showToast, showInlineProgress, hideInlineProgress, setInlineProgressPhase, setInlineProgressValue, finishInlineProgress, validateSavePath, resolveChainIO, buildOutputPath, friendlyError, escapeHtml, confirmDialog } from '../state.js';
import { t } from '../i18n';

let optimizeProgressListenerRegistered = false;

function ensureOptimizeProgressListener() {
    if (optimizeProgressListenerRegistered) return;
    if (!window.runtime) return;
    optimizeProgressListenerRegistered = true;
    window.runtime.EventsOn('optimize:progress', (update) => {
        setInlineProgressPhase('optimize', update.phase ? t('optimize.phase.' + update.phase) : t('common.working'));
        if (update.total > 0) {
            setInlineProgressValue('optimize', update.current, update.total);
        } else {
            const fill = $('optimize-progress-fill');
            fill.classList.add('indeterminate');
            fill.style.width = '';
        }
    });
}

// Switching any output location invalidates the chained path from the last run,
// since it points into the old directory.
const OUTPUT_TOGGLES = [
    'opt-output', 'analyze-output', 'depot-output', 'mult-output',
    'belt-output', 'bp-output', 'session-output',
];

// Running out of memory kills the app with no error message, so warn up front.
// If the check itself fails, carry on: loading the save will report any real problem.
async function confirmEnoughMemory(path) {
    let check;
    try {
        check = await window.go.main.App.CheckOptimizeMemory(path);
    } catch (err) {
        return true;
    }
    if (!check || check.enough) return true;
    return confirmDialog(t('optimize.memory_title'), t('optimize.memory_msg', {
        needed: check.neededGB.toFixed(1),
        available: check.availableGB.toFixed(1),
    }));
}

export function resetOptimizeUI() {
    hideInlineProgress('optimize');
    $('optimize-result').classList.add('hidden');
}

export function initOptimize() {
    document.addEventListener('change', (e) => {
        if (e.target.name && OUTPUT_TOGGLES.includes(e.target.name)) {
            state.lastOutputPath = '';
            state.lastOutputAction = '';
        }
    });

    $('btn-optimize').addEventListener('click', async () => {
        const validationError = validateSavePath(state.selectedSavePath);
        if (validationError) { showToast(validationError, 'error'); return; }
        const io = await resolveChainIO(t('chain.label.optimize'), 'opt-output');
        if (!io) return;
        const { inputPath, outputDir } = io;
        if (!await confirmEnoughMemory(inputPath)) return;
        ensureOptimizeProgressListener();
        const btnOpt = $('btn-optimize');
        try {
            btnOpt.disabled = true;
            $('btn-optimize-cancel').classList.remove('hidden');
            showInlineProgress('optimize', t('optimize.loading'));
            const optResult = $('optimize-result');
            optResult.classList.add('hidden');
            const outputPath = buildOutputPath(outputDir, inputPath, '_optimized');
            const result = await window.go.main.App.OptimizeSave(inputPath, outputPath);
            finishInlineProgress('optimize', t('common.done'), true);
            optResult.classList.remove('hidden');
            optResult.innerHTML = `
                <div class="savings">${result.savingsPct.toFixed(1)}%</div>
                <p>${t('optimize.result_original', { orig: result.originalSizeMB.toFixed(1), new: result.newSizeMB.toFixed(1) })}</p>
                <p>${t('optimize.result_saved', { saved: result.savingsMB.toFixed(1), objects: result.totalObjects.toLocaleString(), levels: result.levelCount })}</p>
                <p class="muted">${t('optimize.output', { path: escapeHtml(outputPath) })}</p>
            `;
            state.lastOutputPath = outputPath;
            state.lastOutputAction = t('optimize.action_label');
            showToast(t('optimize.complete'), 'success');
        } catch (err) {
            const msg = String(err);
            if (msg.includes('cancelled')) {
                showToast(t('optimize.cancelled'), 'info');
                finishInlineProgress('optimize', t('common.cancelled'), false);
            } else {
                showToast(t('optimize.failed', { err: friendlyError(err) }), 'error');
                finishInlineProgress('optimize', t('common.failed'), false);
            }
        } finally {
            btnOpt.disabled = false;
            $('btn-optimize-cancel').classList.add('hidden');
        }
    });

    $('btn-optimize-cancel').addEventListener('click', async () => {
        try {
            await window.go.main.App.CancelOptimize();
        } catch (err) {
            // Cancel call failed - likely no optimization running
        }
    });
}
