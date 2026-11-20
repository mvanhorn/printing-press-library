//go:build !windows

package cli

import "syscall"

const etsyNoFollow = syscall.O_NOFOLLOW
