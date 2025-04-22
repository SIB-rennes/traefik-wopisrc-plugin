// Package stickyheader
// DO NOT MODIFY THIS!
// cookie.go is copied from net/http/cookie.go in go 1.23.0 or above, and modified a little bit.
// Since ParseSetCookie func is introduced in Go 1.23.0. See doc https://pkg.go.dev/net/http#ParseSetCookie
// Up to now(2024.11.30), https://github.com/traefik/yaegi (latest version is v0.16.1) only supports Go version 1.21 and 1.22
// in this case, when using this func in yaegi, there will be a compile error: package http "net/http" has no symbol ParseSetCookie
// so, in order to use this func, we need to copy this file from net/http/cookie.go in Go 1.23.0
// please remove it and directly use http.ParseSetCookie when yaegi supports Go 1.23.0
package traefik_query_sticky

import (
	"errors"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	errBlankCookie           = errors.New("http: blank cookie")
	errEqualNotFoundInCookie = errors.New("http: '=' not found in cookie")
	errInvalidCookieName     = errors.New("http: invalid cookie name")
	errInvalidCookieValue    = errors.New("http: invalid cookie value")
)

// ParseSetCookie parses a Set-Cookie header value and returns a cookie.
// It returns an error on syntax error.
func ParseSetCookie(line string) (*http.Cookie, error) {
	parts := strings.Split(textproto.TrimString(line), ";")
	if len(parts) == 1 && parts[0] == "" {
		return nil, errBlankCookie
	}
	parts[0] = textproto.TrimString(parts[0])
	name, value, ok := strings.Cut(parts[0], "=")
	if !ok {
		return nil, errEqualNotFoundInCookie
	}
	name = textproto.TrimString(name)
	if !isCookieNameValid(name) {
		return nil, errInvalidCookieName
	}
	value, quoted, ok := parseCookieValue(value, true)
	if !ok {
		return nil, errInvalidCookieValue
	}
	c := &http.Cookie{
		Name:   name,
		Value:  value,
		Raw:    line,
	}
	for i := 1; i < len(parts); i++ {
		parts[i] = textproto.TrimString(parts[i])
		if len(parts[i]) == 0 {
			continue
		}

		attr, val, _ := strings.Cut(parts[i], "=")
		lowerAttr, isASCII := ToLower(attr)
		if !isASCII {
			continue
		}
		val, _, ok = parseCookieValue(val, false)
		if !ok {
			c.Unparsed = append(c.Unparsed, parts[i])
			continue
		}

		switch lowerAttr {
		case "samesite":
			lowerVal, ascii := ToLower(val)
			if !ascii {
				c.SameSite = http.SameSiteDefaultMode
				continue
			}
			switch lowerVal {
			case "lax":
				c.SameSite = http.SameSiteLaxMode
			case "strict":
				c.SameSite = http.SameSiteStrictMode
			case "none":
				c.SameSite = http.SameSiteNoneMode
			default:
				c.SameSite = http.SameSiteDefaultMode
			}
			continue
		case "secure":
			c.Secure = true
			continue
		case "httponly":
			c.HttpOnly = true
			continue
		case "domain":
			c.Domain = val
			continue
		case "max-age":
			secs, err := strconv.Atoi(val)
			if err != nil || secs != 0 && val[0] == '0' {
				break
			}
			if secs <= 0 {
				secs = -1
			}
			c.MaxAge = secs
			continue
		case "expires":
			c.RawExpires = val
			exptime, err := time.Parse(time.RFC1123, val)
			if err != nil {
				exptime, err = time.Parse("Mon, 02-Jan-2006 15:04:05 MST", val)
				if err != nil {
					c.Expires = time.Time{}
					break
				}
			}
			c.Expires = exptime.UTC()
			continue
		case "path":
			c.Path = val
			continue
		case "partitioned":
			continue
		}
		c.Unparsed = append(c.Unparsed, parts[i])
	}
	return c, nil
}

func isCookieNameValid(raw string) bool {
	if raw == "" {
		return false
	}
	return strings.IndexFunc(raw, isNotToken) < 0
}

func isNotToken(r rune) bool {
	return !IsTokenRune(r)
}

// parseCookieValue parses a cookie value according to RFC 6265.
// If allowDoubleQuote is true, parseCookieValue will consider that it
// is parsing the cookie-value;
// otherwise, it will consider that it is parsing a cookie-av value
// (cookie attribute-value).
//
// It returns the parsed cookie value, a boolean indicating whether the
// parsing was successful, and a boolean indicating whether the parsed
// value was enclosed in double quotes.
func parseCookieValue(raw string, allowDoubleQuote bool) (value string, quoted, ok bool) {
	// Strip the quotes, if present.
	if allowDoubleQuote && len(raw) > 1 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		raw = raw[1 : len(raw)-1]
		quoted = true
	}
	for i := 0; i < len(raw); i++ {
		if !validCookieValueByte(raw[i]) {
			return "", quoted, false
		}
	}
	return raw, quoted, true
}

func validCookieValueByte(b byte) bool {
	return 0x20 <= b && b < 0x7f && b != '"' && b != ';' && b != '\\'
}

// ToLower returns the lowercase version of s if s is ASCII and printable.
func ToLower(s string) (lower string, ok bool) {
	if !IsPrint(s) {
		return "", false
	}
	return strings.ToLower(s), true
}

// IsPrint returns whether s is ASCII and printable according to
// https://tools.ietf.org/html/rfc20#section-4.2.
func IsPrint(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < ' ' || s[i] > '~' {
			return false
		}
	}
	return true
}

var isTokenTable = [256]bool{
	'!':  true,
	'#':  true,
	'$':  true,
	'%':  true,
	'&':  true,
	'\'': true,
	'*':  true,
	'+':  true,
	'-':  true,
	'.':  true,
	'0':  true,
	'1':  true,
	'2':  true,
	'3':  true,
	'4':  true,
	'5':  true,
	'6':  true,
	'7':  true,
	'8':  true,
	'9':  true,
	'A':  true,
	'B':  true,
	'C':  true,
	'D':  true,
	'E':  true,
	'F':  true,
	'G':  true,
	'H':  true,
	'I':  true,
	'J':  true,
	'K':  true,
	'L':  true,
	'M':  true,
	'N':  true,
	'O':  true,
	'P':  true,
	'Q':  true,
	'R':  true,
	'S':  true,
	'T':  true,
	'U':  true,
	'W':  true,
	'V':  true,
	'X':  true,
	'Y':  true,
	'Z':  true,
	'^':  true,
	'_':  true,
	'`':  true,
	'a':  true,
	'b':  true,
	'c':  true,
	'd':  true,
	'e':  true,
	'f':  true,
	'g':  true,
	'h':  true,
	'i':  true,
	'j':  true,
	'k':  true,
	'l':  true,
	'm':  true,
	'n':  true,
	'o':  true,
	'p':  true,
	'q':  true,
	'r':  true,
	's':  true,
	't':  true,
	'u':  true,
	'v':  true,
	'w':  true,
	'x':  true,
	'y':  true,
	'z':  true,
	'|':  true,
	'~':  true,
}

func IsTokenRune(r rune) bool {
	return r < utf8.RuneSelf && isTokenTable[byte(r)]
}