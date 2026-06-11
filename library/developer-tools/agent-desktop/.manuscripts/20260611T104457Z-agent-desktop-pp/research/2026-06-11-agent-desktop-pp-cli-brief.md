# Agent Desktop Printing Press Brief

Goal: make `agent-desktop` visible in Printing Press without moving desktop
automation logic into the Printing Press catalog.

Decision: publish a small Go bridge named `agent-desktop-pp-cli`. The bridge
installs or delegates to the real Rust `agent-desktop` package. It does not
launch apps, take snapshots, click UI, or synthesize input by itself.

Distribution source:

- Repository: `https://github.com/lahfir/agent-desktop`
- npm package: `agent-desktop`
- Release assets: GitHub Releases for `lahfir/agent-desktop`

The bridge defaults its installer to `agent-desktop@latest`, so newly installed
Printing Press users resolve the current remote package instead of a copied
binary snapshot.
