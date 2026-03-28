package i18n

import (
	"embed"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strings"
)

//go:embed locales/*.json
var localeFS embed.FS

var translations map[string]map[string]string

// Init loads all locale JSON files into memory.
func Init() {
	translations = make(map[string]map[string]string)
	for _, lang := range []string{"pt", "en"} {
		data, err := localeFS.ReadFile("locales/" + lang + ".json")
		if err != nil {
			log.Fatalf("i18n: failed to load locale %s: %v", lang, err)
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			log.Fatalf("i18n: failed to parse locale %s: %v", lang, err)
		}
		translations[lang] = m
	}
	log.Printf("i18n: loaded %d locales (pt=%d keys, en=%d keys)",
		len(translations), len(translations["pt"]), len(translations["en"]))
}

// T returns the translated string for the given language and key.
// Falls back to Portuguese, then returns the key itself.
func T(lang, key string) string {
	if m, ok := translations[lang]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	if lang != "pt" {
		if m, ok := translations["pt"]; ok {
			if v, ok := m[key]; ok {
				return v
			}
		}
	}
	return key
}

// TH returns a translated string marked as safe HTML.
// Use only for trusted locale strings that contain markup (e.g. <br>, <strong>).
func TH(lang, key string) template.HTML {
	return template.HTML(T(lang, key))
}

// DetectLang extracts the preferred language from the HTTP request.
// Priority: ?lang= query param > "lang" cookie > Accept-Language header > "pt".
func DetectLang(r *http.Request) string {
	if lang := r.URL.Query().Get("lang"); lang == "en" || lang == "pt" {
		return lang
	}
	if c, err := r.Cookie("lang"); err == nil {
		if c.Value == "en" || c.Value == "pt" {
			return c.Value
		}
	}
	if primaryLang(r.Header.Get("Accept-Language")) == "en" {
		return "en"
	}
	return "pt"
}

// LangHTMLAttr returns the full BCP 47 tag for the <html lang> attribute.
func LangHTMLAttr(lang string) string {
	switch lang {
	case "en":
		return "en"
	default:
		return "pt-BR"
	}
}

// primaryLang extracts the primary language from an Accept-Language header value.
func primaryLang(header string) string {
	if header == "" {
		return ""
	}
	tag := strings.SplitN(header, ",", 2)[0]
	tag = strings.TrimSpace(strings.SplitN(tag, ";", 2)[0])
	if strings.HasPrefix(tag, "en") {
		return "en"
	}
	return tag
}
