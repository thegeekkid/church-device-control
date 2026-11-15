$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$dist = Join-Path $root 'dist'
New-Item -ItemType Directory -Force $dist | Out-Null

# GitHub supplies source archives from the release tag. Distribute the two
# Windows executables directly, with their notices and post-signing checksums.
Copy-Item (Join-Path $root 'build/Church-TV-Control.exe') (Join-Path $dist 'Church-TV-Control.exe') -Force
Copy-Item (Join-Path $root 'LICENSE') (Join-Path $dist 'LICENSE.txt') -Force
Copy-Item (Join-Path $root 'THIRD-PARTY-NOTICES.txt') $dist -Force
$files = @('Church-TV-Control.exe', 'Church-Device-Control-Setup.exe', 'LICENSE.txt', 'THIRD-PARTY-NOTICES.txt')
$checksums = foreach ($name in $files) {
    $hash = (Get-FileHash (Join-Path $dist $name) -Algorithm SHA256).Hash.ToLowerInvariant()
    "$hash  $name"
}
Set-Content (Join-Path $dist 'SHA256.txt') $checksums -Encoding ascii
