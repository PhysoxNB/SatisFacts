import { qsa, $, callbacks, applyZoom, persistSettings } from './state.js';
import { setLang, getLang } from './i18n';
import { initHome, selectSaveFile, refreshSaveInfo } from './features/home.js';
import { initSession, checkSessionNameIssues } from './features/session.js';
import { initOptimize } from './features/optimize.js';
import { initDepot, reloadDepotItems } from './features/depot.js';
import { initBelts } from './features/belts.js';
import { initAnalyze } from './features/analyze.js';
import { initMultipliers } from './features/multipliers.js';
import { initBlueprints } from './features/blueprints.js';
import { initSettings, initAutoDetectListener, initDragDrop, loadSettings } from './features/settings.js';
import { initMapSettings } from './features/mapsettings.js';
import { initGameConfig } from './features/config.js';

// Register late-binding callbacks for circular dependencies
callbacks.selectSaveFile = selectSaveFile;
callbacks.checkSessionNameIssues = checkSessionNameIssues;
callbacks.loadSettings = loadSettings;

// ---- i18n init ----
async function initI18n() {
    let lang = 'en';
    try {
        const settings = await window.go.main.App.GetSettings();
        if (settings && settings.language) lang = settings.language;
        if (settings && settings.zoomFactor > 0) applyZoom(settings.zoomFactor);
    } catch (e) { /* use default */ }
    setLang(lang);
    updateFlagActive(lang);
    // Notify all modules that the language is set (initial render with correct language)
    callbacks.onLanguageChange.forEach(cb => cb());
    qsa('.lang-flag').forEach(btn => {
        btn.addEventListener('click', async () => {
            const newLang = btn.dataset.lang;
            if (newLang === getLang()) return;
            setLang(newLang);
            updateFlagActive(newLang);
            try {
                await persistSettings({ language: newLang });
                await window.go.main.App.SetLanguage(newLang);
                reloadDepotItems();
                refreshSaveInfo();
                callbacks.onLanguageChange.forEach(cb => cb());
            } catch (e) { /* ignore persistence error */ }
        });
    });
}

function updateFlagActive(lang) {
    qsa('.lang-flag').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.lang === lang);
    });
}

// ---- Navigation ----
qsa('.nav-item').forEach(btn => {
    btn.addEventListener('click', () => {
        qsa('.nav-item').forEach(b => b.classList.remove('active'));
        btn.classList.add('active');
        qsa('.page').forEach(p => p.classList.remove('active'));
        $('page-' + btn.dataset.page).classList.add('active');
    });
});

// ---- Init all feature modules ----
initHome();
initSession();
initOptimize();
initDepot();
initBelts();
initAnalyze();
initMultipliers();
initBlueprints();
initSettings();
initMapSettings();
initAutoDetectListener();
initDragDrop();
initGameConfig();

// ---- Apply translations after all modules are initialized ----
initI18n();
