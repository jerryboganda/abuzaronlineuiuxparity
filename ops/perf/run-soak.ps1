param(
    [double]$DurationHours = 8,
    [int]$IntervalSeconds = 5,
    [string]$ApiBaseUrl = 'http://127.0.0.1:8080',
    [string]$SessionCookie = $env:ABUZAR_PERF_SESSION_COOKIE,
    [string]$ItemLegacyId = 'ITEM-000001',
    [string]$GodownId = '90000000-0000-0000-0000-000000000201',
    [string]$OutputPath = '',

    # --- Write-load mode (opt-in; the default remains exactly the safe,
    # read-only behavior above). See ops/perf/README.md before enabling. ---
    [switch]$EnableWriteTraffic,
    [switch]$ConfirmTestTenant,
    [string]$WriteTenantCode = 'demo',
    [string]$WriteUsername = 'admin',
    [string]$WritePassword = $env:ABUZAR_PERF_WRITE_PASSWORD,
    [string]$WriteBranchId = '',
    [string]$WriteCounterId = '',
    [string]$WriteDocumentKind = 'loose-purchase',
    [int]$WriteIntervalSeconds = 10,
    [int]$WriteBatchSize = 1,
    [int]$WriteConcurrency = 1,
    [int]$WriteMaxDocuments = 0,
    [string]$WriteItemSearch = 'DEMO',
    [string]$WriteSupplierCode = 'SOAK-WRITE-SUPPLIER',
    [string]$WriteGodownCode = '',
    [string]$WriteReferencePrefix = 'SOAK-WRITE',
    [string]$WriteManifestPath = '',

    # --- Cleanup mode: void every document recorded in a write-mode manifest
    # from a previous run, then exit. Requires the same tenant safety gate. ---
    [switch]$VoidWriteTraffic,

    # --- Monitoring / alerting (Part B). Postgres monitoring is skipped
    # (with a note) when -PgMonitorConnectionString is not supplied or the
    # psql client is not on PATH; host monitoring always runs. ---
    [string]$PgMonitorConnectionString = $env:ABUZAR_PERF_PG_MONITOR_URL,
    [int]$MonitorIntervalSeconds = 60,
    [string]$MonitorOutputPath = '',
    [string]$AlertLogPath = '',
    [double]$MaxErrorRatePercent = 1.0,
    [double]$ReadLineAddP95BudgetMs = 150,
    [double]$HeavyReportP95BudgetMs = 5000,
    [double]$PostP95BudgetMs = 1000
)

$ErrorActionPreference = 'Stop'
if ($DurationHours -le 0 -or $DurationHours -gt 24) {
    throw 'DurationHours must be greater than 0 and no more than 24.'
}
if ($IntervalSeconds -lt 1 -or $IntervalSeconds -gt 3600) {
    throw 'IntervalSeconds must be between 1 and 3600.'
}
if ($EnableWriteTraffic -or $VoidWriteTraffic) {
    if ($WriteIntervalSeconds -lt 1 -or $WriteIntervalSeconds -gt 3600) {
        throw 'WriteIntervalSeconds must be between 1 and 3600.'
    }
    if ($WriteBatchSize -lt 1 -or $WriteBatchSize -gt 50) {
        throw 'WriteBatchSize must be between 1 and 50 per tick.'
    }
    if ($WriteConcurrency -lt 1 -or $WriteConcurrency -gt 16) {
        throw 'WriteConcurrency must be between 1 and 16.'
    }
}
if ([string]::IsNullOrWhiteSpace($OutputPath)) {
    $root = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
    $OutputPath = Join-Path $root ('tmp\phase-w-soak-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '.jsonl')
}
if ([string]::IsNullOrWhiteSpace($WriteManifestPath)) {
    $WriteManifestPath = [System.IO.Path]::ChangeExtension($OutputPath, $null).TrimEnd('.') + '-write-manifest.jsonl'
}
if ([string]::IsNullOrWhiteSpace($MonitorOutputPath)) {
    $MonitorOutputPath = [System.IO.Path]::ChangeExtension($OutputPath, $null).TrimEnd('.') + '-monitor.jsonl'
}
if ([string]::IsNullOrWhiteSpace($AlertLogPath)) {
    $AlertLogPath = [System.IO.Path]::ChangeExtension($OutputPath, $null).TrimEnd('.') + '-alerts.log'
}
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $OutputPath) | Out-Null

