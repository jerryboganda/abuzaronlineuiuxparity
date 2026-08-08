<#
.SYNOPSIS
    Validates a completed AbuzarNext cutover go/no-go file against the
    decision rule in docs/RUNBOOK_CUTOVER.md and
    docs/CUTOVER_GO_NO_GO_TEMPLATE.json.

.DESCRIPTION
    Enforces: "A validator must reject GO when any required check is
    pending, blocked, or fail" (docs/RUNBOOK_CUTOVER.md, Section 5, step 9),
    and additionally requires every required approval
    (releaseManager, dba, businessApprover, branchOperator) to be present.

    This script does not mutate the input file and does not decide
    anything on its own authority -- it only reports what the file
    already says. Populating the file with real evidence remains a human,
    physical step per the runbook.

.PARAMETER Path
    Path to a go/no-go JSON file using the same schema as
    docs/CUTOVER_GO_NO_GO_TEMPLATE.json.

.OUTPUTS
    A human-readable summary to stdout. Exit code 0 only when the
    document is GO. Exit code 1 for NO-GO or a malformed document.

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File .\ops\cutover\validate-go-no-go.ps1 -Path .\docs\CUTOVER_GO_NO_GO_TEMPLATE.json
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Path
)

$ErrorActionPreference = 'Stop'

if (-not (Test-Path -LiteralPath $Path)) {
    throw "Go/no-go file not found: $Path"
}

$raw = Get-Content -LiteralPath $Path -Raw
try {
    $doc = $raw | ConvertFrom-Json
} catch {
    throw "Failed to parse JSON at '$Path': $($_.Exception.Message)"
}

if (-not $doc.PSObject.Properties['checks'] -or $null -eq $doc.checks) {
    throw "'$Path' has no 'checks' array; it is not a valid go/no-go document."
}

$requiredApprovers = @('releaseManager', 'dba', 'businessApprover', 'branchOperator')
$blockingStatuses = @('pending', 'blocked', 'fail')

$blockers = New-Object System.Collections.Generic.List[string]
$checkRows = New-Object System.Collections.Generic.List[object]

foreach ($check in @($doc.checks)) {
    $isRequired = [bool]$check.required
    $status = [string]$check.status
    $isBlocking = $isRequired -and ($blockingStatuses -contains $status)

    if ($isBlocking) {
        $reason = "status='$status'"
        if ($check.currentBlocker) {
            $reason += "; $($check.currentBlocker)"
        }
        $blockers.Add("CHECK   [$($check.id)] $reason")
    }

    $checkRows.Add([pscustomobject]@{
        Id       = $check.id
        Required = $isRequired
        Status   = $status
        Blocking = $isBlocking
    })
}

$approvalRows = New-Object System.Collections.Generic.List[object]
foreach ($approver in $requiredApprovers) {
    $value = $null
    if ($doc.PSObject.Properties['approvals'] -and $null -ne $doc.approvals -and $doc.approvals.PSObject.Properties[$approver]) {
        $value = $doc.approvals.$approver
    }
    $present = -not ($null -eq $value -or ([string]$value).Trim() -eq '')
    if (-not $present) {
        $blockers.Add("APPROVAL [$approver] missing")
    }
    $approvalRows.Add([pscustomobject]@{
        Approver = $approver
        Present  = $present
        Value    = $value
    })
}

$decision = if ($blockers.Count -eq 0) { 'GO' } else { 'NO-GO' }

# ---- Summary (safe to paste into a sign-off record) ----------------------

$line = ('=' * 72)
Write-Output $line
Write-Output ' AbuzarNext Cutover Go/No-Go Validation'
Write-Output "  Source file : $Path"
Write-Output "  Doc status  : $($doc.status)"
Write-Output "  Generated   : $($doc.generatedAtUtc)"
Write-Output "  Checked at  : $([DateTime]::UtcNow.ToString('o'))"
Write-Output $line
Write-Output ''
Write-Output 'Required checks:'
foreach ($row in ($checkRows | Where-Object { $_.Required })) {
    $marker = if ($row.Blocking) { 'BLOCKING' } else { 'OK      ' }
    Write-Output ("  [{0}] {1,-38} status={2}" -f $marker, $row.Id, $row.Status)
}

$optionalRows = @($checkRows | Where-Object { -not $_.Required })
if ($optionalRows.Count -gt 0) {
    Write-Output ''
    Write-Output 'Non-required checks (informational only):'
    foreach ($row in $optionalRows) {
        Write-Output ("  [INFO    ] {0,-38} status={1}" -f $row.Id, $row.Status)
    }
}

Write-Output ''
Write-Output 'Required approvals:'
foreach ($row in $approvalRows) {
    $marker = if ($row.Present) { 'OK      ' } else { 'BLOCKING' }
    $shown = if ($row.Present) { $row.Value } else { '(missing)' }
    Write-Output ("  [{0}] {1,-20} {2}" -f $marker, $row.Approver, $shown)
}

Write-Output ''
if ($blockers.Count -gt 0) {
    Write-Output "Blocking items ($($blockers.Count)):"
    $i = 1
    foreach ($b in $blockers) {
        Write-Output ("  {0,2}. {1}" -f $i, $b)
        $i++
    }
    Write-Output ''
}

Write-Output "DECISION: $decision"
Write-Output $line

if ($decision -eq 'GO') {
    exit 0
} else {
    exit 1
}
