// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package client

import (
	"errors"
	"net/http"
	"net/url"
)

var errCarstayForeignRedirect = errors.New("refusing Carstay provider redirect: foreign effective origin")
var errCarstayRedirectLimit = errors.New("stopped after 10 redirects")

// Policy refusals cannot become valid by replaying the original request.
func permanentCarstayRedirectError(err error) bool {
	return errors.Is(err, errCarstayForeignRedirect) || errors.Is(err, errCarstayRedirectLimit) ||
		errors.Is(err, ErrRedirectUnsupportedScheme) || errors.Is(err, ErrRedirectProtocolDowngrade) ||
		errors.Is(err, ErrRedirectPrivateDestination)
}

// Observed directory contracts stay on their original provider origin. Go
// otherwise retains arbitrary configured/request headers across public-host
// redirects. Refuse the hop before any inherited credential can be sent.
func refuseCarstayForeignRedirect(next *url.URL, via []*http.Request) error {
	if redirectTargetLeavesOrigin(next, via) {
		return errCarstayForeignRedirect
	}
	return nil
}
