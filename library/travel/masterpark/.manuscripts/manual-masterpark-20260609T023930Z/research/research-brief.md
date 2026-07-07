# MasterPark Research Brief

MasterPark's reservation website uses the `netParkV2` WordPress plugin and AJAX endpoint at `/wp-content/plugins/netParkV2/ajax.php`. The CLI fetches the reservation page to obtain the CSRF nonce, then calls the same JSON methods the browser uses.

Important observed methods:

- `getQuotes` for price quotes.
- `verifyLogin` for account session/profile validation.
- `listReservations` for account reservation history/profile view.
- `saveReservation` for gated reservation submission.

The CLI is intentionally conservative:

- Booking is dry-run by default.
- Real booking requires `--submit --yes`.
- `PRINTING_PRESS_VERIFY=1` prevents mutation.
- Passwords are not printed or persisted.
- Credential command flags allow external secret managers such as 1Password without making the CLI 1Password-specific.

Live finding: immediately after a successful `saveReservation`, MasterPark sent a confirmation email but `listReservations` did not include the new booking. The docs therefore treat save response + email as authoritative for fresh bookings.
