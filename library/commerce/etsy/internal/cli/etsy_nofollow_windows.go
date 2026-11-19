//go:build windows

package cli

// O_EXCL still refuses existing destinations, including symlinks.
const etsyNoFollow = 0
