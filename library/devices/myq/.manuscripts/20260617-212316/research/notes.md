# MyQ Research Notes

Sources used for the print:

- Archived `joeshaw/myq` code and README for the legacy login flow and the device endpoints.
- Live network search confirming that the core myQ account/device flow still exists without needing a paid subscription for basic garage-door control.

Observed command surface:

- `devices` lists accounts and garage devices.
- `state <serial-number>` fetches the current door state.
- `open <serial-number>` sends the open action and waits for the door to report open.
- `close <serial-number>` sends the close action and waits for the door to report closed.

Auth shape:

- Username/password login is required.
- No subscription gate is required for the basic open/close flow.
