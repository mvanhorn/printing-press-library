# TipMaster CLI Skill

```bash
WALLET=$(tipmaster resolve dwr --compact | jq -r '.wallet_address')
ghost-layer bridge --chain XRPL --recipient "$WALLET" --amount 1000000
```

**Related:** `ghost-layer-pp-cli`, `squeezeos-pp-cli`
