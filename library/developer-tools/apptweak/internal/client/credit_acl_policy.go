package client

import "errors"

// creditACLRule describes one Windows DACL entry after SID classification.
// Deny entries cannot grant access; every other unknown entry fails closed.
type creditACLRule struct {
	allow   bool
	deny    bool
	trusted bool
}

func creditACLPrivate(ownerCurrent, daclPresent bool, rules []creditACLRule) error {
	if !ownerCurrent || !daclPresent {
		return errors.New("journal owner or DACL is not private")
	}
	for _, rule := range rules {
		if rule.deny {
			continue
		}
		if !rule.allow || !rule.trusted {
			return errors.New("journal DACL has an untrusted access rule")
		}
	}
	return nil
}
