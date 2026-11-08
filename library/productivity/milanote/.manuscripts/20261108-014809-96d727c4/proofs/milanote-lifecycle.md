# Milanote write lifecycle proof

Sandbox: disposable boards created inside a dedicated test board on the operator's own account, all trashed at the end. Ids are replaced with placeholders. Run with the final source tree.

## Setup
$ milanote-pp-cli boards create CLI proof (safe to delete) --parent <sandbox-board> --json  # exit 0
  created <test-board>

## layout md, layout check, outline
$ milanote-pp-cli layout md test.md --board <test-board> --json  # exit 0
  verified: 5 native cards created
$ milanote-pp-cli boards get <test-board> --json  # exit 0
  verified: read back 5 cards including a native to-do list
$ milanote-pp-cli layout check <test-board> --json  # exit 0
  verified: tidiness 100, no overlaps
$ milanote-pp-cli outline <test-board> --json  # exit 0
  verified: outline carries heading and ticked item

## layout csv, images grid
$ milanote-pp-cli layout csv shots.csv --board <test-board> --title-col Shot --cols 2 --json  # exit 0
$ milanote-pp-cli boards get <test-board> --json  # exit 0
  verified: 2 numbered cards added from CSV
$ milanote-pp-cli images grid frames --board <test-board> --cols 2 --number --json  # exit 0
  verified: 2 images uploaded in natural filename order
$ milanote-pp-cli boards get <test-board> --type image --json  # exit 0
  verified: both images on the board

## snapshot diff
$ milanote-pp-cli snapshot save <test-board> --name base --json  # exit 0
$ milanote-pp-cli boards get <test-board> --type task_list --json  # exit 0
$ milanote-pp-cli boards get <test-board> --json  # exit 0
$ milanote-pp-cli todos check <test-board> <task-list> rent lens --json  # exit 0
$ milanote-pp-cli elements move <test-board> <note> --x 4 --y 12 --json  # exit 0
$ milanote-pp-cli snapshot diff <test-board> --from base --json  # exit 0
  verified: diff reports the move and the tick

## layout grid and undo
$ milanote-pp-cli boards get <test-board> --json  # exit 0
$ milanote-pp-cli layout grid <test-board> --cols 2 --json  # exit 0
  verified: cards re-spaced
$ milanote-pp-cli layout check <test-board> --json  # exit 0
  verified: no overlaps after grid
$ milanote-pp-cli layout grid <test-board> --undo --json  # exit 0
  verified: undo restored every moved card
$ milanote-pp-cli boards get <test-board> --json  # exit 0
  verified: positions identical to before the grid

## template apply
$ milanote-pp-cli template save <test-board> --out kit.yaml --no-content --json  # exit 0
$ milanote-pp-cli template apply kit.yaml --parent <sandbox-board> --name CLI proof kit (safe to delete) --json  # exit 0
  verified: new board built from kit
$ milanote-pp-cli boards get <kit-board> --json  # exit 0
  verified: kit board has its cards

## mirror
$ milanote-pp-cli boards create CLI proof mirror (safe to delete) --parent <sandbox-board> --json  # exit 0
$ milanote-pp-cli mirror mirror.md --board <mirror-board> --once --json  # exit 0
  verified: first sync created 3 cards
$ milanote-pp-cli mirror mirror.md --board <mirror-board> --once --json  # exit 0
  verified: file edit pushed to the board
$ milanote-pp-cli boards get <mirror-board> --json  # exit 0
$ milanote-pp-cli notes edit <mirror-board> <note2> second from board --json  # exit 0
$ milanote-pp-cli mirror mirror.md --board <mirror-board> --once --json  # exit 0
  verified: board edit pulled to the file
  verified: file now contains the board edit
$ milanote-pp-cli notes edit <mirror-board> <note2> second board side --json  # exit 0
$ milanote-pp-cli mirror mirror.md --board <mirror-board> --once --json  # exit 0
  verified: two-sided edit reported, neither side overwritten
$ milanote-pp-cli mirror mirror.md --board <mirror-board> --once --prefer file --json  # exit 0
  verified: --prefer file settled the conflict

## chrome-login (throwaway Chrome profile, temporary config; no real config touched)
  throwaway Chrome: seeded
$ milanote-pp-cli chrome-login --port 9334 --json  # exit 0
  verified: session cookie read from Chrome, accepted by Milanote, and saved
$ milanote-pp-cli whoami --json  # exit 0 (from the saved temporary login)
  verified: saved login works on a fresh process
  temporary Chrome profile and config deleted

## Cleanup
$ milanote-pp-cli elements trash <sandbox-board> <test-board> <mirror-board> <kit-board> --json  # exit 0
  verified: all 3 disposable boards trashed
$ milanote-pp-cli boards list --parent <sandbox-board> --depth 1 --json  # exit 0
  verified: no disposable boards remain
  local snapshot, undo and pairing records for these boards removed
