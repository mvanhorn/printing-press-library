import test from "node:test";
import assert from "node:assert/strict";
import { pathFixInstructions, type PathFixContext } from "../src/pathfix.js";

const MAC_HOME = "/Users/amosclaw";
const MAC_BIN = `${MAC_HOME}/go/bin`;
const MAC_LOCAL_BIN = `${MAC_HOME}/.local/bin`;
const WIN_BIN = "C:\\Users\\you\\go\\bin";

function ctx(overrides: Partial<PathFixContext>): PathFixContext {
  return { binDir: MAC_BIN, platform: "darwin", home: MAC_HOME, ...overrides };
}

test("macOS zsh recommends ~/.zshrc with the portable $HOME form", () => {
  const out = pathFixInstructions(ctx({ shell: "/bin/zsh" }));
  assert.match(out, /\(zsh\)/);
  assert.match(out, /~\/\.zshrc/);
  assert.match(out, /export PATH="\$HOME\/go\/bin:\$PATH"/);
  assert.match(out, /source ~\/\.zshrc/);
});

test("macOS zsh recommends portable $HOME form for the default user bin dir", () => {
  const out = pathFixInstructions(ctx({ shell: "/bin/zsh", binDir: MAC_LOCAL_BIN }));
  assert.match(out, /export PATH="\$HOME\/\.local\/bin:\$PATH"/);
  assert.doesNotMatch(out, new RegExp(MAC_HOME.replaceAll("/", "\\/")));
});

test("macOS bash recommends ~/.bash_profile, not ~/.bashrc", () => {
  const out = pathFixInstructions(ctx({ shell: "/bin/bash" }));
  assert.match(out, /~\/\.bash_profile/);
  assert.doesNotMatch(out, /\.bashrc/);
});

test("Linux bash recommends ~/.bashrc", () => {
  const out = pathFixInstructions(
    ctx({ platform: "linux", shell: "/usr/bin/bash", home: "/home/dev", binDir: "/home/dev/go/bin" }),
  );
  assert.match(out, /~\/\.bashrc/);
  assert.doesNotMatch(out, /\.bash_profile/);
});

test("fish uses fish_add_path, never bash export syntax", () => {
  const out = pathFixInstructions(ctx({ shell: "/opt/homebrew/bin/fish", binDir: MAC_LOCAL_BIN }));
  assert.match(out, /fish_add_path \$HOME\/\.local\/bin/);
  assert.doesNotMatch(out, /export PATH/);
});

test("custom GOPATH prints the literal dir, not $HOME/go/bin", () => {
  const out = pathFixInstructions(ctx({ shell: "/bin/zsh", binDir: "/opt/tools/bin" }));
  assert.match(out, /export PATH="\/opt\/tools\/bin:\$PATH"/);
  assert.doesNotMatch(out, /\$HOME\/go\/bin/);
});

test("unknown Unix shell falls back to generic guidance", () => {
  const out = pathFixInstructions(ctx({ shell: "/bin/ksh" }));
  assert.match(out, /shell's startup file/);
  assert.match(out, /export PATH="\$HOME\/go\/bin:\$PATH"/);
});

test("Windows (no detectable shell) gives the persistent SetEnvironmentVariable command", () => {
  const out = pathFixInstructions({ binDir: WIN_BIN, platform: "win32", home: "C:\\Users\\you" });
  assert.match(out, /SetEnvironmentVariable/);
  assert.match(out, /"User"\)/);
  assert.ok(out.includes(WIN_BIN), "includes the literal Windows bin dir");
  assert.match(out, /environment variables/i); // GUI fallback present
  assert.doesNotMatch(out, /setx/); // never recommend the truncating setx footgun
});

test("Windows Git Bash translates the path to POSIX form and edits ~/.bashrc", () => {
  const out = pathFixInstructions({
    binDir: WIN_BIN,
    platform: "win32",
    shell: "C:\\Program Files\\Git\\usr\\bin\\bash.exe",
    home: "C:\\Users\\you",
  });
  assert.match(out, /Git Bash/);
  assert.match(out, /\/c\/Users\/you\/go\/bin/);
  assert.match(out, /~\/\.bashrc/);
});

test("missing binDir falls back to $(go env GOPATH)/bin on Unix", () => {
  const out = pathFixInstructions({ binDir: null, platform: "darwin", shell: "/bin/zsh" });
  assert.match(out, /\$\(go env GOPATH\)\/bin/);
});

test("missing binDir falls back to %USERPROFILE%\\go\\bin on Windows", () => {
  const out = pathFixInstructions({ binDir: null, platform: "win32" });
  assert.ok(out.includes("%USERPROFILE%\\go\\bin"));
});
