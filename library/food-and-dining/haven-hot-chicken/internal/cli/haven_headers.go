package cli

import "github.com/mvanhorn/printing-press-library/library/food-and-dining/haven-hot-chicken/internal/client"

// Public brand/version routing is needed by generic sync as well as endpoint commands.
func init() {
	registerClientHook(func(c *client.Client) error {
		if c.Config.Headers == nil {
			c.Config.Headers = map[string]string{}
		}
		if c.Config.Headers["Accept-Version"] == "" {
			c.Config.Headers["Accept-Version"] = "v3.5"
		}
		if c.Config.Headers["Thanx-Merchant"] == "" {
			c.Config.Headers["Thanx-Merchant"] = "havenhotchicken"
		}
		return nil
	})
}
