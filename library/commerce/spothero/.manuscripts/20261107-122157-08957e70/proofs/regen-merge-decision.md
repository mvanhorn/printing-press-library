# Promote path decision

regen-merge dry-run reports 24 NOVEL files: the old hand-written internal/auth, internal/client, internal/output and internal/cli command files. All of them are ported into the fresh tree: internal/client to internal/consumer, internal/auth to internal/session and internal/chromecookies, internal/cli book, cancel, account and search to internal/cli/spothero_cmds.go (search renamed find).

Applying regen-merge would put the old `package client` files into the generated internal/client package, with the old module path in their imports, which cannot compile, and would restore the checkout and refund shapes this reprint replaced. So the run used the swap, with .git backed up and restored and a full tarball of the old tree under runstate. The old files remain in git history.
