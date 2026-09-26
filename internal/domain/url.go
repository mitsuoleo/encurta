package domain

import (
	"net"
	"net/url"
	"strconv"
	"strings"
)

func NormalizeAndValidateURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrInvalidURL
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "file:") {
		return "", ErrInvalidURL
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", ErrInvalidURL
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", ErrInvalidURL
	}
	if parsed.Host == "" {
		return "", ErrInvalidURL
	}
	if parsed.User != nil {
		return "", ErrInvalidURL
	}
	if forbiddenHost(parsed.Hostname()) {
		return "", ErrInvalidURL
	}
	return parsed.String(), nil
}

func forbiddenHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return true
	}
	switch host {
	case "localhost", "metadata.google.internal", "metadata.internal":
		return true
	}
	if strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		ip = parseWeirdIPv4(host)
	}
	if ip == nil {
		return false
	}
	return forbiddenIP(ip)
}

func forbiddenIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

func parseWeirdIPv4(host string) net.IP {
	parts := strings.Split(host, ".")
	if len(parts) < 1 || len(parts) > 4 {
		return nil
	}
	vals := make([]uint64, len(parts))
	for i, p := range parts {
		n, ok := parseIPv4Atom(p)
		if !ok {
			return nil
		}
		vals[i] = n
	}
	var a, b, c, d uint64
	switch len(vals) {
	case 1:
		if vals[0] > 0xffffffff {
			return nil
		}
		a, b, c, d = vals[0]>>24, (vals[0]>>16)&0xff, (vals[0]>>8)&0xff, vals[0]&0xff
	case 2:
		if vals[0] > 0xff || vals[1] > 0xffffff {
			return nil
		}
		a = vals[0]
		b, c, d = (vals[1]>>16)&0xff, (vals[1]>>8)&0xff, vals[1]&0xff
	case 3:
		if vals[0] > 0xff || vals[1] > 0xff || vals[2] > 0xffff {
			return nil
		}
		a, b = vals[0], vals[1]
		c, d = (vals[2]>>8)&0xff, vals[2]&0xff
	default:
		for _, v := range vals {
			if v > 0xff {
				return nil
			}
		}
		a, b, c, d = vals[0], vals[1], vals[2], vals[3]
	}
	return net.IPv4(byte(a), byte(b), byte(c), byte(d))
}

func parseIPv4Atom(s string) (uint64, bool) {
	if s == "" {
		return 0, false
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "0x") {
		if lower == "0x" {
			return 0, false
		}
		n, err := strconv.ParseUint(lower[2:], 16, 32)
		return n, err == nil
	}
	if len(s) > 1 && s[0] == '0' {
		for i := 0; i < len(s); i++ {
			if s[i] < '0' || s[i] > '7' {
				return 0, false
			}
		}
		n, err := strconv.ParseUint(s, 8, 32)
		return n, err == nil
	}
	n, err := strconv.ParseUint(s, 10, 32)
	return n, err == nil
}
