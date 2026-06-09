package auth

import (
	"net/http/cookiejar"

	"golang.org/x/net/publicsuffix"
)

func newJar() (*cookiejar.Jar, error) {
	return cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
}
