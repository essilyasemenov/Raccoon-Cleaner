package pipeline

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	reHTTP            = regexp.MustCompile(`(?i)https?://`)
	reWWW             = regexp.MustCompile(`(?i)www\.[a-z0-9-]+`)
	reDomain          = regexp.MustCompile(`(?i)[a-z0-9-]+\.(?:` + tldAlternation + `)(?:/|\b)`)
	reEmail           = regexp.MustCompile(`(?i)^` + emailPattern + `$`)
	reEmailInPassword = regexp.MustCompile(`(?i)` + emailPattern)
)

const emailPattern = `[A-Z0-9!#$%&'*+/=?^_` + "`" + `{|}~-]+(?:\.[A-Z0-9!#$%&'*+/=?^_` + "`" + `{|}~-]+)*@(?:[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?\.)+[A-Z]{2,63}`

const tldAlternation = `com|org|net|info|online|ai|dev|gg|icu|xyz|io|co|me|app|site|club|top|pro|biz|tv|cc|` +
	`ru|ua|by|kz|su|edu|gov|mil|int|arpa|us|uk|de|fr|es|it|nl|pl|cz|sk|ch|at|be|se|no|fi|` +
	`dk|ie|pt|gr|tr|il|jp|kr|cn|in|br|au|nz|ca|mx|ar|cl|za|sg|hk|tw|th|vn|id|my|ph|` +
	`shop|store|blog|cloud|tech|space|fun|live|news|media|email|zip|mov|click|link|` +
	`page|pw|to|ws|fm|ly|gd|gl|tk|ml|ga|cf|gq|work|company|earth|world|today|agency`

// isFullLineURL is true when the entire trimmed line is a URL (§9.1).
// Multi-token lines like "http://x.com user@a.b p" are not full-line URLs.
func isFullLineURL(s string) bool {
	if strings.IndexFunc(s, unicode.IsSpace) >= 0 {
		return false
	}
	if _, ok := credentialPair(s); ok {
		return false
	}
	return ContainsURL(s)
}

// ContainsURL reports whether s looks like a URL (email-safe, F31).
func ContainsURL(s string) bool {
	if reHTTP.MatchString(s) {
		return true
	}
	if reWWW.MatchString(s) {
		return true
	}
	if isValidEmail(s) {
		at := strings.LastIndexByte(s, '@')
		return at > 0 && strings.ContainsAny(s[:at], `/\`)
	}
	locs := reDomain.FindAllStringIndex(s, -1)
	for _, loc := range locs {
		start := loc[0]
		if start > 0 {
			r, _ := utf8.DecodeLastRuneInString(s[:start])
			if r == '@' {
				continue // email local@domain
			}
		}
		return true
	}
	return false
}

func isValidEmail(s string) bool {
	if !strings.Contains(s, "@") || !reEmail.MatchString(s) {
		return false
	}
	return strings.LastIndexByte(s, '@') <= maxEmailLocal
}
