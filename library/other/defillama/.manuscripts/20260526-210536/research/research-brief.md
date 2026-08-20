# DefiLlama CLI Research Brief

Generated from the public DefiLlama OpenAPI description and packaged through the Printing Press publish path.

The CLI exposes DefiLlama protocol, TVL, stablecoin, fees, DEX, raises, hacks, oracles, treasury, and institutional endpoints with agent-friendly JSON flags, local sync/search, analytics, and compound workflow commands.

Spec normalization note: the source OpenAPI document used JSON-Schema tuple-style `items: [...]` in several chart arrays. For Printing Press compatibility, those tuple item arrays were normalized to `items.oneOf` in the local generation copy at `/tmp/defillama-api.printing-press.yaml`; endpoint paths and descriptions were otherwise preserved from the source spec.
