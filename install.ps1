[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$Repo    = "asertym/lvm"
$Binary  = "llava"
$Arch    = if ([Environment]::Is64BitOperatingSystem -and ($env:PROCESSOR_ARCHITECTURE -eq "ARM64" -or $env:PROCESSOR_ARCHITEW6432 -eq "ARM64")) { "arm64" } else { "amd64" }
$Asset   = "llava-windows-$Arch.exe"
$InstallDir = Join-Path $env:USERPROFILE "bin"

# ── helpers ───────────────────────────────────────────────────────────────────
function Write-Green { param($msg) Write-Host "✓ $msg" -ForegroundColor Green }
function Write-Yellow { param($msg) Write-Host "→ $msg" -ForegroundColor Yellow }
function Write-Red   { param($msg) Write-Host "✗ $msg" -ForegroundColor Red }
function Write-Bold  { param($msg) Write-Host $msg -ForegroundColor White }

# ── fetch latest release version ──────────────────────────────────────────────
function Get-LatestVersion
{
    $url = "https://api.github.com/repos/$Repo/releases/latest"
    try
    {
        $release = Invoke-RestMethod -Uri $url -Headers @{ 'User-Agent' = 'llava-installer' }
        return $release.tag_name
    } catch
    {
        Write-Red "Could not fetch latest release from GitHub: $_"
        exit 1
    }
}

# ── download binary ───────────────────────────────────────────────────────────
function Get-Binary
{
    param($Version)

    $url  = "https://github.com/$Repo/releases/download/$Version/$Asset"
    $tmp  = Join-Path $env:TEMP "llava-install.exe"

    Write-Host "Downloading $Binary $Version (windows/amd64)..."

    try
    {
        $client = New-Object System.Net.WebClient
        $client.DownloadFile($url, $tmp)
    } catch
    {
        Write-Red "Download failed: $url"
        Write-Red $_.Exception.Message
        exit 1
    }

    return $tmp
}

# ── install binary ────────────────────────────────────────────────────────────
function Install-Binary
{
    param($TmpPath)

    if (-not (Test-Path $InstallDir))
    {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }

    $dest = Join-Path $InstallDir "$Binary.exe"
    try
    {
        Copy-Item -Path $TmpPath -Destination $dest -Force
        Remove-Item $TmpPath -Force
    } catch
    {
        Write-Red "Install failed: $_"
        exit 1
    }

    Write-Green "Installed $Binary to $dest"
    return $dest
}

# ── add install dir to user PATH (REG_EXPAND_SZ) ──────────────────────────────
function Add-ToPath
{
    $regPath = 'HKCU:\Environment'
    $current = (Get-ItemProperty -Path $regPath -Name PATH -ErrorAction SilentlyContinue).PATH
    if ($null -eq $current) { $current = '' }

    $parts = $current -split ';' | Where-Object { $_ -ne '' }
    if ($parts -contains $InstallDir)
    {
        Write-Green "$InstallDir already in PATH"
        return
    }

    $newPath = ($InstallDir + ';' + ($parts -join ';')).TrimEnd(';')
    try
    {
        Set-ItemProperty -Path $regPath -Name PATH -Value $newPath -Type ExpandString
    } catch
    {
        Write-Red "Could not update PATH: $_"
        return
    }

    Write-Green "Added $InstallDir to user PATH"

    # Broadcast so new terminals pick up the change
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
function Main
{
    Write-Host ""
    Write-Bold "llava — llama.cpp version manager"
    Write-Host ""

    $version = Get-LatestVersion
    $tmp     = Get-Binary -Version $version
    Install-Binary -TmpPath $tmp | Out-Null
    Add-ToPath

    Write-Host ""
    Write-Bold "Done. Open a new terminal, then run:"
    Write-Host "  llava init"
    Write-Host "  llava install latest"
    Write-Host ""
}

Main