# ---------------------------------------------------------------------------
# Safety gate for write traffic. Write-load mode must NEVER run against a
# real/production tenant, and must also never run against a tenant that holds
# real imported/migrated business data used as parity or acceptance evidence
# (e.g. the "legacy-reference-sandbox" tenant, which contains real reconciled
# data from the canonical source and must stay untouched by synthetic writes)
# even though such a tenant's name may otherwise look "sandbox"-like.
#
# Three independent checks must all pass:
#   1. The operator explicitly passes -ConfirmTestTenant (a human decision,
#      not inferred from any flag that could be scripted by accident).
#   2. The tenant code does not match a hard-denied pattern that indicates
#      real/imported/reference data, regardless of confirmation.
#   3. The tenant code matches a recognized synthetic test/demo naming
#      pattern.
# All are required (defense in depth) -- none alone is trusted.
# ---------------------------------------------------------------------------
function Assert-TestTenantSafe {
    param([string]$TenantCode, [bool]$Confirmed)
    if (-not $Confirmed) {
        throw 'Write-load / cleanup mode requires -ConfirmTestTenant to be passed explicitly. ' +
            "Refusing to run write traffic against tenant '$TenantCode' without an explicit " +
            'operator confirmation that this is a non-production test/demo tenant.'
    }
    if ($TenantCode -match '(?i)(legacy|reference|canonical|production|migrat|\bprod\b|\blive\b)') {
        throw "Refusing to run write traffic: tenant code '$TenantCode' matches a hard-denied " +
            'pattern (legacy/reference/canonical/production/migrat*/prod/live). This blocks tenants ' +
            'that hold real imported or reference business data (e.g. legacy-reference-sandbox) even ' +
            'if -ConfirmTestTenant is passed. This check cannot be overridden by any flag.'
    }
    if ($TenantCode -notmatch '(?i)(^|[-_])(demo|test|qa|dev)([-_]|$)') {
        throw "Refusing to run write traffic: tenant code '$TenantCode' does not match a " +
            'recognized synthetic test/demo naming pattern (demo/test/qa/dev as a whole segment). ' +
            'Point this at an explicitly-marked, purely-synthetic test/demo tenant (the seeded ' +
            '"demo" tenant is the known-good default). This check is intentionally conservative ' +
            'and does not accept a real/production or real-data-bearing tenant code under any flag.'
    }
}

# PowerShell's Invoke-WebRequest/Invoke-RestMethod manage cookies through an
# internal CookieContainer. When UseCookies is on (the default) a manually
# supplied "Cookie" header is silently DROPPED in favor of whatever is in that
# container -- passing -Headers @{ Cookie = $token } alone sends the request
# with no cookie at all and the API answers 401. The fix is to preload a
# WebRequestSession's CookieContainer with the session cookie and pass
# -WebSession, never -Headers, for the cookie itself.
function New-CookieWebSession {
    param([string]$ApiBaseUrl, [string]$CookiePair)
    $session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
    if (-not [string]::IsNullOrWhiteSpace($CookiePair)) {
        $parts = $CookiePair -split '=', 2
        $cookie = New-Object System.Net.Cookie($parts[0].Trim(), $parts[1].Trim())
        $uri = [Uri]$ApiBaseUrl
        $cookie.Domain = $uri.Host
        $session.Cookies.Add($uri, $cookie)
    }
    return $session
}

function Invoke-WriteLogin {
    param([string]$ApiBaseUrl, [string]$TenantCode, [string]$Username, [string]$Password)
    if ([string]::IsNullOrWhiteSpace($Password)) {
        throw 'WritePassword (or $env:ABUZAR_PERF_WRITE_PASSWORD) is required for write-load / cleanup mode.'
    }
    $body = @{ username = $Username; password = $Password; tenantCode = $TenantCode } | ConvertTo-Json -Compress
    $response = Invoke-WebRequest -UseBasicParsing -Uri ($ApiBaseUrl.TrimEnd('/') + '/v1/auth/login') `
        -Method Post -ContentType 'application/json' -Body $body -TimeoutSec 15
    $setCookie = $response.Headers['Set-Cookie']
    if ($setCookie -is [array]) { $setCookie = $setCookie[0] }
    if ([string]::IsNullOrWhiteSpace($setCookie)) {
        throw 'Login succeeded but no session cookie was returned.'
    }
    $cookiePair = ($setCookie -split ';')[0]
    $context = ($response.Content | ConvertFrom-Json).context
    # Server-confirmed tenant code is authoritative; re-check it even though
    # the login query itself is scoped by tenantCode, as a second, independent
    # confirmation that we authenticated into the tenant we think we did.
    Assert-TestTenantSafe -TenantCode $context.tenantCode -Confirmed $true
    $session = New-CookieWebSession -ApiBaseUrl $ApiBaseUrl -CookiePair $cookiePair
    return [pscustomobject]@{ Cookie = $cookiePair; Session = $session; Context = $context }
}

function Set-WriteSessionContext {
    param([string]$ApiBaseUrl, [object]$Session, [string]$BranchId, [string]$CounterId)
    if ([string]::IsNullOrWhiteSpace($BranchId) -or [string]::IsNullOrWhiteSpace($CounterId)) { return }
    $body = @{ branchId = $BranchId; counterId = $CounterId } | ConvertTo-Json -Compress
    Invoke-WebRequest -UseBasicParsing -Uri ($ApiBaseUrl.TrimEnd('/') + '/v1/session/context') `
        -Method Post -ContentType 'application/json' -WebSession $Session -Body $body -TimeoutSec 15 | Out-Null
}

