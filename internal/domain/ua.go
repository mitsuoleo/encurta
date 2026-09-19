package domain

import "strings"

func ClassifyUserAgent(ua string) (device, browser string) {
	l := strings.ToLower(ua)
	device = "desktop"
	switch {
	case strings.Contains(l, "ipad") || strings.Contains(l, "tablet"):
		device = "tablet"
	case strings.Contains(l, "mobi") || strings.Contains(l, "android") || strings.Contains(l, "iphone"):
		device = "mobile"
	case ua == "":
		device = "unknown"
	}

	switch {
	case strings.Contains(l, "edg/"):
		browser = "edge"
	case strings.Contains(l, "chrome/") && !strings.Contains(l, "edg/"):
		browser = "chrome"
	case strings.Contains(l, "firefox/"):
		browser = "firefox"
	case strings.Contains(l, "safari/") && !strings.Contains(l, "chrome/"):
		browser = "safari"
	case ua == "":
		browser = "unknown"
	default:
		browser = "other"
	}
	return device, browser
}
