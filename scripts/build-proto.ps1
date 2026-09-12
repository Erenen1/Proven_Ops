param()

$protocPath = "C:\Users\ern\AppData\Local\Microsoft\WinGet\Packages\Google.Protobuf_Microsoft.Winget.Source_8wekyb3d8bbwe\bin\protoc.exe"
$goBin = "C:\Users\ern\go\bin"
$goInstall = "C:\Program Files\Go\bin"

$env:PATH = "$goBin;$goInstall;$env:PATH"

if (-not (Test-Path "proto\v1")) {
    New-Item -ItemType Directory -Force -Path "proto\v1" | Out-Null
}

& $protocPath --proto_path=proto --go_out=proto/v1 --go_opt=paths=source_relative --go-grpc_out=proto/v1 --go-grpc_opt=paths=source_relative proto/agent.proto
if ($LASTEXITCODE -eq 0) {
    Write-Host "Protobuf compiled successfully into proto/v1"
} else {
    Write-Error "Protobuf compilation failed with exit code $LASTEXITCODE"
}
