# Surfline run — retro candidates (generator issues)

1. **auth.Optional not propagated to doctor env-var requiredness.**
   Setting `auth: {optional: true}` in the internal YAML spec updates the
   doctor "Auth" line to "optional — not configured" but the env-var block
   still routes a missing token to `authEnvRequiredMissing`, producing
   `FAIL Env Vars: ERROR missing required: <VAR>`. For an optional-auth API
   this is a false failure. Patched doctor.go by hand (env-var else-branch →
   authEnvInfo). The generator should honor Auth.Optional in the env-var
   requiredness template, not just the auth-status line.

2. **Novel feature named "search" replaced the framework `search` command.**
   research.json novel_features included command "search" (offline FTS). The
   generator emitted internal/cli/search.go as a TODO stub named
   newNovelSearchCmd, shadowing the rich framework search command. A novel
   feature whose command collides with a framework command name should either
   be renamed or layered on top of the framework command, not replace it with
   a stub.
