# Build agyx.exe with version metadata stamped in.
#   .\build.ps1            -> version from the latest git tag (or "dev")
#   .\build.ps1 -Version 1.2.0
param(
    [string]$Version = "",
    [string]$Out = "agyx.exe"
)

if ($Version -eq "") {
    $tag = git describe --tags --abbrev=0 2>$null
    if ($LASTEXITCODE -eq 0 -and $tag) { $Version = $tag.TrimStart("v") } else { $Version = "dev" }
}
$commit = git rev-parse --short HEAD 2>$null
if ($LASTEXITCODE -ne 0) { $commit = "" }

$ldflags = "-s -w -X main.version=$Version -X main.commit=$commit"
Write-Host "building agyx $Version ($commit) -> $Out"
go build -trimpath -ldflags $ldflags -o $Out .
