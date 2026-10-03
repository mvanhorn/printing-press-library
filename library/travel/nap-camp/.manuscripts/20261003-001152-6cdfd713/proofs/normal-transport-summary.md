# Normal transport and retained limits

The independent normal-mode sustained-429 fixture used a 100 ms timeout. The CLI
failed explicitly with exit 5 in about 0.11 seconds, a Nap Camp source GET context
and empty stdout. The generated normal retry loop can continue until the timeout;
direct classified throttling in dogfood mode exits 7. Neither path returns empty
success data, fabricated availability or price evidence. Regression tests cover
the classified provider failures and timeout boundary.

The publication review identified the earlier generated-client allocation limit:
plain bodies were read before a domain 4 MiB rejection, and compressed inflation
had a separate 32 MiB cap. The published CLI now bounds both response reading and
inflation to 4 MiB, using one extra byte only to detect overflow. Oversized success
or error bodies fail with provider request context and no partial data; the body
is closed without downloading the rest. Exact-size bodies remain valid. Tests
cover absent/inaccurate content lengths, raw-source calls and compressed output.
This per-CLI fix resolves the publication finding without changing the Printing
Press, frozen installed build or global configuration.

The full live matrix retains six framework learning confirm/reject cases as
unverified because no matching local candidate fixture exists. Other skips are
protective or nonapplicable. The learning-disable environment override is not
exported during the full matrix; playbook amend, teach and teach-playbook JSON
dry-run checks were evaluated successfully. The compact row summary records each
status and reason. Raw command responses, host paths, private review logs and
installation records are intentionally excluded from these public manuscripts.
