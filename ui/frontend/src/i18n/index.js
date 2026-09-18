import en from './en.js';
import de from './de.js';

const dictionaries = { en, de };

let currentLang = 'en';
let currentDict = dictionaries.en;

export function setLang(lang) {
    if (!dictionaries[lang]) return;
    currentLang = lang;
    currentDict = dictionaries[lang];
    applyTranslations();
}

export function getLang() {
    return currentLang;
}

export function t(key, params) {
    let str = currentDict[key] ?? dictionaries.en[key] ?? key;
    if (params) {
        for (const [k, v] of Object.entries(params)) {
            str = str.replaceAll(`{${k}}`, v);
        }
    }
    return str;
}

export function availableLangs() {
    return Object.keys(dictionaries);
}

export function applyTranslations(root = document) {
    root.querySelectorAll('[data-i18n]').forEach(el => {
        el.textContent = t(el.dataset.i18n);
    });
    root.querySelectorAll('[data-i18n-html]').forEach(el => {
        el.innerHTML = t(el.dataset.i18nHtml);
    });
    root.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
        el.placeholder = t(el.dataset.i18nPlaceholder);
    });
    root.querySelectorAll('[data-i18n-title]').forEach(el => {
        el.title = t(el.dataset.i18nTitle);
    });
}
