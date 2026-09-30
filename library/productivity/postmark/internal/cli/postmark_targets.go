// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"errors"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil"
)

const (
	// postmarkFanoutConcurrency bounds parallel per-server and per-domain
	// reads so one account does not trip Postmark's rate limit.
	postmarkFanoutConcurrency = 4
	// postmarkDogfoodServers keeps a live-dogfood fan-out inside the
	// harness's per-command timeout.
	postmarkDogfoodServers = 2
)

// postmarkTarget is one server a fan-out command acts on, with a client that
// carries that server's token.
type postmarkTarget struct {
	Name         string
	ID           int64
	deliveryType string
	client       *client.Client
}

// targetScope says which servers resolvePostmarkTargets returns.
type targetScope struct {
	// allServers fans out across every server the account token lists when
	// no server was selected explicitly.
	allServers bool
	// dogfoodCap trims the list to postmarkDogfoodServers under the
	// live-dogfood harness.
	dogfoodCap bool
}

// resolvePostmarkTargets picks the servers a command reads: every listed
// server (scope.allServers), the one selected with --server, or the server
// behind the configured server token.
func resolvePostmarkTargets(ctx context.Context, flags *rootFlags, scope targetScope) ([]postmarkTarget, error) {
	targets, err := resolveAllPostmarkTargets(ctx, flags, scope.allServers)
	if err != nil {
		return nil, err
	}
	if scope.dogfoodCap && cliutil.IsDogfoodEnv() && len(targets) > postmarkDogfoodServers {
		targets = targets[:postmarkDogfoodServers]
	}
	return targets, nil
}

func resolveAllPostmarkTargets(ctx context.Context, flags *rootFlags, all bool) ([]postmarkTarget, error) {
	base, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	name, explicit := postmarkSelectedServer()
	hasAccount := hasAccountToken(base)
	if all && !explicit && hasAccount {
		refs, err := listPostmarkServers(base)
		if err != nil {
			return nil, classifyAPIErrorOnly(err)
		}
		targets := make([]postmarkTarget, 0, len(refs))
		for _, ref := range refs {
			c, err := flags.newClient()
			if err != nil {
				return nil, err
			}
			setPostmarkServerToken(c.Config, ref.token)
			targets = append(targets, ref.target(c))
		}
		return targets, nil
	}
	if explicit && hasAccount {
		ref, err := resolvePostmarkServer(base, name)
		if err != nil {
			return nil, err
		}
		return []postmarkTarget{ref.target(base)}, nil
	}
	if !hasServerToken(base.Config) {
		return nil, configErr(errors.New("no server selected: set POSTMARK_ACCOUNT_TOKEN and pass --server <name>, or set " + postmarkServerTokenEnv))
	}
	target, err := currentPostmarkServer(ctx, base)
	if err != nil {
		return nil, err
	}
	return []postmarkTarget{target}, nil
}

// currentPostmarkServer identifies the server behind the configured server token.
func currentPostmarkServer(ctx context.Context, c *client.Client) (postmarkTarget, error) {
	var info struct {
		ID   int64  `json:"ID"`
		Name string `json:"Name"`
	}
	if err := postmarkGetJSON(ctx, c, "/server", nil, &info); err != nil {
		return postmarkTarget{}, err
	}
	return postmarkTarget{Name: info.Name, ID: info.ID, client: c}, nil
}

// postmarkFanout runs fn for every item with bounded concurrency and returns
// the results in item order plus one failure per item that errored, built
// by label from the item's name.
func postmarkFanout[S, T any](ctx context.Context, items []S, name func(S) string, label func(name string, err error) postmarkFetchFailure, fn func(context.Context, S) (T, error)) ([]T, []postmarkFetchFailure) {
	results, errs := cliutil.FanoutRun(ctx, items, name, fn, cliutil.WithConcurrency(postmarkFanoutConcurrency))
	out := make([]T, 0, len(results))
	for _, r := range results {
		out = append(out, r.Value)
	}
	failures := make([]postmarkFetchFailure, 0, len(errs))
	for _, e := range errs {
		failures = append(failures, label(e.Source, e.Err))
	}
	return out, failures
}

// fanoutTargets runs fn once per server target; failures carry the server name.
func fanoutTargets[T any](ctx context.Context, targets []postmarkTarget, fn func(context.Context, postmarkTarget) (T, error)) ([]T, []postmarkFetchFailure) {
	return postmarkFanout(ctx, targets,
		func(t postmarkTarget) string { return t.Name },
		func(name string, err error) postmarkFetchFailure {
			return postmarkFetchFailure{Server: name, Error: err.Error()}
		},
		fn)
}
