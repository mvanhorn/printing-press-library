# Lifecycle proof: brief cites -> brief check -> brief gaps -> citecheck packet

Run 2026-11-16 in a disposable sandbox (fresh home, fresh pull log, fresh output folder) against a real Lexis+ subscription session, using a two-sentence sample brief (`brief.txt`) that cites one California case and one statute section. These commands take a brief file the dogfood harness cannot supply, so they are proved here.

```
$ lexis-plus-pp-cli brief cites <sandbox>/src/brief.txt
27 Cal.3d 167
Cal Lab Code 1102.5
(exit 0)
```

```
$ lexis-plus-pp-cli brief check <sandbox>/src/brief.txt
recognized      27 Cal.3d 167                
recognized      Cal Lab Code 1102.5          
(exit 0)
```

```
$ lexis-plus-pp-cli brief gaps <sandbox>/src/brief.txt
Cited but not pulled (2):
  27 Cal.3d 167
  Cal Lab Code 1102.5
Pulled but not cited in this draft (0):
(exit 0)
```

```
$ lexis-plus-pp-cli citecheck packet <sandbox>/src/brief.txt --out <sandbox>/packet --no-shepards
READY   27 Cal.3d 167  Tameny v. Atlantic Richfield Co.  cited 1 time(s)
READY   Cal Lab Code 1102.5  Cal Lab Code § 1102.5  cited 1 time(s)
manifest -> <sandbox>/packet/manifest.json
(exit 0)
```

Packet folder:
```
  Cal. Lab. Code 1102.5.citecheck.txt
  Cal. Lab. Code 1102.5.docx
  Tameny v. Atlantic Richfield Co. - 27 Cal. 3d 167.citecheck.txt
  Tameny v. Atlantic Richfield Co. - 27 Cal. 3d 167.docx
  manifest.json
```
