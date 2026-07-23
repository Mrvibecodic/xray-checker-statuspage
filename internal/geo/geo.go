// Package geo — определение страны по имени сервера и «чистое» отображаемое имя.
package geo

import (
	"regexp"
	"strings"
)

// RUMonths — короткие русские месяцы (индекс 1..12).
var RUMonths = []string{"", "янв", "фев", "мар", "апр", "май", "июн",
	"июл", "авг", "сен", "окт", "ноя", "дек"}

type ccKW struct{ kw, cc string }

var countryKeywords = []ccKW{
	{"netherland", "nl"}, {"нидерланд", "nl"}, {"holland", "nl"}, {"amsterdam", "nl"},
	{"germany", "de"}, {"герман", "de"}, {"frankfurt", "de"}, {"deutsch", "de"},
	{"finland", "fi"}, {"финлянд", "fi"}, {"helsinki", "fi"},
	{"united states", "us"}, {"usa", "us"}, {"сша", "us"}, {"america", "us"},
	{"new york", "us"}, {"los angeles", "us"}, {"miami", "us"}, {"dallas", "us"}, {"seattle", "us"},
	{"united kingdom", "gb"}, {"britain", "gb"}, {"england", "gb"}, {"london", "gb"},
	{"великобритан", "gb"}, {"англия", "gb"},
	{"france", "fr"}, {"франц", "fr"}, {"paris", "fr"}, {"marseille", "fr"},
	{"japan", "jp"}, {"япон", "jp"}, {"tokyo", "jp"}, {"osaka", "jp"},
	{"singapore", "sg"}, {"сингапур", "sg"},
	{"turkey", "tr"}, {"турц", "tr"}, {"istanbul", "tr"}, {"стамбул", "tr"},
	{"russia", "ru"}, {"росси", "ru"}, {"moscow", "ru"}, {"москва", "ru"}, {"питер", "ru"},
	{"poland", "pl"}, {"польш", "pl"}, {"warsaw", "pl"},
	{"sweden", "se"}, {"швец", "se"}, {"stockholm", "se"},
	{"switzerland", "ch"}, {"швейцар", "ch"}, {"zurich", "ch"},
	{"canada", "ca"}, {"канад", "ca"}, {"toronto", "ca"},
	{"italy", "it"}, {"italia", "it"}, {"итал", "it"}, {"milan", "it"}, {"milano", "it"},
	{"rome", "it"}, {"roma", "it"}, {"naples", "it"}, {"napoli", "it"}, {"неапол", "it"},
	{"turin", "it"}, {"torino", "it"}, {"турин", "it"}, {"venice", "it"}, {"venezia", "it"}, {"венеци", "it"},
	{"spain", "es"}, {"испан", "es"}, {"madrid", "es"},
	{"hong kong", "hk"}, {"гонконг", "hk"},
	{"korea", "kr"}, {"корея", "kr"}, {"seoul", "kr"},
	{"india", "in"}, {"инди", "in"}, {"mumbai", "in"},
	{"austria", "at"}, {"австри", "at"}, {"vienna", "at"},
	{"norway", "no"}, {"норвег", "no"}, {"oslo", "no"},
	{"denmark", "dk"}, {"дани", "dk"},
	{"ireland", "ie"}, {"ирланд", "ie"}, {"dublin", "ie"},
	{"czech", "cz"}, {"чех", "cz"}, {"prague", "cz"},
	{"ukraine", "ua"}, {"украин", "ua"}, {"kyiv", "ua"}, {"kiev", "ua"},
	{"emirates", "ae"}, {"dubai", "ae"}, {"оаэ", "ae"}, {"uae", "ae"},
	{"israel", "il"}, {"израил", "il"},
	{"brazil", "br"}, {"бразил", "br"},
	{"australia", "au"}, {"австрал", "au"}, {"sydney", "au"},
	{"china", "cn"}, {"китай", "cn"},
	{"hungary", "hu"}, {"венгр", "hu"},
	{"romania", "ro"}, {"румын", "ro"},
	{"bulgaria", "bg"}, {"болгар", "bg"},
	{"latvia", "lv"}, {"латви", "lv"}, {"riga", "lv"},
	{"lithuania", "lt"}, {"литва", "lt"},
	{"estonia", "ee"}, {"эстони", "ee"},
	{"kazakhstan", "kz"}, {"казахстан", "kz"},
	{"georgia", "ge"}, {"груз", "ge"},
	{"armenia", "am"}, {"армени", "am"},
	{"serbia", "rs"}, {"серб", "rs"},
	{"greece", "gr"}, {"греци", "gr"},
	{"portugal", "pt"}, {"португал", "pt"},
	{"belgium", "be"}, {"бельги", "be"},
	{"mexico", "mx"}, {"мексик", "mx"},
	{"argentina", "ar"}, {"аргентин", "ar"},
	{"europe", "eu"}, {"европа", "eu"}, {"евросоюз", "eu"},
}

// DetectCountry: сначала из флаг-эмодзи (любая страна), иначе — словарь (или "").
func DetectCountry(name string) string {
	if cc := ccFromFlag(name); cc != "" {
		return cc
	}
	n := strings.ToLower(name)
	for _, e := range countryKeywords {
		if strings.Contains(n, e.kw) {
			return e.cc
		}
	}
	return ""
}

