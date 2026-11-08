param([switch]$Signed)
$ErrorActionPreference = 'Stop'
$package = Join-Path $PSScriptRoot '..\dist\Church-TV-Control'
New-Item -ItemType Directory -Force $package | Out-Null
Copy-Item (Join-Path $PSScriptRoot '..\build\Church-TV-Control.exe') $package
$text = Get-Content (Join-Path $PSScriptRoot '..\README.txt') -Raw
if ($Signed) {
    $text = $text.Replace('This build is unsigned.', 'This build is signed using Azure Artifact Signing.')
}
Set-Content (Join-Path $package 'START-HERE.txt') $text -Encoding utf8
$source = Join-Path $package 'source'
New-Item -ItemType Directory -Force $source | Out-Null
Get-ChildItem (Join-Path $PSScriptRoot '..') -File | Where-Object { $_.Extension -in '.go','.mod','.syso' } | Copy-Item -Destination $source
Copy-Item (Join-Path $PSScriptRoot '..\winres') $source -Recurse
Copy-Item (Join-Path $PSScriptRoot '..\tools') $source -Recurse
Copy-Item (Join-Path $PSScriptRoot '..\scripts') $source -Recurse
Copy-Item (Join-Path $PSScriptRoot '..\docs') $source -Recurse
Copy-Item (Join-Path $PSScriptRoot '..\README.txt') $source
$hash = (Get-FileHash (Join-Path $package 'Church-TV-Control.exe') -Algorithm SHA256).Hash.ToLower()
$checksum = "$hash  Church-TV-Control.exe"
Set-Content (Join-Path $package 'SHA256.txt') $checksum -Encoding ascii
Set-Content (Join-Path $PSScriptRoot '..\dist\SHA256.txt') $checksum -Encoding ascii
Compress-Archive -Path $package -DestinationPath (Join-Path $PSScriptRoot '..\dist\Church-TV-Control.zip') -Force
