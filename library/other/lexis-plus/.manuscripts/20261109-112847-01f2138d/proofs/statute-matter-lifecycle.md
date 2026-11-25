# Lifecycle proof: statute get -> statute whatsnew -> matter list -> matter binder

Run 2026-11-16 in a disposable sandbox (fresh home, fresh pull log, fresh output folder) against a real Lexis+ subscription session. One California statute section, Cal. Lab. Code 1102.5, was pulled; no client or account data involved. `statute whatsnew` and `matter binder` only work on a pull that an earlier command recorded, which is why they are proved here instead of by a single-command happy path.

```
$ lexis-plus-pp-cli statute get Cal Lab Code 1102.5 --out <sandbox>/out --matter Proof
SAVED   Cal. Lab. Code 1102.5  Cal. Lab. Code 1102.5 (with annotations)
        -> <sandbox>/out/Cal. Lab. Code 1102.5.docx
(exit 0)
```

```
$ lexis-plus-pp-cli statute whatsnew Cal Lab Code 1102.5
Compared with your pull of 2026-11-16
New copy: <sandbox>/out/Cal. Lab. Code 1102.5 2026-11-16.docx

Added (0):

Removed (0):
(exit 0)
```

```
$ lexis-plus-pp-cli matter list
Proof                          2
(exit 0)
```

```
$ lexis-plus-pp-cli matter binder Proof --out <sandbox>/binder
Binder written to <sandbox>/binder (2 files + index.csv)
(exit 0)
```

Binder contents:
```
  01 - Cal. Lab. Code 1102.5.docx
  02 - Cal. Lab. Code 1102.5.docx
  index.csv
```
