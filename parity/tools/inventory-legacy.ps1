param(
    [string]$LegacyRoot = 'D:\ABUZAR\V2_AbuzarSoftware\Application',
    [string]$Output = 'D:\ABUZAR\AbuzarNext\parity\catalog\legacy-module-inventory.json'
)

$resolvedRoot = (Resolve-Path -LiteralPath $LegacyRoot -ErrorAction Stop).Path
$resolvedOutput = [System.IO.Path]::GetFullPath($Output)
$outputDirectory = Split-Path -Parent $resolvedOutput
New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null

# This is a metadata-only inventory. It reads names and sizes from the legacy
# install; it does not open databases, copy binaries, or modify the reference.
$files = Get-ChildItem -LiteralPath $resolvedRoot -Recurse -File -ErrorAction Stop |
    Where-Object { $_.Extension -in @('.pbd', '.exe', '.bmp', '.png', '.ico', '.wav', '.pbx') } |
    Sort-Object FullName |
    ForEach-Object {
        $relative = $_.FullName.Substring($resolvedRoot.Length).TrimStart('\', '/')
        [pscustomobject]@{
            relativePath = $relative
            extension = $_.Extension.ToLowerInvariant()
            length = $_.Length
            lastWriteTime = $_.LastWriteTimeUtc.ToString('o')
        }
    }

$manifest = [pscustomobject]@{
    generatedAt = [DateTime]::UtcNow.ToString('o')
    source = 'redacted legacy application path'
    files = @($files)
}
$json = $manifest | ConvertTo-Json -Depth 5
[System.IO.File]::WriteAllText($resolvedOutput, $json, [System.Text.UTF8Encoding]::new($false))
Write-Output "Wrote metadata inventory for $($manifest.files.Count) legacy artifacts to $resolvedOutput"
