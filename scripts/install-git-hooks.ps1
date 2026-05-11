$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
try {
  git config core.hooksPath .githooks
  Write-Host "Installed repo-local git hooks: core.hooksPath=.githooks"
} finally {
  Pop-Location
}
