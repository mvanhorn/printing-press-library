# Fandango research

The build uses the official Fabric Origin Fandango OpenAPI contract and its documented licensed access model. The approved scope includes all twelve official endpoints plus `movie-plan`, `starting-soon`, `format-find`, `theater-compare`, `movie-availability`, and `watchlist-showtimes`.

The CLI deliberately does not scrape fandango.com, complete payment, promise seat selection, or rank prices that the official contract does not expose consistently.