// availableFlags — коды, под которые есть вшитый SVG (internal/web/assets/flags).
var availableFlags = map[string]bool{
	"ad": true, "ae": true, "af": true, "ag": true, "ai": true, "al": true, "am": true, "ao": true, "aq": true, "ar": true, "as": true, "at": true,
	"au": true, "aw": true, "ax": true, "az": true, "ba": true, "bb": true, "bd": true, "be": true, "bf": true, "bg": true, "bh": true, "bi": true,
	"bj": true, "bl": true, "bm": true, "bn": true, "bo": true, "bq": true, "br": true, "bs": true, "bt": true, "bv": true, "bw": true, "by": true,
	"bz": true, "ca": true, "cc": true, "cd": true, "cf": true, "cg": true, "ch": true, "ci": true, "ck": true, "cl": true, "cm": true, "cn": true,
	"co": true, "cp": true, "cr": true, "cu": true, "cv": true, "cw": true, "cx": true, "cy": true, "cz": true, "de": true, "dg": true, "dj": true,
	"dk": true, "dm": true, "do": true, "dz": true, "ec": true, "ee": true, "eg": true, "eh": true, "er": true, "es": true, "et": true, "eu": true,
	"fi": true, "fj": true, "fk": true, "fm": true, "fo": true, "fr": true, "ga": true, "gb": true, "gd": true, "ge": true, "gf": true, "gg": true,
	"gh": true, "gi": true, "gl": true, "gm": true, "gn": true, "gp": true, "gq": true, "gr": true, "gs": true, "gt": true, "gu": true, "gw": true,
	"gy": true, "hk": true, "hm": true, "hn": true, "hr": true, "ht": true, "hu": true, "ic": true, "id": true, "ie": true, "il": true, "im": true,
	"in": true, "io": true, "iq": true, "ir": true, "is": true, "it": true, "je": true, "jm": true, "jo": true, "jp": true, "ke": true, "kg": true,
	"kh": true, "ki": true, "km": true, "kn": true, "kp": true, "kr": true, "kw": true, "ky": true, "kz": true, "la": true, "lb": true, "lc": true,
	"li": true, "lk": true, "lr": true, "ls": true, "lt": true, "lu": true, "lv": true, "ly": true, "ma": true, "mc": true, "md": true, "me": true,
	"mf": true, "mg": true, "mh": true, "mk": true, "ml": true, "mm": true, "mn": true, "mo": true, "mp": true, "mq": true, "mr": true, "ms": true,
	"mt": true, "mu": true, "mv": true, "mw": true, "mx": true, "my": true, "mz": true, "na": true, "nc": true, "ne": true, "nf": true, "ng": true,
	"ni": true, "nl": true, "no": true, "np": true, "nr": true, "nu": true, "nz": true, "om": true, "pa": true, "pc": true, "pe": true, "pf": true,
	"pg": true, "ph": true, "pk": true, "pl": true, "pm": true, "pn": true, "pr": true, "ps": true, "pt": true, "pw": true, "py": true, "qa": true,
	"re": true, "ro": true, "rs": true, "ru": true, "rw": true, "sa": true, "sb": true, "sc": true, "sd": true, "se": true, "sg": true, "sh": true,
	"si": true, "sj": true, "sk": true, "sl": true, "sm": true, "sn": true, "so": true, "sr": true, "ss": true, "st": true, "sv": true, "sx": true,
	"sy": true, "sz": true, "tc": true, "td": true, "tf": true, "tg": true, "th": true, "tj": true, "tk": true, "tl": true, "tm": true, "tn": true,
	"to": true, "tr": true, "tt": true, "tv": true, "tw": true, "tz": true, "ua": true, "ug": true, "um": true, "un": true, "us": true, "uy": true,
	"uz": true, "va": true, "vc": true, "ve": true, "vg": true, "vi": true, "vn": true, "vu": true, "wf": true, "ws": true, "xk": true, "ye": true,
	"yt": true, "za": true, "zm": true, "zw": true,
}

// ccFromFlag достаёт ISO-код из флаг-эмодзи (пара regional indicator), но только
// если под него есть вшитый SVG; иначе "" (на странице будет флаг Земли xx.svg).
func ccFromFlag(name string) string {
	rs := []rune(name)
	for i := 0; i+1 < len(rs); i++ {
		a, b := rs[i], rs[i+1]
		if a >= 0x1F1E6 && a <= 0x1F1FF && b >= 0x1F1E6 && b <= 0x1F1FF {
			cc := string([]rune{a - 0x1F1E6 + 'a', b - 0x1F1E6 + 'a'})
			if availableFlags[cc] {
				return cc
			}
		}
	}
	return ""
}

var prefixRe = regexp.MustCompile(`^([A-Za-zА-Яа-яЁё]{2,3})[\s\-_|.]+(.+)$`)

// DisplayName убирает флаг-эмодзи, а короткий префикс — только если он
// дублирует определённую страну (cc); иначе префикс — часть имени сервера.
func DisplayName(name, cc string) string {
	if name == "" {
		return name
	}
	s := strings.Map(func(r rune) rune {
		if r >= 0x1F1E6 && r <= 0x1F1FF {
			return -1
		}
		return r
	}, name)
	s = strings.TrimSpace(s)
	if cc != "" {
		if m := prefixRe.FindStringSubmatch(s); m != nil {
			p := strings.ToLower(m[1])
			if p == cc || DetectCountry(p) == cc {
				s = strings.TrimSpace(m[2])
			}
		}
	}
	if s == "" {
		return name
	}
	return s
}
