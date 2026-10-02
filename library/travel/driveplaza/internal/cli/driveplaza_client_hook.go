package cli

import "github.com/mvanhorn/printing-press-library/library/travel/driveplaza/internal/client"

func init() {
	registerClientHook(func(c *client.Client) error { client.ApplyDrivePlazaLimits(c); return nil })
}
