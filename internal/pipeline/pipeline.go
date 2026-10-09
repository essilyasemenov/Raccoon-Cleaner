package pipeline

import (
	"encoding/csv"
	"encoding/json"
	"path"
	"regexp"
	"strings"
	"unicode"
)

// Reason names (stable for summary).
const (
	ReasonEmpty          = "empty"
	ReasonEarlyURLFull   = "early_url_full"
	ReasonEarlyJWT       = "early_jwt"
	ReasonEarlyAndroid   = "early_android"
	ReasonEarlyBase64    = "early_base64"
	ReasonEarlyHex       = "early_hex"
	ReasonEarlyShortener = "early_shortener"
	ReasonOversizeLine   = "oversize_line"
	ReasonNoSeparator    = "no_separator"
	ReasonBrokenUTF8     = "broken_utf8_or_qqq"
	ReasonStars          = "stars"
	ReasonURLAfterColon  = "url_after_colon"
	ReasonInvalidPair    = "invalid_pair"
	ReasonDuplicate      = "duplicate"
	ReasonLoginEqPass    = "login_eq_pass"
	ReasonShortField     = "short_field"
	ReasonBanned         = "banned_substring"
	ReasonFilename       = "filename_field"
	ReasonJSON           = "json_field"
	ReasonLongBase64     = "long_base64_field"
	ReasonEmailPassword  = "email_in_password"
	ReasonHashUsername   = "hash_in_username"
	ReasonRepeatedRun    = "repeated_character_run"
	ReasonArtifactField  = "artifact_field"
	ReasonFinalURL       = "final_url_match"
	ReasonFinalInvalid   = "final_invalid_format"
	ReasonFinalForbidden = "final_forbidden_fragment"
)

const (
	maxRawLineLen       = 2048
	minFieldLen         = 5
	maxLoginLen         = 254
	maxEmailLocal       = 64
	minOpaquePassLen    = 30
	minArtifactLoginLen = 40
	minArtifactEmailLen = 65
)

