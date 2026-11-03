# Profile credential review fixes

Two valid findings in the new generator framework were fixed locally:

- Explicit-profile logout now persists a non-secret global-fallback suppression flag under the profile credential lock before deleting the profile credential file. Other global credentials remain intact. Explicit sign-in clears the flag. Exported environment credentials retain their separately reported behavior. Regression tests cover profiles with their own key and profiles that inherited a same-server global key, reload after logout, unchanged global access, and successful new sign-in.
- Token-save messages and JSON now derive their credential destination from the same Config method used for storage. A regression test verifies the reported explicit-profile file matches the actual saved file and differs from the global file.

Full unit suite and go vet passed. Full live matrix passed 186/186 executed checks; 136 skipped/unverified remain separately recorded. Publish validation including reachable govulncheck passed.
