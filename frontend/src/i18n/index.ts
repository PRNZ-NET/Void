import enTranslations from '../translations/en';
import ruTranslations from '../translations/ru';

type TranslationKeys = 
    | 'connection.createNewChat'
    | 'connection.joinChat'
    | 'connection.nodeUrl'
    | 'connection.username'
    | 'connection.usernamePlaceholder'
    | 'connection.enterChatId'
    | 'connection.or'
    | 'connection.connect'
    | 'connection.disconnect'
    | 'chat.room'
    | 'chat.online'
    | 'chat.typeMessage'
    | 'chat.send'
    | 'chat.noMessages'
    | 'chat.encrypted'
    | 'errors.connectionFailed'
    | 'errors.sendFailed'
    | 'security.keyMismatch'
    | 'security.expected'
    | 'security.received';

type Translations = {
    [key: string]: any;
};

const translations: Record<string, Translations> = {
    en: enTranslations,
    ru: ruTranslations,
};

let currentLanguage: string = 'en';

export const setLanguage = (lang: string) => {
    if (translations[lang]) {
        currentLanguage = lang;
        localStorage.setItem('language', lang);
    }
};

export const getLanguage = (): string => {
    const saved = localStorage.getItem('language');
    if (saved && translations[saved]) {
        currentLanguage = saved;
        return saved;
    }
    return currentLanguage;
};

export const t = (key: TranslationKeys): string => {
    const keys = key.split('.');
    let value: any = translations[currentLanguage] || translations['en'];
    
    for (const k of keys) {
        value = value?.[k];
        if (value === undefined) {
            value = translations['en'];
            for (const fallbackKey of keys) {
                value = value?.[fallbackKey];
            }
            break;
        }
    }
    
    return value || key;
};

export const getAvailableLanguages = (): string[] => {
    return Object.keys(translations);
};

getLanguage();