var (
	reEmailPassExtract = regexp.MustCompile(`^((?:[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}):[^:]*):.*$`)
	reJWT              = regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]*\b`)
	reAndroid          = regexp.MustCompile(`(?i)android[_\- ]?id\s*[=:]\s*[A-Za-z0-9+/=_\-]{16,}`)
	reBase64Full       = regexp.MustCompile(`^[A-Za-z0-9+/=_\-]{40,}$`)
	reHexFull          = regexp.MustCompile(`^[A-Fa-f0-9]{80,}$`)
	reShortener        = regexp.MustCompile(`(?i)(bit\.ly|tinyurl|goo\.gl|t\.co|is\.gd|buff\.ly|ow\.ly|shorturl|adf\.ly|utm_source=|utm_medium=|utm_campaign=|fbclid=|gclid=)`)
	reUsername         = regexp.MustCompile(`(?i)^[A-Z0-9](?:[A-Z0-9._-]{0,62}[A-Z0-9])?$`)
	rePassword         = regexp.MustCompile(`^[!-9;-~]{1,128}$`)
	reBcrypt           = regexp.MustCompile(`(?i)^\$2[aby]\$(?:0[4-9]|[12][0-9]|3[01])\$[./a-z0-9]{53}$`)
	reMD5Crypt         = regexp.MustCompile(`^\$1\$[./A-Za-z0-9]{0,8}\$[./A-Za-z0-9]{22}$`)
	reAPR1             = regexp.MustCompile(`^\$apr1\$[./A-Za-z0-9]{0,8}\$[./A-Za-z0-9]{22}$`)
	reSHA256Crypt      = regexp.MustCompile(`^\$5\$(?:rounds=[1-9][0-9]{3,8}\$)?[./A-Za-z0-9]{1,16}\$[./A-Za-z0-9]{43}$`)
	reSHA512Crypt      = regexp.MustCompile(`(?i)^\$6\$(?:rounds=[1-9][0-9]{3,8}\$)?[./a-z0-9]{1,16}\$[./a-z0-9]{86}$`)
	reArgon2           = regexp.MustCompile(`(?i)^\$argon2(?:id|i|d)\$(?:v=[0-9]+\$)?m=[0-9]+,t=[0-9]+,p=[0-9]+\$[a-z0-9+/]+={0,2}\$[a-z0-9+/]+={0,2}$`)
	rePHPass           = regexp.MustCompile(`^\$[PH]\$[./0-9A-Za-z]{31}$`)
	reHexDigest        = regexp.MustCompile(`(?i)^[0-9a-f]+$`)
	reMySQL5           = regexp.MustCompile(`^\*[0-9A-Fa-f]{40}$`)
	reColonMD5         = regexp.MustCompile(`(?i)^[0-9a-f]{32}:(?:[a-z0-9]{2}|[a-z0-9]{8}|[a-z0-9]{16,32}|[!-9;-~]{5}|[0-9]{32}:[0-9a-f]{2})$`)
	reDjangoSHA1       = regexp.MustCompile(`(?i)^sha1\$[^$:\r\n]{0,32}\$[0-9a-f]{40}$`)
	reDjangoSHA256     = regexp.MustCompile(`(?i)^sha256\$[^$:\r\n]{0,32}\$[0-9a-f]{64}$`)
	reDjangoSHA384     = regexp.MustCompile(`(?i)^sha384\$[^$:\r\n]{0,32}\$[0-9a-f]{96}$`)
	reSHA1Crypt        = regexp.MustCompile(`^\$4\$[./A-Za-z0-9]{8}\$[./A-Za-z0-9]+$`)
	reDrupalSHA512     = regexp.MustCompile(`^\$S\$[./A-Za-z0-9]{52}$`)
	reHEXWrapper       = regexp.MustCompile(`^\$HEX\[[0-9A-Fa-f]+\]$`)
	reSHAWrapper       = regexp.MustCompile(`^\$SHA\[[A-Za-z0-9+/=]+\]$`)
	reMD5Date          = regexp.MustCompile(`(?i)^([0-9a-f]{32})[a-z0-9]{8}_([0-9]{2}\.[0-9]{2}\.[0-9]{4})$`)
	reUUIDFragment     = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	reAppProfile       = regexp.MustCompile(`^[A-Za-z0-9._-]+_\[[A-Za-z0-9._-]+\]_[A-Za-z0-9._-]+$`)
	reSoftwareProfile  = regexp.MustCompile(`(?i)^(?:[a-z0-9._-]+\((?:profile|default)[a-z0-9._-]*\)|[a-z0-9]+software_[a-z0-9]+(?:stable|profile[0-9]*))$`)
	rePathArtifact     = regexp.MustCompile(`^[A-Za-z0-9._$-]+(?:/[A-Za-z0-9._$-]+){2,}/?$`)
	reNumericToken     = regexp.MustCompile(`^[0-9]+/[A-Za-z0-9_-]{20,}$`)
	reAssignmentPath   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*=[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+){2,}/[A-Za-z0-9._/-]+$`)
	reEncodedMarker    = regexp.MustCompile(`(?i)^\{[a-z][a-z0-9_-]{1,19}\}[_:][a-z0-9+/_=-]{8,}$`)
	reTemplateAssign   = regexp.MustCompile(`(?i)^[a-z0-9._-]+=\([a-z0-9._-]{2,}\)[^a-z0-9]{2,}$`)
	reFinalURL         = regexp.MustCompile(`(?i)^[^:]+:.+?(?:https?://|www\.[a-z0-9-]+|[a-z0-9-]+\.(?:` + finalURLTLDs + `)(?:/|\b)).*$`)
)

const finalURLTLDs = `com|org|net|edu|gov|co|uk|ru|fr|de|jp|cn|io|me|tv|ua|pl|ro|it|es|nl|be|se|no|fi|dk|ch|at|in|br|mx|ar|pe|cl|za|ng|ke|eg|sa|ae|il|tr|ir|pk|bd|vn|th|my|ph|au|nz|ca|bg|biz|info|site|online|xyz|top|club|shop|app|dev|pro|live|tech|store|blog|cloud|ai|gg|ly|to|cc|link|click|work|space|website|fun|icu|vip|wang|xin|ren|asia|mobi|name|tel|travel|jobs|cat|museum|aero|coop|int|mil|arpa`

// "not saved" (F9) becomes "not:saved" after P4 space→colon; keep both forms.
var banWords = []string{"admin", "unknown", "test", "not_saved", "not saved", "notsaved", "not:saved"}

