# Private mirror writes
Treat export-dir as a filesystem write destination for MCP. Private temporary files and rename prevent following a pre-existing output-file symlink. Preserve permissions of directories the user created.
