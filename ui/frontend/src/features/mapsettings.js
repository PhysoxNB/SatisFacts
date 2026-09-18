import { $, state, showToast, friendlyError, callbacks, persistSettings } from '../state.js';
import { t, applyTranslations } from '../i18n';

const DEFAULT_MAP_COLORS = {
    production:    '#4a9eff',
    miner:         '#ffaa44',
    station:       '#ff66aa',
    power_gen:     '#ff6600',
    power_storage: '#ffdd44',
    power_pole:    '#ddaa44',
    signal:        '#ff4444',
    belt:          '#88dd88',
    pipe:          '#ff8800',
    rail:          '#aa88ff',
    hypertube:     '#ff8866',
    special:       '#fbbf24',
    resource_node: '#88ff88',
    structure:     '#888888',
    foundation:    '#cccccc',
    blue_slug:     '#4a9eff',
    yellow_slug:   '#f5a623',
    purple_slug:   '#9b59b6',
    somersloop:    '#e91e63',
    mercer_sphere: '#a855f7',
    crash_site:    '#e67e22',
};

const COLOR_GROUPS = [
    { label: 'mapsettings.group.production', cats: ['production', 'miner'] },
    { label: 'mapsettings.group.power', cats: ['power_gen', 'power_storage', 'power_pole'] },
    { label: 'mapsettings.group.transport', cats: ['belt', 'pipe', 'rail', 'hypertube', 'signal'] },
    { label: 'mapsettings.group.stations', cats: ['station'] },
    { label: 'mapsettings.group.special', cats: ['special'] },
    { label: 'mapsettings.group.build', cats: ['structure', 'foundation'] },
    { label: 'mapsettings.group.collectibles', cats: ['blue_slug', 'yellow_slug', 'purple_slug', 'somersloop', 'mercer_sphere', 'crash_site'] },
];

const CAT_LABELS = {
    production: 'mapsettings.cat.production',
    miner: 'mapsettings.cat.miner',
    station: 'mapsettings.cat.station',
    power_gen: 'mapsettings.cat.power_gen',
    power_storage: 'mapsettings.cat.power_storage',
    power_pole: 'mapsettings.cat.power_pole',
    signal: 'mapsettings.cat.signal',
    belt: 'mapsettings.cat.belt',
    pipe: 'mapsettings.cat.pipe',
    rail: 'mapsettings.cat.rail',
    hypertube: 'mapsettings.cat.hypertube',
    special: 'mapsettings.cat.special',
    resource_node: 'mapsettings.cat.resource_node',
    structure: 'mapsettings.cat.structure',
    foundation: 'mapsettings.cat.foundation',
    blue_slug: 'mapsettings.cat.blue_slug',
    yellow_slug: 'mapsettings.cat.yellow_slug',
    purple_slug: 'mapsettings.cat.purple_slug',
    somersloop: 'mapsettings.cat.somersloop',
    mercer_sphere: 'mapsettings.cat.mercer_sphere',
    crash_site: 'mapsettings.cat.crash_site',
};

function getColor(cat) {
    if (state.appSettings?.mapColors && state.appSettings.mapColors[cat]) {
        return state.appSettings.mapColors[cat];
    }
    return DEFAULT_MAP_COLORS[cat] || '#ffffff';
}

async function buildColorUI() {
    const container = $('map-colors-container');
    if (!container) return;
    // Ensure settings are loaded so getColor() returns saved values
    if (!state.appSettings) {
        try {
            state.appSettings = await window.go.main.App.GetSettings();
        } catch (e) { return; }
    }
    let html = '';
    for (const group of COLOR_GROUPS) {
        html += '<div class="card" style="margin-bottom: 12px;">';
        html += '<h2>' + t(group.label) + '</h2>';
        html += '<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(220px,1fr));gap:8px 16px;">';
        for (const cat of group.cats) {
            const color = getColor(cat);
            const label = t(CAT_LABELS[cat] || cat);
            html += '<div style="display:flex;align-items:center;gap:10px;padding:6px 8px;background:#1a1a22;border-radius:6px;">';
            html += '<input type="color" data-map-color="' + cat + '" value="' + color + '" style="width:36px;height:28px;border:1px solid #444;border-radius:4px;cursor:pointer;background:none;flex-shrink:0;">';
            html += '<label style="font-size:14px;cursor:pointer;">' + label + '</label>';
            html += '</div>';
        }
        html += '</div>';
        html += '</div>';
    }
    container.innerHTML = html;
    applyTranslations(container);

    container.querySelectorAll('input[data-map-color]').forEach(input => {
        input.dataset.prevColor = input.value;
        input.addEventListener('change', async () => {
            if (input.value === input.dataset.prevColor) return;
            input.dataset.prevColor = input.value;
            const cat = input.dataset.mapColor;
            await saveMapColor(cat, input.value);
        });
    });
}

async function saveMapColor(cat, hex) {
    try {
        // Read fresh rather than trusting state.appSettings, so two quick edits
        // in a row can't drop the first one.
        const settings = await window.go.main.App.GetSettings();
        const mapColors = Object.assign({}, settings.mapColors);
        mapColors[cat] = hex;
        await persistSettings({ mapColors });
        showToast(t('settings.saved'), 'success');
    } catch (err) {
        showToast(t('settings.save_failed', { err: friendlyError(err) }), 'error');
    }
}

export function initMapSettings() {
    const resetBtn = $('btn-reset-map-colors');
    if (resetBtn) {
        resetBtn.addEventListener('click', async () => {
            try {
                await persistSettings({ mapColors: {} });
                buildColorUI();
                showToast(t('mapsettings.reset_done'), 'success');
            } catch (err) {
                showToast(t('settings.save_failed', { err: friendlyError(err) }), 'error');
            }
        });
    }

    // Register language change callback first (before any await) so it's ready immediately
    callbacks.onLanguageChange.push(() => {
        buildColorUI();
    });
}