// Stats accumulates per-file pipeline counters.
type Stats struct {
	Kept       int
	Invalid    int
	Duplicates int
	Reasons    map[string]int
}

func (s *Stats) drop(reason string) {
	s.Invalid++
	s.Reasons[reason]++
}

// Processor runs the line pipeline with per-file dedupe.
type Processor struct {
	// ponytail: dedupe is in memory; use a disk-backed set only if unique output exhausts RAM.
	seen  map[string]struct{}
	Stats *Stats
}

func NewProcessor() *Processor {
	return &Processor{
		seen:  make(map[string]struct{}),
		Stats: &Stats{Reasons: make(map[string]int)},
	}
}

// ProcessLine returns kept line or ("", false) if dropped.
func (p *Processor) ProcessLine(line string) (string, bool) {
	// P0
	line = strings.TrimSpace(line)
	if line == "" {
		p.Stats.drop(ReasonEmpty)
		return "", false
	}
	// ponytail: 2 KiB preserves normal URL-prefixed records; raise it if real inputs exceed that bound.
	if len(line) > maxRawLineLen {
		p.Stats.drop(ReasonOversizeLine)
		return "", false
	}

	// P1 early junk
	if reason := earlyJunk(line); reason != "" {
		p.Stats.drop(reason)
		return "", false
	}
	if pair, ok := credentialPair(line); ok {
		line = pair
	}

	// P2 extract
	line = extractEmailPass(line)

	// P3: spaces around colon + unicode space normalize start
	line = normalizeSpacesToASCII(line)
	line = collapseColonSpaces(line)

	// P4: Fields join with :
	if strings.Contains(line, " ") {
		fields := strings.Fields(line)
		line = strings.Join(fields, ":")
	}

	// P5 extract again
	line = extractEmailPass(line)

	// P6 last two non-empty :
	login, pass, ok := lastPair(line)
	if !ok {
		p.Stats.drop(ReasonNoSeparator)
		return "", false
	}
	line = login + ":" + pass

	// P8 ??? / U+FFFD / **
	if strings.Contains(line, "\uFFFD") || strings.Contains(line, "???") {
		p.Stats.drop(ReasonBrokenUTF8)
		return "", false
	}
	if strings.Contains(line, "**") {
		p.Stats.drop(ReasonStars)
		return "", false
	}

	if match := reMD5Date.FindStringSubmatch(pass); match != nil {
		pass = match[1]
		line = login + ":" + pass
	}
	if reColonMD5.MatchString(pass) {
		pass, _, _ = strings.Cut(pass, ":")
		line = login + ":" + pass
	}
	knownHash := isAllowedHash(pass)

	// P9 URL after first :
	if !knownHash && ContainsURL(pass) {
		p.Stats.drop(ReasonURLAfterColon)
		return "", false
	}

	// P10 validate pair
	if !validPair(login, pass) {
		p.Stats.drop(ReasonInvalidPair)
		return "", false
	}

	if reason := junkFieldReason(login, pass); reason != "" {
		p.Stats.drop(reason)
		return "", false
	}

	// P13 " → ""
	login = strings.ReplaceAll(login, `"`, `""`)
	pass = strings.ReplaceAll(pass, `"`, `""`)

	// P14 login == pass
	if strings.EqualFold(login, pass) {
		p.Stats.drop(ReasonLoginEqPass)
		return "", false
	}

	// P15 minimum length after quotes (F30) — byte len; password alphabet is ASCII
	if len(login) < minFieldLen || len(pass) < minFieldLen {
		p.Stats.drop(ReasonShortField)
		return "", false
	}

	// P16 ban
	if (knownHash && banned(login)) || (!knownHash && banned(line)) {
		p.Stats.drop(ReasonBanned)
		return "", false
	}

	line = login + ":" + pass
	if reason := finalOutputReason(line); reason != "" {
		p.Stats.drop(reason)
		return "", false
	}

	// P17 dedupe
	key := strings.ToLower(line)
	if _, exists := p.seen[key]; exists {
		p.Stats.Duplicates++
		p.Stats.Reasons[ReasonDuplicate]++
		return "", false
	}
	p.seen[key] = struct{}{}

	p.Stats.Kept++
	return `"` + login + `","` + pass + `"`, true
}