function Get-OrCreateWriteSupplier {
    param([string]$ApiBaseUrl, [object]$Session, [string]$Code)
    $encoded = [Uri]::EscapeDataString($Code)
    $listUri = $ApiBaseUrl.TrimEnd('/') + '/v1/master/supplier?search=' + $encoded
    $existing = Invoke-RestMethod -UseBasicParsing -WebSession $Session -TimeoutSec 15 -Uri $listUri
    $match = $existing.records | Where-Object { $_.code -eq $Code } | Select-Object -First 1
    if ($match) { return $match.id }
    $body = @{
        code = $Code
        name = 'Soak write-load test supplier (auto-created; safe to delete)'
        legacyId = $Code
    } | ConvertTo-Json -Compress
    try {
        $created = Invoke-RestMethod -UseBasicParsing -WebSession $Session -TimeoutSec 15 -Method Post `
            -ContentType 'application/json' -Uri ($ApiBaseUrl.TrimEnd('/') + '/v1/master/supplier') -Body $body
        return $created.id
    } catch {
        # Another concurrent soak run may have created the same code between
        # the lookup and the insert; fall back to a second lookup before
        # treating this as a real failure.
        $existing2 = Invoke-RestMethod -UseBasicParsing -WebSession $Session -TimeoutSec 15 -Uri $listUri
        $match2 = $existing2.records | Where-Object { $_.code -eq $Code } | Select-Object -First 1
        if ($match2) { return $match2.id }
        throw
    }
}

function Get-WriteGodownId {
    param([string]$ApiBaseUrl, [object]$Session, [string]$Code)
    $uri = $ApiBaseUrl.TrimEnd('/') + '/v1/master/godown'
    if (-not [string]::IsNullOrWhiteSpace($Code)) { $uri += '?search=' + [Uri]::EscapeDataString($Code) }
    $response = Invoke-RestMethod -UseBasicParsing -WebSession $Session -TimeoutSec 15 -Uri $uri
    if (-not [string]::IsNullOrWhiteSpace($Code)) {
        $match = $response.records | Where-Object { $_.code -eq $Code } | Select-Object -First 1
        if ($match) { return $match.id }
    }
    if (-not $response.records -or $response.records.Count -eq 0) {
        throw 'No godowns were found for the write-load tenant; seed a demo godown before enabling write traffic.'
    }
    return $response.records[0].id
}

function Get-WriteItemIds {
    param([string]$ApiBaseUrl, [object]$Session, [string]$Search)
    $uri = $ApiBaseUrl.TrimEnd('/') + '/v1/items/lookup?q=' + [Uri]::EscapeDataString($Search)
    $response = Invoke-RestMethod -UseBasicParsing -WebSession $Session -TimeoutSec 15 -Uri $uri
    if (-not $response.items -or $response.items.Count -eq 0) {
        throw "No items matched search '$Search' in the write-load tenant; cannot post a document. " +
            'Seed demo items (see docs/evidence/ACCEPTANCE_EVIDENCE_2026-08-07.md, "demo tenant seeded") first.'
    }
    return @($response.items | ForEach-Object { [pscustomobject]@{ id = $_.id; price = $_.payload.SalePrice } })
}

function New-WriteDocumentBody {
    param([string]$Kind, [string]$ItemId, [string]$Price, [string]$SupplierId, [string]$GodownId, [string]$ReferencePrefix)
    $stamp = [DateTime]::UtcNow.ToString('yyyyMMddHHmmssfff')
    $suffix = "$stamp-$([guid]::NewGuid().ToString('N').Substring(0, 8))"
    $reference = "$ReferencePrefix-$suffix"
    $unitPrice = if ([string]::IsNullOrWhiteSpace($Price)) { '1.00' } else { $Price }
    $occurredAt = [DateTime]::UtcNow.ToString('o')
    $document = [ordered]@{
        kind        = $Kind
        occurredAt  = $occurredAt
        supplierId  = $SupplierId
        godownId    = $GodownId
        reference   = $reference
        remarks     = 'Automated write-load soak probe (run-soak.ps1 -EnableWriteTraffic); safe to void/delete.'
        lines       = @(
            [ordered]@{
                lineNumber = 1
                itemId     = $ItemId
                quantity   = '1'
                unitPrice  = $unitPrice
                unitCost   = $unitPrice
                batchNumber = $reference
            }
        )
    }
    $command = [ordered]@{
        commandId      = [guid]::NewGuid().ToString()
        kind           = $Kind
        action         = 'save-and-post'
        idempotencyKey = $reference
        occurredAt     = $occurredAt
        document       = $document
    }
    return @{ body = ($command | ConvertTo-Json -Depth 8 -Compress); reference = $reference }
}

function Invoke-WriteDocumentPost {
    param([string]$Uri, [object]$Session, [string]$Body)
    $watch = [Diagnostics.Stopwatch]::StartNew()
    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri $Uri -Method Post -ContentType 'application/json' `
            -WebSession $Session -Body $Body -TimeoutSec 15
        $watch.Stop()
        $parsed = $response.Content | ConvertFrom-Json
        return [pscustomobject]@{
            ok = $true; status = [int]$response.StatusCode; durationMs = $watch.Elapsed.TotalMilliseconds
            documentId = $parsed.document.id; version = $parsed.document.version; error = $null
        }
    } catch {
        $watch.Stop()
        $status = $null
        try { if ($_.Exception.Response) { $status = [int]$_.Exception.Response.StatusCode } } catch {}
        return [pscustomobject]@{
            ok = $false; status = $status; durationMs = $watch.Elapsed.TotalMilliseconds
            documentId = $null; version = $null; error = $_.Exception.Message
        }
    }
}

