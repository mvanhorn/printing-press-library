/**
 * Platform- and shell-aware instructions for adding a binary directory to PATH.
 * Used when a freshly installed CLI binary is not yet discoverable by name — either
 * because it was written to the default user bin directory (`~/.local/bin` on
 * macOS/Linux, `%LOCALAPPDATA%\\Programs\\PrintingPress\\bin` on Windows) or to Go's
 * `$(go env GOPATH)/bin`. Builds the exact, copy-pasteable fix for the detected
 * (platform, shell) rather than a single Unix-flavored hint that is wrong on
 * Windows and imprecise on fish.
 *
 * Pure and dependency-free so the full (platform × shell) matrix is unit-testable.
 */

export interface PathFixContext {
  /** Resolved `go install` directory (GOBIN or GOPATH/bin). Null when `go env` couldn't resolve it. */
  binDir: string | null;
  platform: NodeJS.Platform;
  /** process.env.SHELL — the login shell on Unix; set under Git Bash/MSYS on Windows. */
  shell?: string;
  /** process.env.HOME ?? process.env.USERPROFILE — used to prefer portable $HOME/... forms. */
  home?: string;
}

type ShellKind = "zsh" | "bash" | "fish" | "gitbash" | "windows" | "unknown";

function detectShell(platform: NodeJS.Platform, shell?: string): ShellKind {
  const s = (shell ?? "").toLowerCase();
  if (platform === "win32") {
    // The only Windows shell we can reliably detect from a Node grandchild of npx
    // is Git Bash / MSYS, which sets SHELL to a bash path. pwsh vs cmd is not
    // reliably distinguishable here, so we emit a combined Windows block instead
    // of guessing and printing the wrong syntax.
    return s.includes("bash") ? "gitbash" : "windows";
  }
  if (s.includes("zsh")) return "zsh";
  if (s.includes("fish")) return "fish";
  if (s.includes("bash")) return "bash";
  return "unknown";
}

/**
 * The PATH entry to print for a Unix shell: portable `$HOME/...` forms for known
 * per-user defaults, else the literal path.
 * fish passes its own `nullFallback` because its command-substitution syntax
 * `(...)` differs from bash's `$(...)`.
 */
function unixPathEntry(
  binDir: string | null,
  home?: string,
  nullFallback = "$(go env GOPATH)/bin",
): string {
  if (!binDir) return nullFallback;
  if (home && binDir === `${home}/.local/bin`) return "$HOME/.local/bin";
  if (home && binDir === `${home}/go/bin`) return "$HOME/go/bin";
  return binDir;
}

/** C:\Users\you\go\bin -> /c/Users/you/go/bin (Git Bash / MSYS path form). */
function toPosixPath(winPath: string): string {
  const m = winPath.match(/^([A-Za-z]):[\\/](.*)$/);
  if (!m) return winPath.replace(/\\/g, "/");
  return `/${m[1]!.toLowerCase()}/${m[2]!.replace(/\\/g, "/")}`;
}

function rcFile(kind: "zsh" | "bash", platform: NodeJS.Platform): string {
  if (kind === "zsh") return "~/.zshrc";
  // bash login shells on macOS read .bash_profile, not .bashrc; Linux desktop
  // terminals start interactive non-login shells that read .bashrc.
  return platform === "darwin" ? "~/.bash_profile" : "~/.bashrc";
}

/**
 * Returns a copy-pasteable instruction block (no leading/trailing newline) telling
 * the user how to add `binDir` to PATH for their detected platform and shell.
 */
export function pathFixInstructions(ctx: PathFixContext): string {
  const kind = detectShell(ctx.platform, ctx.shell);

  if (kind === "windows") {
    const dir = ctx.binDir ?? "%USERPROFILE%\\go\\bin";
    return [
      "PowerShell (persistent — recommended), then open a new terminal:",
      `    [Environment]::SetEnvironmentVariable("Path", [Environment]::GetEnvironmentVariable("Path","User") + ";${dir}", "User")`,
      "",
      "Current session only:",
      `    $env:Path += ";${dir}"        # PowerShell`,
      `    set PATH=%PATH%;${dir}        # cmd.exe`,
      "",
      'Or via the GUI: press Win, search "environment variables", open',
      `"Edit environment variables for your account", and add ${dir} to Path.`,
    ].join("\n");
  }

  if (kind === "gitbash") {
    const posix = ctx.binDir ? toPosixPath(ctx.binDir) : "$(go env GOPATH)/bin";
    return [
      "Add it to PATH (Git Bash):",
      `    echo 'export PATH="${posix}:$PATH"' >> ~/.bashrc && source ~/.bashrc`,
    ].join("\n");
  }

  if (kind === "fish") {
    const entry = unixPathEntry(ctx.binDir, ctx.home, "(go env GOPATH)/bin");
    return ["Add it to PATH (fish):", `    fish_add_path ${entry}`].join("\n");
  }

  const entry = unixPathEntry(ctx.binDir, ctx.home);
  if (kind === "unknown") {
    return [
      "Add it to PATH by adding this line to your shell's startup file",
      "(e.g. ~/.zshrc or ~/.bashrc), then restart your shell:",
      `    export PATH="${entry}:$PATH"`,
    ].join("\n");
  }

  const file = rcFile(kind, ctx.platform);
  return [
    `Add it to PATH (${kind}):`,
    `    echo 'export PATH="${entry}:$PATH"' >> ${file} && source ${file}`,
  ].join("\n");
}
