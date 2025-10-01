package utils

import "net/url"

func IsValidURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	return true
}