func earlyJunk(line string) string {
	if isFullLineURL(line) {
		return ReasonEarlyURLFull
	}
	subject := line
	if pair, ok := credentialPair(line); ok {
		subject = pair
		_, pass, _ := strings.Cut(pair, ":")
		if isAllowedHash(pass) {
			return ""
		}
	}
	if reJWT.MatchString(subject) {
		return ReasonEarlyJWT
	}
	if reAndroid.MatchString(subject) {
		return ReasonEarlyAndroid
	}
	if reBase64Full.MatchString(subject) {
		return ReasonEarlyBase64
	}
	if reHexFull.MatchString(subject) {
		return ReasonEarlyHex
	}
	if reShortener.MatchString(subject) {
		return ReasonEarlyShortener
	}
	return ""
}

func extractEmailPass(line string) string {
	if _, _, ok := knownHashPair(line); ok {
		return line
	}
	m := reEmailPassExtract.FindStringSubmatch(line)
	if len(m) >= 2 {
		return m[1]
	}
	return line
}

func normalizeSpacesToASCII(line string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, line)
}

func collapseColonSpaces(line string) string {
	for {
		next := strings.ReplaceAll(line, " :", ":")
		next = strings.ReplaceAll(next, ": ", ":")
		if next == line {
			return line
		}
		line = next
	}
}

func lastPair(line string) (login, pass string, ok bool) {
	if login, pass, ok := knownHashPair(line); ok {
		return login, pass, true
	}
	for i := len(line); i > 0; {
		j := strings.LastIndexByte(line[:i], ':')
		part := line[j+1 : i]
		if part != "" {
			if pass == "" {
				pass = part
			} else {
				return part, pass, true
			}
		}
		if j < 0 {
			break
		}
		i = j
	}
	return "", "", false
}

func knownHashPair(line string) (login, pass string, ok bool) {
	for end := len(line); end > 0; {
		i := strings.LastIndexByte(line[:end], ':')
		if i < 0 {
			break
		}
		candidatePass := line[i+1:]
		prefix := line[:i]
		j := strings.LastIndexByte(prefix, ':')
		candidateLogin := prefix[j+1:]
		if isAllowedHash(candidatePass) && validPair(candidateLogin, candidatePass) {
			login, pass, ok = candidateLogin, candidatePass, true
		}
		end = i
	}
	return login, pass, ok
}

func credentialPair(line string) (string, bool) {
	line = strings.TrimSpace(line)
	for _, delimiter := range []rune{',', '\t', '|', ';'} {
		if !strings.ContainsRune(line, delimiter) {
			continue
		}
		reader := csv.NewReader(strings.NewReader(line))
		reader.Comma = delimiter
		reader.FieldsPerRecord = -1
		reader.TrimLeadingSpace = true
		reader.LazyQuotes = true
		if parts, err := reader.Read(); err == nil {
			if pair, ok := separatedPair(parts, string(delimiter)); ok {
				return pair, true
			}
		}
	}
	if pair, ok := separatedPair(strings.Fields(line), " "); ok {
		return pair, true
	}

	line = collapseColonSpaces(normalizeSpacesToASCII(line))
	if login, pass, ok := knownHashPair(line); ok {
		return login + ":" + pass, true
	}
	for end := len(line); end > 0; {
		i := strings.LastIndexByte(line[:end], ':')
		if i < 0 {
			break
		}
		if isURLField(line[i+1:]) {
			if login, pass, ok := lastPair(line[:i]); ok && validPair(login, pass) {
				return login + ":" + pass, true
			}
		}
		end = i
	}
	login, pass, ok := lastPair(line)
	if !ok || !validPair(login, pass) {
		return "", false
	}
	return login + ":" + pass, true
}

func separatedPair(parts []string, separator string) (string, bool) {
	if len(parts) < 2 {
		return "", false
	}
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if len(parts) == 2 {
		if validPair(parts[0], parts[1]) {
			return parts[0] + ":" + parts[1], true
		}
		return "", false
	}

	if validPair(parts[0], parts[1]) && isURLField(strings.Join(parts[2:], separator)) {
		return parts[0] + ":" + parts[1], true
	}
	last := len(parts) - 1
	if validPair(parts[last-1], parts[last]) && isURLField(strings.Join(parts[:last-1], separator)) {
		return parts[last-1] + ":" + parts[last], true
	}
	return "", false
}

