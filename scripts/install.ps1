# Transparent WSL app installation script for Windows (non-admin)
# Usage: powershell -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 [-Version v1.2.3]

[CmdletBinding()]
param(
  [string]$Version = ""  # -Version v1.2.3 ; empty = latest
)

# Template variables ----------------------------------------------------------

$Owner   = "Data-Corruption"
$Repo    = "sprout"

$AppName = "sprout"
$Service = $true

# -----------------------------------------------------------------------------

$ErrorActionPreference = "Stop"

function Fail([string]$msg, [int]$code = 1) { $host.UI.WriteErrorLine("$msg"); [Environment]::Exit($code) }
function Info($msg) { Write-Host $msg }

# ensure WSL is installed and a default distro exists
try { $null = & wsl.exe --status 2>&1 } catch { Fail "WSL not installed/enabled. Enable WSL and install a distro, then re-run." 1 }
# ensure it's running
try { $null = & wsl.exe -e true } catch { Fail "Failed to start WSL. Open a WSL shell once, then re-run." 1 }

# if service, ensure systemd is enabled
if ($Service) {
  try {
    $out = & wsl.exe -e sh -lc 'systemctl --user --version 2>/dev/null | head -n1'
  } catch {
      Fail @"
Failed to check systemd status. To enable WSL systemd (user), follow:
1) In WSL:  sudo sh -c 'printf "[boot]\nsystemd=true\n" >> /etc/wsl.conf'
2) In Windows:  wsl --shutdown
3) Re-open your WSL distro and re-run this installer.

If systemd remains disabled, user services may not work and service installation can fail.
"@ 1
  }
}

# run linux install script in WSL
$linuxInstallCmd = if ([string]::IsNullOrWhiteSpace($Version)) {
  "curl -fsSL https://raw.githubusercontent.com/$Owner/$Repo/main/scripts/install.sh | sh"
} else {
  "curl -fsSL https://raw.githubusercontent.com/$Owner/$Repo/main/scripts/install.sh | sh -s -- $Version"
}
Info "Running Linux installer inside WSL..."
& $env:SystemRoot\System32\wsl.exe -e /bin/sh -lc $linuxInstallCmd
Write-Host "" # spacer
if ($LASTEXITCODE -ne 0) { Fail "Linux install command failed with exit code $LASTEXITCODE." $LASTEXITCODE }

# create windows shim
$shimRoot = Join-Path $env:LOCALAPPDATA "Programs"
$shimDir  = Join-Path $shimRoot $AppName
New-Item -ItemType Directory -Force -Path $shimDir | Out-Null

$shimPathCmd = Join-Path $shimDir "$AppName.cmd"
$shimContent = @"
@echo off
setlocal
set "WSL=%SystemRoot%\System32\wsl.exe"
set "APP=%~n0"
"%WSL%" -e /bin/sh -lc "exec %APP% \"$@\"" -- %*
endlocal
"@
Set-Content -Path $shimPathCmd -Value $shimContent -Encoding ASCII

# ensure shim dir on USER PATH ------------------------------------------------

function PathSplit([string]$path) {
  if ([string]::IsNullOrEmpty($path)) { @() } else { $path.Split(';') | Where-Object { $_ -ne "" } }
}
function PathHas([string]$path, [string]$dir) {
  (PathSplit $path | Where-Object { $_.TrimEnd('\') -ieq $dir.TrimEnd('\') }) -ne $null
}

$userPath = [Environment]::GetEnvironmentVariable("PATH","User")
if (-not (PathHas $userPath $shimDir)) {
  $newUserPath = if ([string]::IsNullOrEmpty($userPath)) { $shimDir } else { "$userPath;$shimDir" }
  [Environment]::SetEnvironmentVariable("PATH", $newUserPath, "User")  # persists for the user
  # also update current session so this shell can use it immediately
  if (-not (PathHas $env:PATH $shimDir)) { $env:PATH = "$env:PATH;$shimDir" }
  Write-Host "Added to user PATH: $shimDir"
  Write-Host "Open a new terminal for other shells to pick it up."
}

if ($Service) {
  Write-Host ""
  Write-Host "Note! to manage the service you'll need to enter WSL first via 'wsl'"
  Write-Host "Otherwise freely use the app as a native Windows cli application."
}