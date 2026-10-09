package pipeline

import (
	"strings"
	"testing"
)

func TestPipelineKeepCases(t *testing.T) {
	bcryptPassword := "$2B$12$" + strings.Repeat("A1", 26) + "A"
	sha512CryptPassword := "$6$salt1234$" + strings.Repeat("A1", 43)
	argon2Password := "$argon2id$v=19$m=65536,t=3,p=4$c2FsdHNhbHQ$aGFzaFZhbHVl"
	maxEmail := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ@example.com"
	cases := []struct {
		in, want string
	}{
		{"user@mail.com:pass123", "user@mail.com:pass123"},
		{"prefix:user@mail.com:pass123", "user@mail.com:pass123"},
		{"prefix user@mail.com:pass123", "user@mail.com:pass123"},
		{"user5 pass1234", "user5:pass1234"},
		{`user5:pa"ss12`, `user5:pa""ss12`},
		{"good@x.com:aa??bb", "good@x.com:aa??bb"},
		{"user@mail.com:secret99", "user@mail.com:secret99"},
		{"user5:pass5", "user5:pass5"},
		{"username1:pa@ssword", "username1:pa@ssword"},
		{"username1:pa/ssword", "username1:pa/ssword"},
		{"username1:my/secret/pass", "username1:my/secret/pass"},
		{"username1:ab=cd/ef", "username1:ab=cd/ef"},
		{"username1:my[secret", "username1:my[secret"},
		{"username1:NormalSoftware_Ordinary", "username1:NormalSoftware_Ordinary"},
		{"readerApplication:normalPass1", "readerApplication:normalPass1"},
		{"john.smith:normalPass1", "john.smith:normalPass1"},
		{"very.long.email.address.for.user@example.com:normalPass1", "very.long.email.address.for.user@example.com:normalPass1"},
		{maxEmail + ":normalPass1", maxEmail + ":normalPass1"},
		{"username1:aaaaaB", "username1:aaaaaB"},
		{"username1:report.pdf2", "username1:report.pdf2"},
		{"username1:5d41402abc4b2a76b9719d911017c592", "username1:5d41402abc4b2a76b9719d911017c592"},
		{"username1:" + bcryptPassword, "username1:" + bcryptPassword},
		{"username1:" + sha512CryptPassword, "username1:" + sha512CryptPassword},
		{"username1:" + argon2Password, "username1:" + argon2Password},
	}
	for _, tc := range cases {
		p := NewProcessor()
		out, ok := p.ProcessLine(tc.in)
		want := csvPair(tc.want)
		if !ok || out != want {
			t.Fatalf("in=%q: got ok=%v out=%q want %q", tc.in, ok, out, want)
		}
	}
}

func csvPair(pair string) string {
	login, pass, _ := strings.Cut(pair, ":")
	return `"` + login + `","` + pass + `"`
}

