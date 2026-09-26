package domain

import (
	"net/url"
	"strings"
	"unicode/utf8"
)

const maxClickFieldLen = 512

func SanitizeClickMeta(userAgent, referer string) (string, string) {
	return clipRunes(userAgent, maxClickFieldLen), sanitizeReferer(referer)
}

func sanitizeReferer(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return clipRunes(raw, maxClickFieldLen)
	}
	u.RawQuery = ""
	u.Fragment = ""
	return clipRunes(u.String(), maxClickFieldLen)
}

func clipRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	var b strings.Builder
	n := 0
	for _, r := range s {
		if n >= max {
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}
