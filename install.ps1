# lvm installer — Windows (PowerShell)
# Usage: irm https://github.com/YOURNAME/lvm/releases/latest/download/install.ps1 | iex

param()
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$Repo    = "YOURNAME/lvm"
$Binary  = "lvm"
$Asset   = "lvm-windows-amd64.exe"
$InstallDir = Join-Path $env:USERPROFILE "bin"

# ── helpers ───────────────────────────────────────────────────────────────────
function Write-Green  { param($msg) Write-Host "✓ $msg" -ForegroundColor Green }
function Write-Yellow { param($msg) Write-Host "→ $msg" -ForegroundColor Yellow }
function Write-Red    { param($msg) Write-Host "✗ $msg" -ForegroundColor Red }
function Write-Bold   { param($msg) Write-Host $msg -ForegroundColor White }

# ── fetch latest release version ──────────────────────────────────────────────
function Get-LatestVersion {
  $url = "https://api.github.com/repos/$Repo/releases/latest"
  try {
    $release = Invoke-RestMethod -Uri $url -Headers @{ 'User-Agent' = 'lvm-installer' }
    return $release.tag_name
  } catch {
    Write-Red "Could not fetch latest release from GitHub: $_"
    exit 1
  }
}

# ── download binary ───────────────────────────────────────────────────────────
function Get-Binary {
  param($Version)

  $url  = "https://github.com/$Repo/releases/download/$Version/$Asset"
  $tmp  = Join-Path $env:TEMP "lvm-install.exe"

  Write-Host "Downloading $Binary $Version (windows/amd64)..."

  try {
    $client = New-Object System.Net.WebClient
    $client.DownloadFile($url, $tmp)
  } catch {
    Write-Red "Download failed: $url"
    Write-Red $_.Exception.Message
    exit 1
  }

  return $tmp
}

# ── install binary ────────────────────────────────────────────────────────────
function Install-Binary {
  param($TmpPath)

  if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
  }

  $dest = Join-Path $InstallDir "$Binary.exe"
  Copy-Item -Path $TmpPath -Destination $dest -Force
  Remove-Item $TmpPath -Force

  Write-Green "Installed $Binary to $dest"
  return $dest
}

# ── add install dir to user PATH (REG_EXPAND_SZ) ─────────────────────────────
function Add-ToPath {
  $regPath = 'HKCU:\Environment'
  $current = (Get-ItemProperty -Path $regPath -Name PATH -ErrorAction SilentlyContinue).PATH
  if ($null -eq $current) { $current = '' }

  # Already present?
  $parts = $current -split ';' | Where-Object { $_ -ne '' }
  if ($parts -contains $InstallDir) {
    Write-Green "$InstallDir already in PATH"
    return
  }

  # Prepend and write back as REG_EXPAND_SZ so it survives new terminals.
  $newPath = ($InstallDir + ';' + ($parts -join ';')).TrimEnd(';')
  Set-ItemProperty -Path $regPath -Name PATH -Value $newPath -Type ExpandString

  Write-Green "Added $InstallDir to user PATH"

  # Broadcast the change so new terminals pick it up without a full logoff.
  $signature = @'
[DllImport("user32.dll", SetLastError=true, CharSet=CharSet.Auto)]
public static extern IntPtr SendMessageTimeout(
  IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam,
  uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);
'@
  $type = Add-Type -MemberDefinition $signature -Name WinEnv -Namespace Win32 -PassThru
  $result = [UIntPtr]::Zero
  $type::SendMessageTimeout(
    [IntPtr]0xffff, 0x001A, [UIntPtr]::Zero,
    'Environment', 2, 5000, [ref]$result
  ) | Out-Null

  Write-Yellow "Open a new terminal for PATH to take effect"
}

# ── main ──────────────────────────────────────────────────────────────────────
function Main {
  Write-Host ""
  Write-Bold "lvm — llama.cpp version manager"
  Write-Host ""

  $version = Get-LatestVersion
  $tmp     = Get-Binary -Version $version
  Install-Binary -TmpPath $tmp | Out-Null
  Add-ToPath

  Write-Host ""
  Write-Bold "Done. Open a new terminal, then run:"
  Write-Host "  lvm init"
  Write-Host "  lvm install latest"
  Write-Host ""
}

Main