function Invoke-WriteBatch {
    param(
        [string]$ApiBaseUrl, [object]$Session, [string]$Kind, [object[]]$Items,
        [string]$SupplierId, [string]$GodownId, [string]$ReferencePrefix,
        [int]$BatchSize, [int]$Concurrency
    )
    $uri = $ApiBaseUrl.TrimEnd('/') + '/v1/documents/' + $Kind
    $plans = for ($i = 0; $i -lt $BatchSize; $i++) {
        $item = $Items[(Get-Random -Minimum 0 -Maximum $Items.Count)]
        New-WriteDocumentBody -Kind $Kind -ItemId $item.id -Price $item.price -SupplierId $SupplierId `
            -GodownId $GodownId -ReferencePrefix $ReferencePrefix
    }
    if ($Concurrency -le 1 -or $BatchSize -le 1) {
        return $plans | ForEach-Object {
            $result = Invoke-WriteDocumentPost -Uri $uri -Session $Session -Body $_.body
            $result | Add-Member -NotePropertyName reference -NotePropertyValue $_.reference -PassThru
        }
    }
    # True parallel posts via a runspace pool. The WebRequestSession (and its
    # CookieContainer) is a plain .NET object shared in-process across
    # runspaces in the same RunspacePool, so every runspace authenticates with
    # the same session established once at login.
    $pool = [runspacefactory]::CreateRunspacePool(1, [Math]::Max(1, $Concurrency))
    $pool.Open()
    try {
        $tasks = foreach ($plan in $plans) {
            $ps = [powershell]::Create()
            $ps.RunspacePool = $pool
            [void]$ps.AddScript({
                param($Uri, $Session, $Body)
                $watch = [Diagnostics.Stopwatch]::StartNew()
                try {
                    $response = Invoke-WebRequest -UseBasicParsing -Uri $Uri -Method Post -ContentType 'application/json' `
                        -WebSession $Session -Body $Body -TimeoutSec 15
                    $watch.Stop()
                    $parsed = $response.Content | ConvertFrom-Json
                    [pscustomobject]@{
                        ok = $true; status = [int]$response.StatusCode; durationMs = $watch.Elapsed.TotalMilliseconds
                        documentId = $parsed.document.id; version = $parsed.document.version; error = $null
                    }
                } catch {
                    $watch.Stop()
                    $status = $null
                    try { if ($_.Exception.Response) { $status = [int]$_.Exception.Response.StatusCode } } catch {}
                    [pscustomobject]@{
                        ok = $false; status = $status; durationMs = $watch.Elapsed.TotalMilliseconds
                        documentId = $null; version = $null; error = $_.Exception.Message
                    }
                }
            }).AddArgument($uri).AddArgument($Session).AddArgument($plan.body)
            [pscustomobject]@{ PS = $ps; Handle = $ps.BeginInvoke(); Reference = $plan.reference }
        }
        foreach ($task in $tasks) {
            $result = $task.PS.EndInvoke($task.Handle)
            $task.PS.Dispose()
            $result | Add-Member -NotePropertyName reference -NotePropertyValue $task.Reference -PassThru
        }
    } finally {
        $pool.Close()
        $pool.Dispose()
    }
}

function Get-Percentile {
    param([double[]]$Values, [double]$Percentile)
    if (-not $Values -or $Values.Count -eq 0) { return 0 }
    $sorted = @($Values | Sort-Object)
    $index = [int]([math]::Floor(($sorted.Count - 1) * $Percentile))
    return [double]$sorted[$index]
}

