# Atlassian Platform Connector -- Windows Installer
# Usage: irm https://raw.githubusercontent.com/jinkp/atlassian-go-mcp/main/install.ps1 | iex

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# -- Config -------------------------------------------------------------------
$Repo       = "jinkp/atlassian-go-mcp"
$InstallDir = Join-Path $env:USERPROFILE ".mcp\atlassian"
$Binaries   = @("atlassian.exe", "atlassian-mcp.exe", "atlassian-api.exe")

# -- Helpers ------------------------------------------------------------------
function Write-Step { param($msg) Write-Host "  -> $msg" -ForegroundColor Cyan }
function Write-Ok   { param($msg) Write-Host "  OK $msg" -ForegroundColor Green }
function Write-Warn { param($msg) Write-Host "  !! $msg" -ForegroundColor Yellow }
function Write-Fail { param($msg) Write-Host "  XX $msg" -ForegroundColor Red }

function Get-LatestVersion {
    $api = "https://api.github.com/repos/$Repo/releases/latest"
    $rel = Invoke-RestMethod -Uri $api -Headers @{ "User-Agent" = "atlassian-installer" }
    return $rel.tag_name
}

function Add-ToUserPath {
    param([string]$Dir)
    $currentPath = [Environment]::GetEnvironmentVariable("PATH", [System.EnvironmentVariableTarget]::User)
    $parts = $currentPath -split ";"
    if ($parts -contains $Dir) { return $false }
    $newPath = ($parts + $Dir) -join ";"
    [Environment]::SetEnvironmentVariable("PATH", $newPath, [System.EnvironmentVariableTarget]::User)
    return $true
}

# -- Banner -------------------------------------------------------------------
Write-Host ""
Write-Host "  Atlassian Platform Connector" -ForegroundColor White
Write-Host "  ----------------------------" -ForegroundColor DarkGray
Write-Host "  CLI + MCP Server + REST API for Atlassian Cloud" -ForegroundColor DarkGray
Write-Host ""

# -- Detect latest version ----------------------------------------------------
Write-Step "Fetching latest release from GitHub..."
try {
    $Version = Get-LatestVersion
    Write-Ok "Latest version: $Version"
} catch {
    Write-Fail "Could not fetch release: $_"
    exit 1
}

# -- Check for existing installation -----------------------------------------
$existingVersion = $null
$mcpBin = Join-Path $InstallDir "atlassian-mcp.exe"
if (Test-Path $mcpBin) {
    Write-Step "Existing installation detected — upgrading"
}

# -- Create install directory -------------------------------------------------
Write-Step "Installing to: $InstallDir"
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# -- Download binaries --------------------------------------------------------
$downloaded = @()
foreach ($bin in $Binaries) {
    $url  = "https://github.com/$Repo/releases/download/$Version/$bin"
    $dest = Join-Path $InstallDir $bin
    $tmp  = "$dest.tmp"

    Write-Step "Downloading $bin..."
    try {
        Invoke-WebRequest -Uri $url -OutFile $tmp -UseBasicParsing
        Move-Item -Path $tmp -Destination $dest -Force
        $sizeMB = [math]::Round((Get-Item $dest).Length / 1MB, 1)
        Write-Ok "$bin ($sizeMB MB)"
        $downloaded += $bin
    } catch {
        Write-Fail "Failed to download ${bin}: $_"
        if (Test-Path $tmp) { Remove-Item $tmp -Force }
    }
}

if ($downloaded.Count -eq 0) {
    Write-Fail "No binaries were downloaded. Aborting."
    exit 1
}

# -- Add to PATH --------------------------------------------------------------
Write-Step "Updating user PATH..."
try {
    $added = Add-ToUserPath -Dir $InstallDir
    if ($added) {
        Write-Ok "Added to PATH: $InstallDir"
        Write-Warn "Restart your terminal for PATH changes to take effect"
    } else {
        Write-Ok "Already in PATH"
    }
} catch {
    Write-Warn "Could not update PATH automatically: $_"
    Write-Warn "Add manually: $InstallDir"
}

# Update current session PATH so commands work immediately
if ($env:PATH -notlike "*$InstallDir*") {
    $env:PATH = "$env:PATH;$InstallDir"
}

# -- Summary ------------------------------------------------------------------
Write-Host ""
Write-Host "  Installed ($Version):" -ForegroundColor White
foreach ($bin in $downloaded) {
    $path = Join-Path $InstallDir $bin
    Write-Host "    $bin" -ForegroundColor Green
    Write-Host "    $path" -ForegroundColor DarkGray
}

# -- Next steps ---------------------------------------------------------------
Write-Host ""
Write-Host "  Quick setup (recommended):" -ForegroundColor White
Write-Host ""
Write-Host "    atlassian-mcp tui" -ForegroundColor Green
Write-Host ""
Write-Host "    The interactive TUI guides you through:" -ForegroundColor DarkGray
Write-Host "      1. Select which modules to enable (jira, agile, goals, etc.)" -ForegroundColor Gray
Write-Host "      2. Enter your Atlassian credentials" -ForegroundColor Gray
Write-Host "      3. Test connectivity to each service" -ForegroundColor Gray
Write-Host "      4. Register the MCP server into your AI client" -ForegroundColor Gray
Write-Host ""
Write-Host "  Manual setup:" -ForegroundColor White
Write-Host ""
Write-Host "  1. Set your credentials:" -ForegroundColor DarkGray
Write-Host '     $env:ATLASSIAN_BASE_URL = "https://your-org.atlassian.net"' -ForegroundColor Gray
Write-Host '     $env:ATLASSIAN_EMAIL    = "you@company.com"' -ForegroundColor Gray
Write-Host '     $env:ATLASSIAN_TOKEN    = "your-api-token"' -ForegroundColor Gray
Write-Host '     # Get a token: https://id.atlassian.com/manage-profile/security/api-tokens' -ForegroundColor DarkGray
Write-Host ""
Write-Host "  2. Register MCP server into your AI client:" -ForegroundColor DarkGray
Write-Host "     atlassian-mcp setup opencode          # OpenCode" -ForegroundColor Gray
Write-Host "     atlassian-mcp setup claude             # Claude Code (CLI)" -ForegroundColor Gray
Write-Host "     atlassian-mcp setup claude-desktop     # Claude Desktop" -ForegroundColor Gray
Write-Host "     atlassian-mcp setup cursor             # Cursor" -ForegroundColor Gray
Write-Host ""
Write-Host "  3. Try the CLI:" -ForegroundColor DarkGray
Write-Host '     atlassian jira search --jql "project=PROJ"' -ForegroundColor Gray
Write-Host "     atlassian agile sprint active --board-id 10" -ForegroundColor Gray
Write-Host "     atlassian goals search --site-id <cloudId>" -ForegroundColor Gray
Write-Host ""
Write-Host "  Docs: https://github.com/$Repo" -ForegroundColor DarkGray
Write-Host ""
Write-Host "  Installation complete!" -ForegroundColor Green
Write-Host ""