func TestPipelineDelimitedPairsAreIdempotent(t *testing.T) {
	bcryptPassword := "$2y$10$IeRPhQy8Z0rIr86sEXsPLufN3vluLJg52z5OWP8/oOHv9PTg.3JI6"
	colonHash := "5d41402abc4b2a76b9719d911017c592:abcd1234"
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "quoted comma", input: `"user123","pass1234"`, want: `"user123","pass1234"`},
		{name: "quoted comma in password", input: `"user123","pa,ss""word"`, want: `"user123","pa,ss""word"`},
		{name: "quoted semicolon", input: `"user123";"pass1234"`, want: `"user123","pass1234"`},
		{name: "pipe", input: `user123|pass1234`, want: `"user123","pass1234"`},
		{name: "tab", input: "user123\tpass1234", want: `"user123","pass1234"`},
		{name: "bcrypt", input: `"user123","` + bcryptPassword + `"`, want: `"user123","` + bcryptPassword + `"`},
		{name: "colon hash", input: "user123:" + colonHash, want: `"user123","5d41402abc4b2a76b9719d911017c592"`},
		{name: "CSV colon hash", input: `"user123","` + colonHash + `"`, want: `"user123","5d41402abc4b2a76b9719d911017c592"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			first, ok := NewProcessor().ProcessLine(tc.input)
			if !ok || first != tc.want {
				t.Fatalf("first pass: got ok=%v out=%q want %q", ok, first, tc.want)
			}
			second, ok := NewProcessor().ProcessLine(first)
			if !ok || second != first {
				t.Fatalf("second pass: got ok=%v out=%q want unchanged %q", ok, second, first)
			}
		})
	}
}

func TestPipelineOpaquePasswordBoundary(t *testing.T) {
	const allowed = "abcdefghijklmnopqrstuvwxyzABC"
	const rejected = allowed + "D"

	if out, ok := NewProcessor().ProcessLine(`"user123","` + allowed + `"`); !ok {
		t.Fatalf("29-byte password was dropped: %q", out)
	}
	p := NewProcessor()
	if out, ok := p.ProcessLine(`"user123","` + rejected + `"`); ok {
		t.Fatalf("30-byte opaque password was kept: %q", out)
	}
	if got := p.Stats.Reasons[ReasonArtifactField]; got != 1 {
		t.Fatalf("artifact reason count=%d; all reasons=%v", got, p.Stats.Reasons)
	}
}

func TestPipelineKeepsKnownHashes(t *testing.T) {
	hex := "0123456789abcdef"
	cases := []struct {
		name string
		pass string
	}{
		{"MD5 or NTLM", strings.Repeat(hex, 2)},
		{"SHA1", strings.Repeat(hex, 2) + hex[:8]},
		{"SHA1 Oracle", strings.Repeat(hex, 3)},
		{"SHA224", strings.Repeat(hex, 3) + hex[:8]},
		{"SHA256", strings.Repeat(hex, 4)},
		{"SHA384", strings.Repeat(hex, 6)},
		{"SHA512", strings.Repeat(hex, 8)},
		{"MD5 Palshop", strings.Repeat(hex, 3) + hex[:3]},
		{"MySQL5", "*" + strings.Repeat(hex, 2) + hex[:8]},
		{"MD5 APR", "$apr1$salt1234$0123456789abcdefABCDEF"},
		{"MD5 MyBB", strings.Repeat(hex, 2) + ":a1b2c3d4"},
		{"MD5 Joomla", strings.Repeat(hex, 2) + ":abcdefghijklmnop"},
		{"MD5 osCommerce", strings.Repeat(hex, 2) + ":ab"},
		{"MD5 IP.Board", strings.Repeat(hex, 2) + ":abcde"},
		{"MD5 Chap", strings.Repeat(hex, 2) + ":" + strings.Repeat("12345678", 4) + ":ab"},
		{"bcrypt with shortener fragment", "$2y$10$t.co" + strings.Repeat("A", 49)},
		{"PHPass", "$P$Babcdefghijklmnopqrstuvwx123456"},
		{"PHPass H", "$H$Babcdefghijklmnopqrstuvwx123456"},
		{"MD5 crypt", "$1$salt1234$0123456789abcdefABCDEF"},
		{"SHA256 crypt", "$5$salt1234$" + strings.Repeat("Abcde123/", 4) + "Abcde12"},
		{"SHA1 Django", "sha1$salt1234$" + strings.Repeat(hex, 2) + hex[:8]},
		{"SHA256 Django", "sha256$salt1234$" + strings.Repeat(hex, 4)},
		{"SHA384 Django", "sha384$salt1234$" + strings.Repeat(hex, 6)},
		{"SHA1 crypt", "$4$salt1234$abcdefghijklmnop"},
		{"SHA512 Drupal", "$S$" + strings.Repeat("Abcde123/", 5) + "Abcde12"},
		{"HEX wrapper", "$HEX[0123456789abcdef]"},
		{"SHA wrapper", "$SHA[0123456789abcdef]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !isAllowedHash(tc.pass) {
				t.Fatal("expected recognized hash format")
			}
			wantPass := tc.pass
			if reColonMD5.MatchString(wantPass) {
				wantPass, _, _ = strings.Cut(wantPass, ":")
			}
			want := csvPair("username1:" + wantPass)
			p := NewProcessor()
			got, ok := p.ProcessLine("username1:" + tc.pass)
			if !ok || got != want {
				t.Fatalf("got ok=%v out=%q want %q reasons=%v", ok, got, want, p.Stats.Reasons)
			}
			again, ok := NewProcessor().ProcessLine(got)
			if !ok || again != got {
				t.Fatalf("second pass: got ok=%v out=%q want unchanged", ok, again)
			}
		})
	}
}

func TestPipelineDropsMalformedHashLikeValues(t *testing.T) {
	cases := []string{
		strings.Repeat("0123456789abcdef", 2) + "a",
		"$P$Babcdefghijklmnopqrstuvwx12345",
		"$1$salt1234$0123456789abcdefABCDE",
		"pbkdf2_sha256$22222$salt1234$abcd",
		"pbkdf2_sha256$22222$qPXBQ6GfIOaZ$YLUtBPuYtCikZgym3C8UN5UX2E/gOCq5KvhDzrs5Cuc=",
		"$y$j9T$salt1234$" + strings.Repeat("Abcde123/", 4) + "Abcde12",
	}
	for _, pass := range cases {
		t.Run(pass[:min(len(pass), 12)], func(t *testing.T) {
			if out, ok := NewProcessor().ProcessLine("username1:" + pass); ok {
				t.Fatalf("malformed hash-like value was kept: %q", out)
			}
		})
	}
}

func TestPipelineFinalGuards(t *testing.T) {
	const chars = "!\"#$%'()*+,-./0123456789;<>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[]^_`abcdefghijklmnopqrstuvwxyz{|}~"
	quotedBoundaryPass := chars + chars[:128-len(chars)]
	cases := []struct {
		name   string
		input  string
		reason string
	}{
		{
			name:   "URL with final-only TLD",
			input:  "username1:prefix.PORTAL.WEBSITE/path",
			reason: ReasonFinalURL,
		},
		{
			name:   "invalid after quote expansion",
			input:  "username1:" + quotedBoundaryPass,
			reason: ReasonFinalInvalid,
		},
		{name: "literal marker", input: "username1:abc==@xyz", reason: ReasonFinalForbidden},
		{name: "Android case insensitive", input: "username1:myANDROIDpass", reason: ReasonFinalForbidden},
		{name: "Discord in username", input: "discordUser:pass1234", reason: ReasonFinalForbidden},
		{name: "bot in username", input: "robotics123:normalPass1", reason: ReasonFinalForbidden},
		{name: "bot in password", input: "reader123:myBoTpass1", reason: ReasonFinalForbidden},
		{name: "double slash in password", input: "reader123:pa//ssword", reason: ReasonFinalForbidden},
		{name: "bot in login with known hash", input: "robotics123:$2y$10$" + strings.Repeat("A", 53), reason: ReasonFinalForbidden},
		{name: "double slash in login with known hash", input: "reader//123:$2y$10$" + strings.Repeat("A", 53), reason: ReasonInvalidPair},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewProcessor()
			if out, ok := p.ProcessLine(tc.input); ok {
				t.Fatalf("expected drop, got %q", out)
			}
			if got := p.Stats.Reasons[tc.reason]; got != 1 {
				t.Fatalf("reason %q count=%d; all reasons=%v", tc.reason, got, p.Stats.Reasons)
			}
		})
	}
}

