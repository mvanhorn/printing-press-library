// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.
// Work that must wait until --deliver has shipped a command's output.

package cli

// deliveryHooks run after the buffered output reached its --deliver sink.
// A command registers one when its side effect (for example marking news
// articles as seen) must not happen if the report never arrives.
var deliveryHooks []func() error

func afterDelivery(fn func() error) {
	deliveryHooks = append(deliveryHooks, fn)
}

// runDeliveryHooks runs and clears the registered hooks. skip drops them
// without running, used when the command or the delivery failed.
func runDeliveryHooks(skip bool) error {
	hooks := deliveryHooks
	deliveryHooks = nil
	if skip {
		return nil
	}
	for _, fn := range hooks {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}