function Get-MonitorSnapshot {
    param([string]$PgConnectionString)
    $snapshot = [ordered]@{ at = [DateTime]::UtcNow.ToString('o'); postgres = $null; host = $null; note = $null }
    try {
        $os = Get-CimInstance Win32_OperatingSystem -ErrorAction Stop
        $systemDrive = $env:SystemDrive.TrimEnd(':')
        $drive = Get-PSDrive -Name $systemDrive -ErrorAction SilentlyContinue
        $snapshot.host = [ordered]@{
            memFreeMB  = [math]::Round($os.FreePhysicalMemory / 1024, 1)
            memTotalMB = [math]::Round($os.TotalVisibleMemorySize / 1024, 1)
            diskFreeGB = if ($drive) { [math]::Round($drive.Free / 1GB, 2) } else { $null }
            diskDrive  = $systemDrive
        }
    } catch {
        $snapshot.note = "Host metrics unavailable: $($_.Exception.Message)"
    }
    if ([string]::IsNullOrWhiteSpace($PgConnectionString)) {
        $note = 'Postgres monitoring skipped: -PgMonitorConnectionString / $env:ABUZAR_PERF_PG_MONITOR_URL not set.'
        $snapshot.note = if ($snapshot.note) { "$($snapshot.note) $note" } else { $note }
        return [pscustomobject]$snapshot
    }
    $psqlExe = Get-Command psql -ErrorAction SilentlyContinue
    if (-not $psqlExe) {
        $note = 'Postgres monitoring skipped: psql was not found on PATH.'
        $snapshot.note = if ($snapshot.note) { "$($snapshot.note) $note" } else { $note }
        return [pscustomobject]$snapshot
    }
    try {
        $query = 'select numbackends, xact_commit, xact_rollback, blks_read, blks_hit, ' +
            'round(100.0*blks_hit/nullif(blks_hit+blks_read,0),2) as cache_hit_ratio, ' +
            '(select count(*) from pg_stat_activity where datname=current_database()) as active_connections ' +
            'from pg_stat_database where datname=current_database();'
        $raw = & psql $PgConnectionString -t -A -F '|' -c $query 2>&1
        if ($LASTEXITCODE -ne 0) {
            $snapshot.note = "psql monitoring query failed (exit $LASTEXITCODE): $raw"
        } else {
            $line = @($raw | Where-Object { $_ -match '\|' })[0]
            if ($line) {
                $parts = $line -split '\|'
                $snapshot.postgres = [ordered]@{
                    numBackends          = [int]$parts[0]
                    xactCommit           = [int64]$parts[1]
                    xactRollback         = [int64]$parts[2]
                    blksRead             = [int64]$parts[3]
                    blksHit              = [int64]$parts[4]
                    cacheHitRatioPercent = if ($parts[5]) { [double]$parts[5] } else { $null }
                    activeConnections    = [int]$parts[6]
                }
            }
        }
    } catch {
        $snapshot.note = "Postgres monitoring failed: $($_.Exception.Message)"
    }
    return [pscustomobject]$snapshot
}

function Write-AlertLine {
    param([string]$Message, [string]$Path)
    $line = "[$([DateTime]::UtcNow.ToString('o'))] $Message"
    Write-Warning $line
    Add-Content -LiteralPath $Path -Value $line
}