func TestPipelineFinalGuardsPreserveKnownHashes(t *testing.T) {
	cases := []struct {
		name, pass string
	}{
		{"bot in bcrypt", "$2y$10$" + strings.Repeat("A", 20) + "BoT" + strings.Repeat("A", 30)},
		{"double slash in SHA-crypt", "$6$salt1234$" + strings.Repeat("A", 40) + "//" + strings.Repeat("A", 44)},
		{"bot in Argon2", "$argon2id$v=19$m=65536,t=3,p=4$c2FsdHNhbHQ$botAAA"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewProcessor()
			want := csvPair("reader123:" + tc.pass)
			if out, ok := p.ProcessLine("reader123:" + tc.pass); !ok || out != want {
				t.Fatalf("got ok=%v out=%q want %q reasons=%v", ok, out, want, p.Stats.Reasons)
			}
		})
	}
}

func TestPipelineDropsJunkFields(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		reason string
	}{
		{name: "local filename in username", input: "config.local:pass1234", reason: ReasonFilename},
		{name: "PDF filename in password", input: "username1:report.pdf", reason: ReasonFilename},
		{name: "JSON array in password", input: `username1:["alpha","beta"]`, reason: ReasonJSON},
		{name: "long base64 in password", input: "username1:" + strings.Repeat("QUJD", 16), reason: ReasonLongBase64},
		{name: "email in password", input: "username1:other@example.com", reason: ReasonEmailPassword},
		{name: "login email fused with year in password", input: `"zita.brussels@gmail.com","zita.brussels@gmail.com2019"`, reason: ReasonEmailPassword},
		{name: "other email fused with digits in password", input: "username1:other@example.com2019", reason: ReasonEmailPassword},
		{name: "email embedded after prefix in password", input: "username1:xxother@example.com!", reason: ReasonEmailPassword},
		{name: "hex hash in username", input: "5d41402abc4b2a76b9719d911017c592:pass1234", reason: ReasonHashUsername},
		{name: "embedded hash in username", input: "BCad41b451ac6166f1168c98a996fc616e.h894ohaw:pass1234", reason: ReasonHashUsername},
		{name: "short username", input: "user:pass1234", reason: ReasonShortField},
		{name: "short password", input: "username1:pass", reason: ReasonShortField},
		{name: "repeated username character", input: "aaaaaa:pass1234", reason: ReasonRepeatedRun},
		{name: "repeated password character", input: "username1:passaaaaaa", reason: ReasonRepeatedRun},
		{name: "encrypted artifact", input: "username1:ENC893*_opaque-value", reason: ReasonArtifactField},
		{name: "Cisco placeholder", input: "CSCO_WEBVPN_USERZ123:CSCO_WEBVPN_PASSWORDZ123", reason: ReasonArtifactField},
		{name: "serialized hash", input: "username1:pbkdf2_sha256$22222$salt$digest", reason: ReasonArtifactField},
		{name: "versioned token", input: "username1:v1.OVCAG7wQI2GxJhRMbibnC7oST2F62nTJ9Yy47sw", reason: ReasonArtifactField},
		{name: "domain as username", input: "portal.example.com:2083/cpsess123/index.cgi", reason: ReasonArtifactField},
		{name: "UUID in password", input: "username1:{9ea5fd70-970e-4087-9a5f-9c1a6b7d3be8}", reason: ReasonArtifactField},
		{name: "file path in password", input: `username1:\Steam\bin\private.key`, reason: ReasonFilename},
		{name: "query string in password", input: "username1:realm=true&lang=en&track=id", reason: ReasonArtifactField},
		{name: "generic username", input: "password:ordinary-secret", reason: ReasonArtifactField},
		{name: "repeated block", input: "username1:abcabcabcXYZ", reason: ReasonArtifactField},
		{name: "long opaque password", input: "username1:" + strings.Repeat("Ab1!", 13), reason: ReasonArtifactField},
		{name: "REQ filename in username", input: "Request_FIEL_ABC_20220427_105817.req:normalPass1", reason: ReasonFilename},
		{name: "XLSM filename in username", input: "StudentCollegeMatchTool-SanAntonio_PC.xlsm:normalPass1", reason: ReasonFilename},
		{name: "database filename in username", input: "Editor.Seller.VPS.Database:normalPass1", reason: ReasonFilename},
		{name: "long metadata username", input: "st.Metadata225bl.B20ap.49et.3jud.CityName:normalPass1", reason: ReasonArtifactField},
		{name: "embedded UUID in username", input: "account-07450627-02a5-4b20-bca7-8df441cbebdc:normalPass1", reason: ReasonArtifactField},
		{name: "social login metadata in password", input: "user@example.com:C0re1SocialLogin_google_101251669829461881989", reason: ReasonArtifactField},
		{name: "Edge application profile", input: `"055110UiApplication","Microsoft_[Edge]_Default"`, reason: ReasonArtifactField},
		{name: "Edge profile second login", input: `"Solano2021Application","Microsoft_[Edge]_Default"`, reason: ReasonArtifactField},
		{name: "Edge profile third login", input: `"Barcelona31Application","Microsoft_[Edge]_Default"`, reason: ReasonArtifactField},
		{name: "Chrome application profile", input: `"Solano2021Application","Google_[Chrome]_Profile_1"`, reason: ReasonArtifactField},
		{name: "Firefox profile with ordinary login", input: `"reader123","Mozilla_[Firefox]_Default"`, reason: ReasonArtifactField},
		{name: "browser logins path", input: `"application","Browser/Logins/Chrome_Profile"`, reason: ReasonArtifactField},
		{name: "browser logins path with prefix", input: `"reader123","Data/Browser/Logins/Firefox_Profile"`, reason: ReasonArtifactField},
		{name: "installation path", input: `"reader123","680/mm/install/install/step5"`, reason: ReasonArtifactField},
		{name: "portal path with IP login", input: `"189.39.118.118","8080/portal/WPortalRH.dll/$/"`, reason: ReasonArtifactField},
		{name: "short OAuth token", input: `"reader123","1/glulDo5UuFlz1jKCUE4SEQty0H"`, reason: ReasonArtifactField},
		{name: "another short OAuth token", input: `"reader123","4/AbCdEfGhIjKlMnOpQrStUvWx"`, reason: ReasonArtifactField},
		{name: "application target metadata", input: `"LegacyGeneric","target=com.logi.ghub/shared"`, reason: ReasonArtifactField},
		{name: "software browser profile", input: `"PASSfakepassword2","SOFTGoogleChrome(Profile1)"`, reason: ReasonArtifactField},
		{name: "encoded value marker", input: `"542013460721","{base64}_MDIwMjE2I0x5eA=="`, reason: ReasonArtifactField},
		{name: "template assignment", input: `"seyi@novatiatranslations.com.ng","seyi_insert=(program)*@%#"`, reason: ReasonArtifactField},
		{name: "Opera software profile", input: `"NainaCargo2018","OperaSoftware_OperaStable"`, reason: ReasonArtifactField},
		{name: "other software profile", input: `"reader123","AcmeSoftware_BrowserProfile2"`, reason: ReasonArtifactField},
		{name: "overlong email login", input: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0@example.com:normalPass1", reason: ReasonArtifactField},
		{name: "URL path fused with email", input: "portal.com/register/user@example.com:normalPass1", reason: ReasonArtifactField},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewProcessor()
			if out, ok := p.ProcessLine(tc.input); ok {
				t.Fatalf("expected drop, got %q", out)
			}
			if got := p.Stats.Reasons[tc.reason]; got != 1 {
				t.Fatalf("reason %q count=%d; all reasons=%v", tc.reason, got, p.Stats.Reasons)
			}
		})
	}
}

