# lvm Cross-Compilation Build Script for Windows
$ErrorActionPreference = "Stop"
$DistDir = "dist"

Write-Host "Building lvm for all platforms..." -ForegroundColor Cyan
Write-Host ""

foreach ($platform in @("Windows-amd64", "Windows-386", "Linux-amd64", "Linux-arm64", "Linux-386", "macOS-amd64", "macOS-arm64")) {
    Write-Host "Building $platform..." -NoNewline
    
    switch ($platform) {
        "Windows-amd64" { $env:GOOS="windows"; $env:GOARCH="amd64"; $ext=".exe" }
        "Windows-386"   { $env:GOOS="windows"; $env:GOARCH="386";   $ext=".exe" }
        "Linux-amd64"   { $env:GOOS="linux";  $env:GOARCH="amd64";  $ext="" }
        "Linux-arm64"   { $env:GOOS="linux";  $env:GOARCH="arm64";  $ext="" }
        "Linux-386"     { $env:GOOS="linux";  $env:GOARCH="386";    $ext="" }
        "macOS-amd64"   { $env:GOOS="darwin"; $env:GOARCH="amd64";  $ext="" }
        "macOS-arm64"   { $env:GOOS="darwin"; $env:GOARCH="arm64";  $ext="" }
    }
    
    try {
        go build -o "$DistDir\lvm-$platform$ext" .
        Write-Host " OK" -ForegroundColor Green
        
        if (Test-Path "$DistDir\lvm-$platform$ext") {
            $kb = [math]::Round((Get-Item "$DistDir\lvm-$platform$ext").Length / 1KB, 2)
            Write-Host "  Size: $($kb) KB" -ForegroundColor Gray
        }
    } catch {
        Write-Host " FAILED ($($_.Exception.Message))" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "Build complete!" -ForegroundColor Green
Write-Host "Output: $DistDir\" -ForegroundColor Cyan

Write-Host "`nBuilt executables:" -ForegroundColor Yellow
Get-ChildItem "$DistDir\*" | Sort-Object Name | ForEach-Object {
    Write-Host "  $($_.Name)" -NoNewline
    if ($_.Length -gt 0) {
        $kb = [math]::Round($_.Length / 1KB, 2)
        Write-Host " ($($kb) KB)" -ForegroundColor Gray
    }
}
