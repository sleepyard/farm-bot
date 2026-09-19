package assets

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

// templateLang 是包级模板语言设置；"en" 表示只用原始素材。
var templateLang atomic.Value // string

func init() {
	templateLang.Store("en")
}

// SetTemplateLang sets the package-level template language (e.g. "cn").
// An empty value resets to the default "en".
func SetTemplateLang(lang string) {
	if lang == "" {
		lang = "en"
	}
	templateLang.Store(lang)
}

// TemplateLang returns the package-level template language, default "en".
func TemplateLang() string {
	return templateLang.Load().(string)
}

// LangVariant maps "Buttons/keep_hand.png" + "cn" to
// "Buttons/keep_hand.cn.png", preserving the extension's original casing
// (".PNG" -> ".cn.PNG").
func LangVariant(rel, lang string) string {
	ext := filepath.Ext(rel)
	return strings.TrimSuffix(rel, ext) + "." + lang + ext
}

// ExistingVariants returns candidate absolute paths for rel, ordered
// [language variant (only if it exists on disk), original]. With lang "en"
// only the original path is returned.
func ExistingVariants(rel, lang string) []string {
	orig := filepath.Join(RootDir(), filepath.FromSlash(rel))
	if lang == "" || lang == "en" {
		return []string{orig}
	}
	variant := filepath.Join(RootDir(), filepath.FromSlash(LangVariant(rel, lang)))
	if st, err := os.Stat(variant); err == nil && !st.IsDir() {
		return []string{variant, orig}
	}
	return []string{orig}
}
