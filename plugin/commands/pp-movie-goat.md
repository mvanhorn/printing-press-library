---
description: "Printing Press CLI for TMDb + OMDb. Multi-source movie ratings (TMDb + OMDb), streaming availability, and cross-taste recommendations Capabilities include: analytics, blind, career, discover, feedback, genres, marathon, movies, people, profile, recommend-for-me, search, tail, tonight, trending, tv, versus, watch. Trigger phrases: 'install movie-goat', 'use movie-goat', 'run movie-goat', 'TMDb + OMDb commands', 'setup movie-goat'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
---

Invoke the `pp-movie-goat` skill with the user's arguments: $ARGUMENTS

If the user passes no arguments or just `help`, show `movie-goat-pp-cli --help`. If the user's arguments start with `install`, follow the skill's install flow. Otherwise, map the arguments to the best `movie-goat-pp-cli` command and execute.
