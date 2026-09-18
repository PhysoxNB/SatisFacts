import { state, $, showToast, showInlineProgress, hideInlineProgress, escapeHtml, resolveOutputDir, baseName, confirmDialog, callbacks, friendlyError } from '../state.js';
import { t } from '../i18n';

const INVALID_CHARS = /[\\\/:*?"<>|]/;

export async function checkSessionNameIssues() {
    const warning = $('session-warning');
    const issuesEl = $('session-warning-issues');
    const fixInput = $('session-fix-input');

    if (!state.selectedSavePath) {
        warning.classList.add('hidden');
        return;
    }

    const sessionName = state.lastSummary?.sessionName;
    if (!sessionName) {
        warning.classList.add('hidden');
        return;
    }

    try {
        const result = await window.go.main.App.CheckSessionName(sessionName);

        if (!result.hasIssues) {
            warning.classList.add('hidden');
            return;
        }

        warning.classList.remove('hidden');
        warning.classList.remove('success');

        issuesEl.innerHTML = '';

        const currentDiv = document.createElement('div');
        currentDiv.className = 'session-warning-current';
        currentDiv.innerHTML = t('session.current_name', { name: '<span class="current-name">' + highlightProblematicChars(sessionName) + '</span>' });
        issuesEl.appendChild(currentDiv);

        result.issues.forEach(issue => {
            const div = document.createElement('div');
            div.className = 'session-warning-issue';
            div.textContent = issueText(issue);
            issuesEl.appendChild(div);
        });

        fixInput.value = result.autoFix;
        updateFixButtonLabel();
        // setSaveValid disables this when an earlier save failed to load.
        $('btn-session-fix').disabled = false;

    } catch (err) {
        warning.classList.add('hidden');
    }
}

function issueText(issue) {
    return t('session.issue.' + issue.type, { chars: issue.chars || '' });
}

function highlightProblematicChars(name) {
    const leadingWS = name.length - name.replace(/^\s+/, '').length;
    const trailingWS = name.length - name.replace(/\s+$/, '').length;
    const trimmedEnd = name.length - trailingWS;
    const trailingDots = trailingWS === 0 ? (name.match(/\.+$/) || [''])[0].length : 0;
    let html = '';
    for (let i = 0; i < name.length; i++) {
        const ch = name[i];
        const isLeadingWS = i < leadingWS;
        const isTrailingWS = trailingWS > 0 && i >= trimmedEnd;
        const isInvalid = INVALID_CHARS.test(ch);
        const isTrailingDot = trailingDots > 0 && i >= trimmedEnd - trailingDots && i < trimmedEnd;
        if (isInvalid || isLeadingWS || isTrailingWS || isTrailingDot) {
            html += '<span class="problematic-char">' + escapeHtml(ch) + '</span>';
        } else {
            html += escapeHtml(ch);
        }
    }
    return html;
}

function updateFixButtonLabel() {
    const overwrite = $('session-overwrite').checked;
    $('btn-session-fix').textContent = overwrite ? t('session.overwrite_original') : t('session.save_fixed');
    $('session-output-toggle').style.display = overwrite ? 'none' : '';
}

export function initSession() {
    $('session-overwrite').addEventListener('change', updateFixButtonLabel);

    $('btn-session-autofix').addEventListener('click', () => {
        if (state.selectedSavePath && state.lastSummary?.sessionName) {
            window.go.main.App.CheckSessionName(state.lastSummary.sessionName).then(result => {
                if (result.hasIssues) {
                    $('session-fix-input').value = result.autoFix;
                }
            }).catch(() => {});
        }
    });

    $('btn-session-fix').addEventListener('click', async () => {
        const newName = $('session-fix-input').value.trim();
        if (!newName) {
            showToast(t('session.enter_valid'), 'error');
            return;
        }

        const checkResult = await window.go.main.App.CheckSessionName(newName);
        if (checkResult.hasIssues) {
            showToast(t('session.still_has_issues', { issues: checkResult.issues.map(issueText).join(' ') }), 'error');
            return;
        }

        const overwrite = $('session-overwrite').checked;
        const fixBtn = $('btn-session-fix');
        const status = $('session-fix-status');

        if (overwrite) {
            const confirmed = await confirmDialog(
                t('session.overwrite_title'),
                t('session.overwrite_msg')
            );
            if (!confirmed) return;
        }

        let outputDir = '';
        if (!overwrite) {
            const { dir, error } = await resolveOutputDir('session-output');
            if (error) { showToast(error, 'error'); return; }
            outputDir = dir;
        }

        fixBtn.disabled = true;
        status.textContent = t('session.fixing');

        try {
            showInlineProgress('session', t('session.fixing_progress'));
            const result = await window.go.main.App.FixSessionName(state.selectedSavePath, newName, overwrite, outputDir);

            status.textContent = '';
            showToast(t('session.fixed_toast', { original: result.originalName, new: result.newName, count: result.bodyReplacements }), 'success');

            const warningEl = $('session-warning');
            const issuesContainer = $('session-warning-issues');
            warningEl.classList.add('success');
            warningEl.querySelector('.session-warning-header span').textContent = t('session.fixed_success');
            issuesContainer.innerHTML = '';
            const resultDiv = document.createElement('div');
            resultDiv.className = 'session-warning-issue';
            resultDiv.innerHTML = t('session.fixed_result', { original: escapeHtml(result.originalName), new: escapeHtml(result.newName), output: escapeHtml(result.outputPath) });
            issuesContainer.appendChild(resultDiv);
            $('session-warning-fix').style.display = 'none';

            // Auto-load the fixed save
            const fixedPath = result.outputPath;
            const fixedName = baseName(fixedPath);
            showToast(t('session.loading_fixed', { name: fixedName }), 'success');
            await callbacks.selectSaveFile({ path: fixedPath, name: fixedName });

        } catch (err) {
            showToast(t('session.error_fixing', { err: friendlyError(err) }), 'error');
            status.textContent = t('session.failed');
        } finally {
            fixBtn.disabled = false;
            hideInlineProgress('session');
        }
    });
}
