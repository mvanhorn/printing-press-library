# Pennylane PP CLI — Research Brief

**Run ID**: 20260510-000758
**API**: Pennylane French Accounting SaaS
**Spec**: https://raw.githubusercontent.com/wanadev/pennylane-mcp/main/openapi/accounting.json

## Alternatives existantes

- `wanadev/pennylane-mcp` (TypeScript, MCP server) — couvre l'API REST mais sans analytics offline, sans validation FEC, sans projection tresorerie.

## Gaps identifies

1. Aucun outil existant ne calcule la balance agee (AR aging) en local sans appel API.
2. La validation FEC DGFiP doit etre faite manuellement ou via des outils payants.
3. La projection de tresorerie (runway) necessite Excel ou un comptable.
4. La detection de derive sur factures recurrentes n'existe pas en CLI.
5. La checklist de cloture annuelle n'est pas automatisee.

## Decision

Proceder au print. Novelty score : 9/10. Les 12 commandes novel apportent une valeur analytique absente de tous les outils existants.