func isURLField(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "//") {
		return false
	}
	lower := strings.ToLower(s)
	if strings.Contains(lower, "://") || strings.HasPrefix(lower, "android:") {
		return true
	}
	if ContainsURL(s) {
		return true
	}
	host, _, _ := strings.Cut(s, "/")
	if strings.Contains(host, "@") {
		return false
	}
	if name, port, found := strings.Cut(host, ":"); found {
		if port == "" || strings.IndexFunc(port, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return false
		}
		host = name
	}
	return strings.Contains(host, ".") && !strings.ContainsAny(host, " \t")
}

func validPair(login, pass string) bool {
	if len(login) < 1 || len(login) > maxLoginLen {
		return false
	}
	if strings.Contains(login, "@") {
		if !isValidEmail(login) {
			return false
		}
	} else if !reUsername.MatchString(login) {
		return false
	}
	return isAllowedHash(pass) || rePassword.MatchString(pass)
}

func finalOutputReason(line string) string {
	// RE2 has no negative lookahead; invert the requested positive pair validation instead.
	login, pass, ok := strings.Cut(line, ":")
	if !ok || !validPair(login, pass) {
		return ReasonFinalInvalid
	}
	knownHash := isAllowedHash(pass)
	if !knownHash && reFinalURL.MatchString(line) {
		return ReasonFinalURL
	}
	if len(pass) >= minOpaquePassLen && !knownHash {
		return ReasonArtifactField
	}
	lowerLogin := strings.ToLower(login)
	lowerPass := strings.ToLower(pass)
	if strings.Contains(login, "==@") || strings.Contains(lowerLogin, "android") || strings.Contains(lowerLogin, "discord") ||
		(!knownHash && (strings.Contains(pass, "==@") || strings.Contains(lowerPass, "android") || strings.Contains(lowerPass, "discord"))) {
		return ReasonFinalForbidden
	}
	if strings.Contains(lowerLogin, "bot") || strings.Contains(login, "//") ||
		(!knownHash && (strings.Contains(lowerPass, "bot") || strings.Contains(pass, "//"))) {
		return ReasonFinalForbidden
	}
	return ""
}

func junkFieldReason(login, pass string) string {
	knownHash := isAllowedHash(pass)
	if hasHashFragment(login) {
		return ReasonHashUsername
	}
	if !knownHash && reEmailInPassword.MatchString(pass) {
		return ReasonEmailPassword
	}
	if looksLikeFilename(login) || (!knownHash && looksLikeFilename(pass)) {
		return ReasonFilename
	}
	if looksLikeJSON(login) || (!knownHash && looksLikeJSON(pass)) {
		return ReasonJSON
	}
	if (len(login) >= 48 && reBase64Full.MatchString(login)) ||
		(!knownHash && len(pass) >= 48 && reBase64Full.MatchString(pass)) {
		return ReasonLongBase64
	}
	if looksLikeArtifact(login, pass, knownHash) {
		return ReasonArtifactField
	}
	if hasRepeatedRun(login, 6) || (!knownHash && hasRepeatedRun(pass, 6)) {
		return ReasonRepeatedRun
	}
	if hasRepeatedBlock(login) || (!knownHash && hasRepeatedBlock(pass)) {
		return ReasonArtifactField
	}
	return ""
}

func isAllowedHash(pass string) bool {
	if reHexDigest.MatchString(pass) {
		switch len(pass) {
		case 32, 40, 48, 51, 56, 64, 96, 128:
			return true
		}
	}
	if rePHPass.MatchString(pass) {
		cost := strings.IndexByte("./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz", pass[3])
		return cost >= 7 && cost <= 30
	}
	return reBcrypt.MatchString(pass) || reMD5Crypt.MatchString(pass) || reAPR1.MatchString(pass) ||
		reSHA256Crypt.MatchString(pass) || reSHA512Crypt.MatchString(pass) || reArgon2.MatchString(pass) ||
		reMySQL5.MatchString(pass) || reColonMD5.MatchString(pass) || reDjangoSHA1.MatchString(pass) ||
		reDjangoSHA256.MatchString(pass) || reDjangoSHA384.MatchString(pass) || reSHA1Crypt.MatchString(pass) ||
		reDrupalSHA512.MatchString(pass) || reHEXWrapper.MatchString(pass) || reSHAWrapper.MatchString(pass)
}

