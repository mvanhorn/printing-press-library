# Publication snapshot review fixes

Three valid automated-review findings are fixed: offline search rejects requested kinds/directions/dates outside the saved source scope; material diffs compare full provider facility entries including map links; automatic pairing uses validated origin/date/kind/direction content and selects the newest compatible pair, including older usable pairs when the latest scope is unpaired. Empty v1 query mode remains compatible with explicit board mode. Automatic selection reads at most 64 MiB; explicit file comparisons retain per-file 8 MiB bounds.

Source full Go tests and vet passed. All seven actual shipcheck legs passed after the changes. The fresh publication live gate passed all 171 executed rows and wrote a runner-owned acceptance marker; 107 skipped/unverified rows are outside that denominator. The same independent reviewer verified the three fixes with targeted suites and 20 independent offline CLI checks, including adjacent service days and the 64 MiB bound, and returned PASS.

No runtime source edit occurred after that fresh acceptance run. Default offline search retains adjacent-day rows. Explicit --date must equal the saved source request date and then selects that service day; a rollover row alone is not evidence of a complete board for a different request date.
