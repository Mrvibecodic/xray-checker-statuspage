// Package geo — определение страны по имени сервера и «чистое» отображаемое имя.
// Прямой перенос detect_country / display_name / RU_MONTHS из app.py.
package geo

import (
	"regexp"
	"strings"
)

// RUMonths — короткие русские месяцы (индекс 1..12), как в app.py.
var RUMonths = []string{"", "янв", "фев", "мар", "апр", "май", "июн",
	"июл", "авг", "сен", "окт", "ноя", "дек"}

type ccKW struct{ kw, cc string }

// countryKeywords — порядок важен: первое совпадение выигрывает (как в Python).
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
	// Европа/Евросоюз — в самом конце: конкретные страны/города выше выигрывают первыми.
	{"europe", "eu"}, {"европа", "eu"}, {"евросоюз", "eu"},
}

// DetectCountry возвращает ISO-код страны по эвристике из имени (или "").
func DetectCountry(name string) string {
	n := strings.ToLower(name)
	for _, e := range countryKeywords {
		if strings.Contains(n, e.kw) {
			return e.cc
		}
	}
	return ""
}

var prefixRe = regexp.MustCompile(`^[A-Za-zА-Яа-яЁё]{2,3}[\s\-_|.]+(.+)$`)

// DisplayName убирает флаг-эмодзи и короткий код-префикс (если страна найдена).
func DisplayName(name, cc string) string {
	if name == "" {
		return name
	}
	// Удаляем regional indicator symbols (флаги): U+1F1E6..U+1F1FF.
	s := strings.Map(func(r rune) rune {
		if r >= 0x1F1E6 && r <= 0x1F1FF {
			return -1
		}
		return r
	}, name)
	s = strings.TrimSpace(s)
	if cc != "" {
		if m := prefixRe.FindStringSubmatch(s); m != nil {
			s = strings.TrimSpace(m[1])
		}
	}
	if s == "" {
		return name
	}
	return s
}
