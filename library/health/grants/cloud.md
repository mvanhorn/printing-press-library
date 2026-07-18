# ☁️ cloud.md — grants-pp-cli (memory-manager, <200 sor)

> Utolsó frissítés / Last update: 2026-07-05 · Státusz: **v1 kész, élő API-n verifikálva**

## 📌 Státusz / Status
- CLI: `grants-pp-cli` — nyitott kutatási pályázatok, kulcs nélkül (Grants.gov + NIH + NSF)
- Build: ✔ `go vet` tiszta, `go build` OK, `go test` zöld (12 alteszt)
- Élő verifikáció: ✔ doctor mind a 3 forrás UP; search/nih/nsf valós sorokat ad;
  `--closing-before`, `--min-award`, `--min-amount` bizonyítottan szűr
- Deployment: ⏸ GitHub push/PR **felhasználói jóváhagyásra vár** (nincs auto-push)

## 🧱 Felépítés / Layout
```
cmd/grants-pp-cli/main.go       belépő
internal/sources/               http.go, grantsgov.go, nih.go, nsf.go, money.go
internal/cli/                   root, search, nih, nsf, doctor, filter (+filter_test)
```

## 📐 Tervezési szabályok (retraction-checker minta)
- Keyless: nincs API-kulcs, .env, regisztráció. (security-scan: 0 secret-literál)
- Nincs `exec.Command` (scan: 0 találat). Minden hívás közvetlen `net/http`.
- Stdlib-only: nincs go.sum, zéró külső függőség. 731 LOC.
- Szűrő-logika tiszta függvényekben (filter.go) → unit-tesztelt.

## ⚠️ Ismert korlátok / Known limits
- Grants.gov `awardCeiling` gyakran 0 → `AwardCap()` visszaesik `estimatedFunding`-ra
  (kijelzésnél "becsült" címke jelzi).
- NSF keyword-relevancia laza (full-text OR); a `--min-amount` szűr, de a lista
  tágabb lehet a vártnál — ez az API viselkedése, nem hiba.
- NIH/NSF *megítélt* granteket ad (benchmark), a *nyitott* kiírások a Grants.gov-ból.

## ▶️ Következő lépés / Next
`deployment` ügynök: `git init` + push + PR — CSAK felhasználói "menj" után.
