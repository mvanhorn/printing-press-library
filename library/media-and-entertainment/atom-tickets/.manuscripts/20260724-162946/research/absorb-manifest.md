# Atom Tickets research

The build uses the official Atom Tickets Partner API documentation at `developers.atomtickets.com` and the documented approved-partner access model. The official endpoint tables were transcribed into the archived OpenAPI contract because the documentation site protects its downloadable OpenAPI file.

The approved scope includes all fourteen documented public Partner API discovery endpoints plus `movie-plan`, `deal-finder`, `accessible-showtimes`, `family-fit`, `preorder-radar`, and `last-call`.

The CLI deliberately does not scrape atomtickets.com, complete payment, promise adjacent seats from aggregate inventory, or expose partner-restricted ordering operations. It returns the documented `checkoutUrl` for the user to complete the purchase on Atom.
