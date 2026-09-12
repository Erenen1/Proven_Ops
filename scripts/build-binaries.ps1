param()

$go = "C:\Program Files\Go\bin\go.exe"
if (-not (Test-Path "bin")) {
    New-Item -ItemType Directory -Force -Path "bin" | Out-Null
}

Write-Host "Building Control Plane (Windows)..."
Push-Location apps/control-plane
& $go build -ldflags="-s -w" -o ../../bin/control-plane.exe ./cmd/server
Pop-Location

Write-Host "Building Agent (Windows)..."
Push-Location agent
& $go build -ldflags="-s -w" -o ../bin/opspilot-agent.exe ./cmd/agent
Pop-Location

Write-Host "Building Agent (Linux amd64)..."
Push-Location agent
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
& $go build -ldflags="-s -w" -o ../bin/opspilot-agent ./cmd/agent
$env:GOOS = ""
$env:GOARCH = ""
Pop-Location

Write-Host "All binaries built successfully in ./bin:"
Get-ChildItem "bin"
