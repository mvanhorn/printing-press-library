# Live videos clips restore-cuts lifecycle

An owned pp-test video was edited using videos clips clean --range 1000:1100 --apply, which saved the exact cuts snapshot and reported the applied operation. Then videos clips restore-cuts --snapshot SNAPSHOT --apply --confirm-exclusive-editing restored the snapshot. A fresh videos clips get proved exact equality with the pre-clean cuts. No original video was edited. Private raw evidence is excluded under <local-evidence>