# ---------------------------------------------------------------------------
# Cleanup mode: void every document recorded by a prior write-load run, then
# exit without running the soak loop.
# ---------------------------------------------------------------------------
if ($VoidWriteTraffic) {
    Assert-TestTenantSafe -TenantCode $WriteTenantCode -Confirmed $ConfirmTestTenant.IsPresent
    if (-not (Test-Path -LiteralPath $WriteManifestPath)) {
        throw "Write manifest not found: $WriteManifestPath"
    }
    $login = Invoke-WriteLogin -ApiBaseUrl $ApiBaseUrl -TenantCode $WriteTenantCode -Username $WriteUsername -Password $WritePassword
    Set-WriteSessionContext -ApiBaseUrl $ApiBaseUrl -Session $login.Session -BranchId $WriteBranchId -CounterId $WriteCounterId
    $entries = Get-Content -LiteralPath $WriteManifestPath | Where-Object { $_.Trim() -ne '' } | ForEach-Object { $_ | ConvertFrom-Json }
    Write-Output "Voiding $($entries.Count) recorded write-soak document(s) in tenant '$WriteTenantCode'..."
    $voided = 0
    $failed = 0
    foreach ($entry in $entries) {
        $voidCommand = [ordered]@{
            commandId       = [guid]::NewGuid().ToString()
            kind            = $entry.kind
            action          = 'void'
            idempotencyKey  = "void-$($entry.documentId)-$([guid]::NewGuid().ToString('N').Substring(0,8))"
            occurredAt      = [DateTime]::UtcNow.ToString('o')
            documentId      = $entry.documentId
            reason          = 'Soak write-load test cleanup (run-soak.ps1 -VoidWriteTraffic)'
            expectedVersion = $entry.version
        } | ConvertTo-Json -Compress
        $uri = $ApiBaseUrl.TrimEnd('/') + '/v1/documents/' + $entry.kind
        try {
            Invoke-WebRequest -UseBasicParsing -Uri $uri -Method Post -ContentType 'application/json' `
                -WebSession $login.Session -Body $voidCommand -TimeoutSec 15 | Out-Null
            $voided++
        } catch {
            $failed++
            Write-Warning "Failed to void $($entry.documentId) ($($entry.reference)): $($_.Exception.Message)"
        }
    }
    [pscustomobject]@{ status = 'void-complete'; voided = $voided; failed = $failed; manifest = $WriteManifestPath } | ConvertTo-Json
    exit 0
}

$readSession = $null
if (-not [string]::IsNullOrWhiteSpace($SessionCookie)) {
    # Supplied through the protected process environment; never print it.
    # See New-CookieWebSession above for why this must be a WebSession and
    # not a manually-set "Cookie" header.
    $readSession = New-CookieWebSession -ApiBaseUrl $ApiBaseUrl -CookiePair $SessionCookie
}
$started = [DateTime]::UtcNow
$deadline = $started.AddHours($DurationHours)
$samples = 0
$errors = 0
$readLatenciesByProbe = @{}

function Probe {
    param([string]$Name, [string]$Uri)
    $watch = [Diagnostics.Stopwatch]::StartNew()
    try {
        if ($readSession) {
            $response = Invoke-WebRequest -UseBasicParsing -Uri $Uri -WebSession $readSession -TimeoutSec 10
        } else {
            $response = Invoke-WebRequest -UseBasicParsing -Uri $Uri -TimeoutSec 10
        }
        $watch.Stop()
        return [pscustomobject]@{
            name = $Name; status = [int]$response.StatusCode
            durationMs = $watch.Elapsed.TotalMilliseconds; error = $null
        }
    } catch {
        $watch.Stop()
        return [pscustomobject]@{
            name = $Name; status = $null
            durationMs = $watch.Elapsed.TotalMilliseconds; error = $_.Exception.Message
        }
    }
}

# ---------------------------------------------------------------------------
# Write-load setup (opt-in). Everything below only runs when the operator
# explicitly asked for write traffic and passed the tenant safety gate.
# ---------------------------------------------------------------------------
$writeEnabled = $EnableWriteTraffic.IsPresent
$writeSession = $null
$writeSupplierId = $null
$writeGodownId = $null
$writeItems = $null
$writeAttempted = 0
$writeSucceeded = 0
$writeFailed = 0
$writeLatencies = New-Object System.Collections.Generic.List[double]

if ($writeEnabled) {
    Assert-TestTenantSafe -TenantCode $WriteTenantCode -Confirmed $ConfirmTestTenant.IsPresent
    Write-Output "Write-load mode ENABLED against tenant '$WriteTenantCode' (kind=$WriteDocumentKind, every ${WriteIntervalSeconds}s, batch=$WriteBatchSize, concurrency=$WriteConcurrency)."
    $login = Invoke-WriteLogin -ApiBaseUrl $ApiBaseUrl -TenantCode $WriteTenantCode -Username $WriteUsername -Password $WritePassword
    $writeSession = $login.Session
    Set-WriteSessionContext -ApiBaseUrl $ApiBaseUrl -Session $writeSession -BranchId $WriteBranchId -CounterId $WriteCounterId
    $writeSupplierId = Get-OrCreateWriteSupplier -ApiBaseUrl $ApiBaseUrl -Session $writeSession -Code $WriteSupplierCode
    $writeGodownId = Get-WriteGodownId -ApiBaseUrl $ApiBaseUrl -Session $writeSession -Code $WriteGodownCode
    $writeItems = Get-WriteItemIds -ApiBaseUrl $ApiBaseUrl -Session $writeSession -Search $WriteItemSearch
    Write-Output "Write-load fixture ready: supplier=$writeSupplierId godown=$writeGodownId items=$($writeItems.Count) (search='$WriteItemSearch')"
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $WriteManifestPath) | Out-Null
    Write-Output "Write-load manifest: $WriteManifestPath (use -VoidWriteTraffic to reverse these documents afterward)"
}

$monitorEnabled = $MonitorIntervalSeconds -gt 0
if ($monitorEnabled) {
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $MonitorOutputPath) | Out-Null
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $AlertLogPath) | Out-Null
    if ([string]::IsNullOrWhiteSpace($PgMonitorConnectionString)) {
        Write-Output 'Postgres monitoring: not configured (pass -PgMonitorConnectionString or $env:ABUZAR_PERF_PG_MONITOR_URL to enable). Host metrics only.'
    }
}

Write-Output "Read-only soak is configured for $DurationHours hour(s); output: $OutputPath"
$lastWriteAt = [DateTime]::MinValue
$lastMonitorAt = [DateTime]::MinValue
$allAlerts = New-Object System.Collections.Generic.List[string]

while ([DateTime]::UtcNow -lt $deadline) {
    $now = [DateTime]::UtcNow
    $sample = @(
        Probe -Name 'health' -Uri ($ApiBaseUrl.TrimEnd('/') + '/v1/health')
        Probe -Name 'metrics' -Uri ($ApiBaseUrl.TrimEnd('/') + '/v1/metrics')
    )
    if (-not [string]::IsNullOrWhiteSpace($SessionCookie)) {
        $sample += Probe -Name 'pos-line-add-proxy' -Uri (
            $ApiBaseUrl.TrimEnd('/') + '/v1/inventory/availability?itemLegacyId=' +
            [Uri]::EscapeDataString($ItemLegacyId) + '&godownId=' + [Uri]::EscapeDataString($GodownId))
        $sample += Probe -Name 'heavy-stock-report-proxy' -Uri (
            $ApiBaseUrl.TrimEnd('/') + '/v1/reports/stock?page=1&pageSize=25&from=1900-01-01&to=2999-12-31&godownId=' +
            [Uri]::EscapeDataString($GodownId))
        $sample += Probe -Name 'finance-journals-proxy' -Uri (
            $ApiBaseUrl.TrimEnd('/') + '/v1/finance/journals?limit=25')
    }
    foreach ($entry in $sample) {
        if (-not $readLatenciesByProbe.ContainsKey($entry.name)) { $readLatenciesByProbe[$entry.name] = New-Object System.Collections.Generic.List[double] }
        $readLatenciesByProbe[$entry.name].Add($entry.durationMs)
        if ($entry.error) { $errors++ }
    }

    $writeTick = $null
    if ($writeEnabled -and ($now - $lastWriteAt).TotalSeconds -ge $WriteIntervalSeconds -and
        ($WriteMaxDocuments -le 0 -or $writeAttempted -lt $WriteMaxDocuments)) {
        $lastWriteAt = $now
        $batchSize = $WriteBatchSize
        if ($WriteMaxDocuments -gt 0) { $batchSize = [Math]::Min($batchSize, $WriteMaxDocuments - $writeAttempted) }
        if ($batchSize -gt 0) {
            $results = Invoke-WriteBatch -ApiBaseUrl $ApiBaseUrl -Session $writeSession -Kind $WriteDocumentKind `
                -Items $writeItems -SupplierId $writeSupplierId -GodownId $writeGodownId `
                -ReferencePrefix $WriteReferencePrefix -BatchSize $batchSize -Concurrency $WriteConcurrency
            $tickSucceeded = 0
            $tickFailed = 0
            foreach ($result in $results) {
                $writeAttempted++
                $writeLatencies.Add($result.durationMs)
                if ($result.ok) {
                    $writeSucceeded++
                    $tickSucceeded++
                    $manifestLine = [pscustomobject]@{
                        at = $now.ToString('o'); documentId = $result.documentId; version = $result.version
                        kind = $WriteDocumentKind; reference = $result.reference
                    } | ConvertTo-Json -Compress
                    Add-Content -LiteralPath $WriteManifestPath -Value $manifestLine
                } else {
                    $writeFailed++
                    $tickFailed++
                    $errors++
                }
            }
            $writeTick = [pscustomobject]@{
                attempted = $results.Count; succeeded = $tickSucceeded; failed = $tickFailed
                p50Ms = Get-Percentile -Values @($results | ForEach-Object { $_.durationMs }) -Percentile 0.50
                p95Ms = Get-Percentile -Values @($results | ForEach-Object { $_.durationMs }) -Percentile 0.95
            }
        }
    }

    $monitorTick = $null
    if ($monitorEnabled -and ($now - $lastMonitorAt).TotalSeconds -ge $MonitorIntervalSeconds) {
        $lastMonitorAt = $now
        $monitorTick = Get-MonitorSnapshot -PgConnectionString $PgMonitorConnectionString
        Add-Content -LiteralPath $MonitorOutputPath -Value ($monitorTick | ConvertTo-Json -Compress -Depth 6)

        $totalReadSamples = ($readLatenciesByProbe.Values | ForEach-Object { $_.Count } | Measure-Object -Sum).Sum
        $totalAttempts = $totalReadSamples + $writeAttempted
        $errorRatePercent = if ($totalAttempts -gt 0) { [math]::Round(100.0 * $errors / $totalAttempts, 3) } else { 0 }
        $lineAddP95 = if ($readLatenciesByProbe.ContainsKey('pos-line-add-proxy')) { Get-Percentile -Values @($readLatenciesByProbe['pos-line-add-proxy']) -Percentile 0.95 } else { 0 }
        $reportP95 = if ($readLatenciesByProbe.ContainsKey('heavy-stock-report-proxy')) { Get-Percentile -Values @($readLatenciesByProbe['heavy-stock-report-proxy']) -Percentile 0.95 } else { 0 }
        $writeP95 = if ($writeLatencies.Count -gt 0) { Get-Percentile -Values @($writeLatencies) -Percentile 0.95 } else { 0 }

        $tickAlerts = @()
        if ($errorRatePercent -gt $MaxErrorRatePercent) {
            $tickAlerts += "FAIL error_rate ${errorRatePercent}% exceeds threshold ${MaxErrorRatePercent}%"
        }
        if ($lineAddP95 -gt $ReadLineAddP95BudgetMs) {
            $tickAlerts += "WARN pos-line-add p95 ${lineAddP95}ms exceeds RUNBOOK_CUTOVER budget ${ReadLineAddP95BudgetMs}ms"
        }
        if ($reportP95 -gt $HeavyReportP95BudgetMs) {
            $tickAlerts += "WARN heavy-report p95 ${reportP95}ms exceeds RUNBOOK_CUTOVER budget ${HeavyReportP95BudgetMs}ms"
        }
        if ($writeEnabled -and $writeP95 -gt $PostP95BudgetMs) {
            $tickAlerts += "FAIL post p95 ${writeP95}ms exceeds RUNBOOK_CUTOVER acceptance gate ${PostP95BudgetMs}ms"
        }
        if ($monitorTick.postgres -and $monitorTick.postgres.activeConnections -gt 0 -and
            $monitorTick.postgres.cacheHitRatioPercent -ne $null -and $monitorTick.postgres.cacheHitRatioPercent -lt 90) {
            $tickAlerts += "WARN Postgres cache hit ratio $($monitorTick.postgres.cacheHitRatioPercent)% is below 90%"
        }
        foreach ($alertMessage in $tickAlerts) {
            Write-AlertLine -Message $alertMessage -Path $AlertLogPath
            $allAlerts.Add($alertMessage)
        }
    }

    $record = [pscustomobject]@{
        at = $now.ToString('o')
        samples = $samples
        read = $sample
        write = $writeTick
        monitor = $monitorTick
        postTraffic = if ($writeEnabled) {
            [pscustomobject]@{
                mode = 'measured'; attempted = $writeAttempted; succeeded = $writeSucceeded; failed = $writeFailed
                p50Ms = Get-Percentile -Values @($writeLatencies) -Percentile 0.50
                p95Ms = Get-Percentile -Values @($writeLatencies) -Percentile 0.95
            }
        } else {
            'not_run'
        }
        note = if ($writeEnabled) {
            "Write traffic is enabled against tenant '$WriteTenantCode'; documents are tagged with reference prefix '$WriteReferencePrefix' and recorded in $WriteManifestPath for reversal via -VoidWriteTraffic."
        } else {
            'No document writes are generated by this safe soak. Pass -EnableWriteTraffic -ConfirmTestTenant to measure post latency against an explicitly-confirmed test/demo tenant.'
        }
    }
    Add-Content -LiteralPath $OutputPath -Value ($record | ConvertTo-Json -Compress -Depth 8)
    $samples++
    Start-Sleep -Seconds $IntervalSeconds
}

$finalErrorRatePercent = 0
$totalReadSamples = ($readLatenciesByProbe.Values | ForEach-Object { $_.Count } | Measure-Object -Sum).Sum
$totalAttempts = $totalReadSamples + $writeAttempted
if ($totalAttempts -gt 0) { $finalErrorRatePercent = [math]::Round(100.0 * $errors / $totalAttempts, 3) }
$finalLineAddP95 = if ($readLatenciesByProbe.ContainsKey('pos-line-add-proxy')) { Get-Percentile -Values @($readLatenciesByProbe['pos-line-add-proxy']) -Percentile 0.95 } else { $null }
$finalReportP95 = if ($readLatenciesByProbe.ContainsKey('heavy-stock-report-proxy')) { Get-Percentile -Values @($readLatenciesByProbe['heavy-stock-report-proxy']) -Percentile 0.95 } else { $null }
$finalWriteP50 = if ($writeLatencies.Count -gt 0) { Get-Percentile -Values @($writeLatencies) -Percentile 0.50 } else { $null }
$finalWriteP95 = if ($writeLatencies.Count -gt 0) { Get-Percentile -Values @($writeLatencies) -Percentile 0.95 } else { $null }

[pscustomobject]@{
    status = 'completed'
    startedAt = $started.ToString('o')
    finishedAt = [DateTime]::UtcNow.ToString('o')
    configuredHours = $DurationHours
    samples = $samples
    errors = $errors
    errorRatePercent = $finalErrorRatePercent
    artifact = $OutputPath
    postTraffic = if ($writeEnabled) {
        [pscustomobject]@{
            mode = 'measured'; tenant = $WriteTenantCode; kind = $WriteDocumentKind
            attempted = $writeAttempted; succeeded = $writeSucceeded; failed = $writeFailed
            p50Ms = $finalWriteP50; p95Ms = $finalWriteP95
            postP95BudgetMs = $PostP95BudgetMs
            gate = if ($finalWriteP95 -ne $null) { if ($finalWriteP95 -le $PostP95BudgetMs) { 'pass' } else { 'fail' } } else { 'no_data' }
            manifest = $WriteManifestPath
        }
    } else {
        'not_run'
    }
    readGates = [pscustomobject]@{
        posLineAddP95Ms = $finalLineAddP95
        posLineAddBudgetMs = $ReadLineAddP95BudgetMs
        posLineAddGate = if ($finalLineAddP95 -ne $null) { if ($finalLineAddP95 -le $ReadLineAddP95BudgetMs) { 'pass' } else { 'fail' } } else { 'no_data' }
        heavyReportP95Ms = $finalReportP95
        heavyReportBudgetMs = $HeavyReportP95BudgetMs
        heavyReportGate = if ($finalReportP95 -ne $null) { if ($finalReportP95 -le $HeavyReportP95BudgetMs) { 'pass' } else { 'fail' } } else { 'no_data' }
    }
    monitor = if ($monitorEnabled) {
        [pscustomobject]@{ artifact = $MonitorOutputPath; alertLog = $AlertLogPath; alertCount = $allAlerts.Count }
    } else {
        'disabled'
    }
    alerts = @($allAlerts)
} | ConvertTo-Json -Depth 6