func TestPipelineDropsBracketedValues(t *testing.T) {
	cases := []struct {
		name, input string
	}{
		{"failure marker", `"bhanu008009","[fail]"`},
		{"unclosed save marker", `"H5bbzksb","[8OT_SAVE99"`},
		{"unclosed save marker with email", `"s9uravrm804@gmail.com","[0OT_SAVE06"`},
		{"ordinary word", `"Salty_Sharkxxx","[waterbird]"`},
		{"password marker", `"ruibaco@sapo.pt","[omw_password]"`},
		{"password-like content", `"9260364643","[Mysteria2002]"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewProcessor()
			if out, ok := p.ProcessLine(tc.input); ok {
				t.Fatalf("expected drop, got %q", out)
			}
			if got := p.Stats.Reasons[ReasonArtifactField]; got != 1 {
				t.Fatalf("artifact reason count=%d; all reasons=%v", got, p.Stats.Reasons)
			}
		})
	}
}

func TestPipelineCleansMD5DateSuffix(t *testing.T) {
	const hash = "5d41402abc4b2a76b9719d911017c592"
	p := NewProcessor()
	out, ok := p.ProcessLine("username1:" + hash + "A1b2C3d4_31.12.2025")
	if !ok || out != csvPair("username1:"+hash) {
		t.Fatalf("got ok=%v out=%q", ok, out)
	}
}

func TestPipelineRejectsMalformedMD5DateSuffix(t *testing.T) {
	p := NewProcessor()
	if out, ok := p.ProcessLine("username1:5d41402abc4b2a76b9719d911017c592A1b2C3d4_31-02-2025"); ok {
		t.Fatalf("expected drop, got %q", out)
	}
}

func TestPipelineDropsOversizeRawLineBeforeExtraction(t *testing.T) {
	p := NewProcessor()
	input := "user@example.com:ENC893*" + strings.Repeat("x", maxRawLineLen) + ":other:pass1234"
	if out, ok := p.ProcessLine(input); ok {
		t.Fatalf("expected drop, got %q", out)
	}
	if got := p.Stats.Reasons[ReasonOversizeLine]; got != 1 {
		t.Fatalf("oversize reason count=%d; all reasons=%v", got, p.Stats.Reasons)
	}
}

func TestPipelineKeepsURLLoginPass(t *testing.T) {
	colonHash := "5d41402abc4b2a76b9719d911017c592:abcd1234"
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "bare domain prefix",
			in:   "example.com:user123:pass1234",
			want: "user123:pass1234",
		},
		{
			name: "HTTP URL prefix",
			in:   "https://example.com/login:user@mail.com:pass1234",
			want: "user@mail.com:pass1234",
		},
		{
			name: "HTTP URL prefix and dotted username",
			in:   "https://example.com/login:user.name:pass1234",
			want: "user.name:pass1234",
		},
		{
			name: "URL with port",
			in:   "http://example.com:8080:user123:pass1234",
			want: "user123:pass1234",
		},
		{
			name: "URL with tracking query",
			in:   "https://example.com/?utm_source=feed:user123:pass1234",
			want: "user123:pass1234",
		},
		{
			name: "short URL prefix",
			in:   "https://bit.ly/path:user123:pass1234",
			want: "user123:pass1234",
		},
		{
			name: "credentials before HTTP URL",
			in:   "user123:pass1234:https://example.com/login",
			want: "user123:pass1234",
		},
		{
			name: "credentials before Android URL",
			in:   "user123:pass1234:android://app",
			want: "user123:pass1234",
		},
		{
			name: "credentials before bare host",
			in:   "user123:pass1234:portal.example",
			want: "user123:pass1234",
		},
		{
			name: "pipe separated",
			in:   "https://example.com|user123|pass1234",
			want: "user123:pass1234",
		},
		{
			name: "semicolon separated reverse order",
			in:   "user123;pass1234;https://example.com",
			want: "user123:pass1234",
		},
		{
			name: "tab separated",
			in:   "https://example.com\tuser123\tpass1234",
			want: "user123:pass1234",
		},
		{
			name: "space separated reverse order",
			in:   "user123 pass1234 https://example.com",
			want: "user123:pass1234",
		},
		{
			name: "quoted CSV",
			in:   `"https://example.com","user123","pass1234"`,
			want: "user123:pass1234",
		},
		{
			name: "quoted CSV reverse order",
			in:   `"user123","pass1234","https://example.com"`,
			want: "user123:pass1234",
		},
		{
			name: "URL prefix and colon hash",
			in:   "https://example.com/login:user123:" + colonHash,
			want: "user123:5d41402abc4b2a76b9719d911017c592",
		},
		{
			name: "URL suffix and colon hash",
			in:   "user123:" + colonHash + ":https://example.com/login",
			want: "user123:5d41402abc4b2a76b9719d911017c592",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewProcessor()
			out, ok := p.ProcessLine(tc.in)
			want := csvPair(tc.want)
			if !ok || out != want {
				t.Fatalf("ProcessLine(%q) = %q, %v; want %q, true", tc.in, out, ok, want)
			}
		})
	}
}

func TestPipelineDropCases(t *testing.T) {
	drops := []string{
		"user:http://site.com",
		"user:www.site.com",
		"user:user",
		"ab:cdefgh",
		"abcd:ef",
		"test@x.com:abcdef",
		"good@x.com:not_saved",
		"good@x.com:notSaved1",
		"good@x.com:aa???bb",
		"good@x.com:aa**bb",
		"plainword",
		"https://example.com/x",
		"user;pass",
		"ADMIN@x.com:abcdef",
	}
	for _, in := range drops {
		p := NewProcessor()
		if out, ok := p.ProcessLine(in); ok {
			t.Fatalf("in=%q: expected drop, got %q", in, out)
		}
	}
}

func TestContainsURLEmailSafe(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"user@mail.com", false},
		{"http://mail.com", true},
		{"www.mail.com", true},
		{"goto.mail.com/x", true},
		{"notmail.com", true},
		{"user@mail.com:pass", false},
		{"first.last+tag@mail.com", false},
		{"portal.com/register/user@mail.com", true},
		{"folder/user@mail.com", true},
	}
	for _, tc := range cases {
		if got := ContainsURL(tc.in); got != tc.want {
			t.Fatalf("ContainsURL(%q)=%v want %v", tc.in, got, tc.want)
		}
	}
}

func TestHTTPPrefixThenCredsSurvivesEarly(t *testing.T) {
	p := NewProcessor()
	out, ok := p.ProcessLine("http://x.com user@a.b p")
	// user@a.b — TLD "b" is not in list; username path...
	// email regex needs TLD [A-Z]{2,63}. "b" is 1 char — invalid email.
	// last two: http://x.com:user@a.b:p → wait after normalize:
	// spaces→ fields join: http://x.com:user@a.b:p
	// extract may get user@a.b:p? regex needs email at start...
	// P6: nonEmpty last two = user@a.b + p → user@a.b:p
	// valid email? TLD b — only 1 letter — invalid → drop invalid_pair
	// Spec example says After P6: user@a.b:p — doesn't say keep.
	// So just ensure not early_url_full:
	_ = out
	if p.Stats.Reasons[ReasonEarlyURLFull] > 0 {
		t.Fatal("should not early-drop multi-token http line")
	}
	_ = ok
}

func TestDedupe(t *testing.T) {
	p := NewProcessor()
	if _, ok := p.ProcessLine("user@mail.com:pass123"); !ok {
		t.Fatal("first should keep")
	}
	if _, ok := p.ProcessLine("User@Mail.com:pass123"); ok {
		t.Fatal("duplicate CI should drop")
	}
	if p.Stats.Duplicates != 1 {
		t.Fatalf("duplicates=%d", p.Stats.Duplicates)
	}
}

func TestBanNotSavedNormalized(t *testing.T) {
	// After normalize, F9 phrase is matched via "not:saved" on the final login:pass line.
	p := NewProcessor()
	if out, ok := p.ProcessLine("xxnot:saved1"); ok {
		t.Fatalf("expected ban drop, got %q", out)
	}
	if p.Stats.Reasons[ReasonBanned] == 0 {
		t.Fatalf("want banned_substring, reasons=%v", p.Stats.Reasons)
	}
	// Space form is also dropped (may be short_field after last-two, or ban).
	p2 := NewProcessor()
	if out, ok := p2.ProcessLine("good@x.com:not saved"); ok {
		t.Fatalf("expected drop for spaced not saved, got %q", out)
	}
}
