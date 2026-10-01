# Shipcheck

Canonical umbrella PASS: all seven legs exit0. Verify {'mode': 'mock', 'pass_rate': 100, 'passed': 20, 'total': 20, 'data_pipeline': True, 'verdict': 'PASS'}; score {'percentage': 80, 'grade': 'A', 'total': 80}. Full live source correctness runner supplements structural/mock verification. First run failed narrative; focused narrative replay passed and the final full umbrella passed. Source schema/live fixes are recorded in build log and implementation diff.

Proof: shipcheck.json, shipcheck.stderr. No acceptance marker was hand-authored. Independent review and full live dogfood still required before promotion.
