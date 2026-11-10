// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

type platformCLIResolver struct{ value []byte }
type platformCLIAdapter struct{ observed platform.ObservedIdentity }

func platformCLITestRegistration(adapter platform.IdentityAdapter) *platformSourceRegistration {
	return &platformSourceRegistration{
		Source: "test-source", Adapter: adapter,
		ValidateSourceProfile:     func(source platform.SourceProfile) error { return source.Validate() },
		CredentialResolverFactory: func() platform.CredentialResolver { return platformResolverFactory() },
	}
}

func (r platformCLIResolver) Resolve(context.Context, string) ([]byte, error) {
	return append([]byte(nil), r.value...), nil
}

func (a platformCLIAdapter) ProbeIdentity(context.Context, platform.ResolvedCredentials, platform.SourceProfile) (platform.ObservedIdentity, error) {
	return a.observed, nil
}

func (platformCLIAdapter) EndpointClass() string { return "GET /identity" }
