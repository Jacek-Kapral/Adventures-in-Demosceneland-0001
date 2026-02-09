$goPath = Join-Path $env:USERPROFILE "go"
$goModCache = Join-Path $goPath "pkg\mod"

[Environment]::SetEnvironmentVariable("GOPATH", $goPath, "User")
$env:GOPATH = $goPath

if (-not (Test-Path $goModCache)) {
    New-Item -ItemType Directory -Path $goModCache -Force
}

go env -w "GOMODCACHE=$goModCache"

Write-Host "GOPATH (User) = $goPath"
Write-Host "GOMODCACHE (go env -w) = $goModCache"
