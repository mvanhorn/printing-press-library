# AFFiNE CLI Research Summary

AFFiNE's GraphQL API gives broad access to workspace, document, comment, and sync surfaces, but agent workflows need safer canvas-specific commands than raw generated GraphQL operations.

This print keeps the generated GraphQL breadth and adds hand-authored canvas commands for layout planning, model normalization, validation, dry-run apply, card image patching, integrity checks, and narrowly scoped repair flows.

The canvas commands are designed around local validation first. Live mutation paths require explicit workspace and document identifiers, support dry-run behavior where applicable, and preserve AFFiNE document identity instead of rebuilding large canvas regions by default.

Novel command groups validated for this run:

- `canvas plan`: deterministic layout planning from a compact tree spec.
- `canvas model`: normalized node and connector extraction from AFFiNE canvas JSON.
- `canvas validate`: local geometry and connector validation.
- `canvas apply`: reviewable dry-run operation generation.
- `canvas card set-image`: bounded card image replacement.
- `canvas doc integrity`: corruption diagnostics for hidden or orphaned canvas content.
- `canvas doc repair`: auditable repair for supported integrity failures.