func hasHashFragment(s string) bool {
	for _, size := range []int{64, 40, 32} {
		for start := 0; start+size <= len(s); start++ {
			if looksLikeHashUsername(s[start : start+size]) {
				return true
			}
		}
	}
	return false
}

func looksLikeArtifact(login, pass string, knownHash bool) bool {
	lowerLogin := strings.ToLower(login)
	lowerPass := strings.ToLower(pass)
	if lowerLogin == "login" || lowerLogin == "username" || lowerLogin == "password" {
		return true
	}
	// ponytail: 40/65-byte login limits trade RFC maxima for metadata filtering; raise only with verified false positives.
	if len(login) >= minArtifactEmailLen || (!isValidEmail(login) && len(login) >= minArtifactLoginLen) || ContainsURL(login) ||
		reUUIDFragment.MatchString(login) || reUUIDFragment.MatchString(pass) {
		return true
	}
	if strings.Contains(lowerLogin, "enc893*") || strings.Contains(lowerLogin, "csco_webvpn_") {
		return true
	}
	if !knownHash && (strings.HasPrefix(pass, "[") || reAppProfile.MatchString(pass) || reSoftwareProfile.MatchString(pass) ||
		(len(pass) >= 20 && rePathArtifact.MatchString(pass)) || reNumericToken.MatchString(pass) ||
		reAssignmentPath.MatchString(pass) || reEncodedMarker.MatchString(pass) || reTemplateAssign.MatchString(pass) ||
		strings.Contains(lowerPass, "enc893*") || strings.Contains(lowerPass, "csco_webvpn_") ||
		strings.HasPrefix(lowerPass, "v1.") || strings.HasPrefix(lowerPass, "pbkdf2_") ||
		strings.HasPrefix(lowerPass, "argon2") || strings.HasPrefix(lowerPass, "scrypt$") ||
		strings.Contains(lowerPass, "sociallogin") || strings.Contains(pass, `\`) || strings.HasPrefix(pass, "/") ||
		(strings.Count(pass, "=") >= 2 && strings.Contains(pass, "&"))) {
		return true
	}
	return false
}

func hasRepeatedBlock(s string) bool {
	for size := 2; size <= 16 && size*3 <= len(s); size++ {
		for start := 0; start+size*3 <= len(s); start++ {
			block := s[start : start+size]
			if s[start+size:start+size*2] == block && s[start+size*2:start+size*3] == block {
				return true
			}
		}
	}
	return false
}

func looksLikeHashUsername(s string) bool {
	hasHexLetter := false
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
			hasHexLetter = true
		case r >= 'A' && r <= 'F':
			hasHexLetter = true
		default:
			return false
		}
	}
	return hasHexLetter
}

func looksLikeFilename(s string) bool {
	ext := strings.ToLower(path.Ext(strings.Trim(s, `"'`)))
	switch ext {
	case ".local", ".pdf", ".json", ".txt", ".csv", ".xml", ".html", ".htm",
		".doc", ".docx", ".xls", ".xlsx", ".xlsm", ".xlsb", ".ppt", ".pptx", ".zip", ".rar",
		".7z", ".tar", ".gz", ".jpg", ".jpeg", ".png", ".gif", ".svg",
		".exe", ".dll", ".log", ".sql", ".db", ".bak", ".tmp", ".ini",
		".yaml", ".yml", ".env", ".js", ".css", ".php", ".py", ".go",
		".java", ".class", ".jar", ".sh", ".bat", ".cmd", ".ps1", ".md",
		".conf", ".config", ".key", ".pem", ".crt", ".cer", ".pfx", ".p12", ".req", ".csr",
		".sqlite", ".sqlite3", ".database", ".mdb", ".accdb":
		return true
	default:
		return false
	}
}

func looksLikeJSON(s string) bool {
	if len(s) < 2 || (s[0] != '{' && s[0] != '[') {
		return false
	}
	return json.Valid([]byte(s))
}

func hasRepeatedRun(s string, limit int) bool {
	run := 1
	for i := 1; i < len(s); i++ {
		if s[i] == s[i-1] {
			run++
			if run >= limit {
				return true
			}
		} else {
			run = 1
		}
	}
	return false
}

func banned(line string) bool {
	s := strings.ToLower(line)
	for _, b := range banWords {
		if strings.Contains(s, b) {
			return true
		}
	}
	return false
}
