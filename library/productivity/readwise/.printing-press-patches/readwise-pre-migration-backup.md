# Existing older stores are snapshotted before migration and newly added typed fields are backfilled.

One-way schema upgrades need a recoverable original database; sparse projections must retain JSON values.

This local delta protects Readwise mirror semantics after reprint. The snapshot, search, and backup changes also belong upstream in Printing Press; retain these guards until the generator provides the same contract.

Round 2: harden new and existing backups to 0600, tolerate a competing VACUUM winner, disclose the path on stderr and in help, and include versioned backups in managed-store purge. Custom databases remain manually managed.
