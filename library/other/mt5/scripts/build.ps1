# Build mt5-pp-cli and mt5-pp-mcp with a version stamp.
#
# Usage:
#   .\scripts\build.ps1                  # dev build (0.1.0-dev)
#   .\scripts\build.ps1 -Version v0.1.0  # release build
#   .\scripts\build.ps1 -OutDir .\dist   # output directory (default: bin/)
#
# The version stamp updates github.com/.../internal/cli.Version via -ldflags,
# which the MCP server also reads via mcp.ServerVersion(). One variable, both
# binaries.
[CmdletBinding()]
param(
    [string]$Version = "",
    [string]$OutDir = "bin"
)

$ErrorActionPreference = "Stop"

if (-not $Version) {
    $tag = (git describe --tags --always --dirty 2>$null)
    if ($LASTEXITCODE -eq 0 -and $tag) {
        $Version = $tag
    } else {
        $Version = "0.1.0-dev"
    }
}

$ldflags = "-X github.com/mvanhorn/printing-press-library/library/other/mt5/internal/cli.Version=$Version"

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

Write-Host "Building mt5-pp-cli  version=$Version → $OutDir\mt5-pp-cli.exe"
& go build -ldflags $ldflags -o (Join-Path $OutDir "mt5-pp-cli.exe") ./cmd/mt5-pp-cli
if ($LASTEXITCODE -ne 0) { throw "build mt5-pp-cli failed" }

Write-Host "Building mt5-pp-mcp  version=$Version → $OutDir\mt5-pp-mcp.exe"
& go build -ldflags $ldflags -o (Join-Path $OutDir "mt5-pp-mcp.exe") ./cmd/mt5-pp-mcp
if ($LASTEXITCODE -ne 0) { throw "build mt5-pp-mcp failed" }

Write-Host ""
Write-Host "OK. Verify:"
Write-Host "  $OutDir\mt5-pp-cli.exe --version"
Write-Host "  $OutDir\mt5-pp-mcp.exe --version"
