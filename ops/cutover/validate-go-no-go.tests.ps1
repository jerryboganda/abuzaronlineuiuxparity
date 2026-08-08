<#
.SYNOPSIS
    Self-contained assertion script for validate-go-no-go.ps1. No test
    framework dependency (the repo has no existing Pester convention).

.DESCRIPTION
    1. Runs the validator against the real docs\CUTOVER_GO_NO_GO_TEMPLATE.json
       and asserts it correctly reports NO-GO (exit 1) with the current
       known blockers listed. This proves the validator works against real
       repo data without ever modifying that file.
    2. Runs the validator against a synthetic fully-passing fixture
       (testdata\passing-fixture.json) and asserts it reports GO (exit 0).
    3. Runs the validator against a malformed file and asserts it fails
       loudly instead of silently defaulting to GO.

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File .\ops\cutover\validate-go-no-go.tests.ps1
#>
$ErrorActionPreference = 'Stop'

$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$validator = Join-Path $PSScriptRoot 'validate-go-no-go.ps1'
$templatePath = Join-Path $root 'docs\CUTOVER_GO_NO_GO_TEMPLATE.json'
$passingFixture = Join-Path $PSScriptRoot 'testdata\passing-fixture.json'

$failures = New-Object System.Collections.Generic.List[string]

function Invoke-Validator {
    param([string]$JsonPath)
    # Native stderr must not be redirected under ErrorActionPreference =
    # 'Stop': PowerShell 5.1 wraps each stderr line as a terminating
    # NativeCommandError, which would abort this script instead of letting
    # us inspect the exit code/output. Relax it only around this call.
    $previousPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $output = & powershell -NoProfile -ExecutionPolicy Bypass -File $validator -Path $JsonPath 2>&1
    } finally {
        $ErrorActionPreference = $previousPreference
    }
    return [pscustomobject]@{
        ExitCode = $LASTEXITCODE
        Output   = ($output -join "`n")
    }
}

function Assert-True {
    param([bool]$Condition, [string]$Message)
    if ($Condition) {
        Write-Output "  PASS: $Message"
    } else {
        Write-Output "  FAIL: $Message"
        $failures.Add($Message)
    }
}

# ---- Test 1: real template must be NO-GO with current blockers listed ----
Write-Output 'Test 1: validator against docs\CUTOVER_GO_NO_GO_TEMPLATE.json'
$real = Invoke-Validator -JsonPath $templatePath
Assert-True ($real.ExitCode -eq 1) "exit code is 1 (got $($real.ExitCode))"
Assert-True ($real.Output -match 'DECISION: NO-GO') 'output contains DECISION: NO-GO'
Assert-True ($real.Output -match 'canonical_inventory') 'blocked check canonical_inventory is listed'
Assert-True ($real.Output -match 'functional_uat') 'blocked check functional_uat is listed'
Assert-True ($real.Output -match 'rollback_rehearsal') 'pending check rollback_rehearsal is listed'
Assert-True ($real.Output -match 'APPROVAL \[releaseManager\] missing') 'missing releaseManager approval is listed'
Assert-True ($real.Output -match 'APPROVAL \[dba\] missing') 'missing dba approval is listed'
Assert-True ($real.Output -match 'APPROVAL \[businessApprover\] missing') 'missing businessApprover approval is listed'
Assert-True ($real.Output -match 'APPROVAL \[branchOperator\] missing') 'missing branchOperator approval is listed'
Assert-True ($real.Output -match 'migration_source_files_001_028[\s\S]*?status=pass') 'the one currently-passing check (migration_source_files_001_028) is reported as OK, not blocking'

# ---- Test 2: synthetic fully-passing fixture must be GO -------------------
Write-Output ''
Write-Output 'Test 2: validator against testdata\passing-fixture.json'
$pass = Invoke-Validator -JsonPath $passingFixture
Assert-True ($pass.ExitCode -eq 0) "exit code is 0 (got $($pass.ExitCode))"
Assert-True ($pass.Output -match 'DECISION: GO') 'output contains DECISION: GO'
Assert-True ($pass.Output -notmatch 'BLOCKING') 'no BLOCKING markers appear in a fully-passing document'

# ---- Test 3: malformed file must fail loudly, not silently pass -----------
Write-Output ''
Write-Output 'Test 3: validator against a malformed file'
$badPath = Join-Path $env:TEMP 'validate-go-no-go-bad-fixture.json'
Set-Content -LiteralPath $badPath -Value '{ "not": "a checklist" }' -Encoding utf8
$bad = Invoke-Validator -JsonPath $badPath
Assert-True ($bad.ExitCode -ne 0) "exit code is non-zero for a document with no 'checks' array (got $($bad.ExitCode))"
Remove-Item -LiteralPath $badPath -Force -ErrorAction SilentlyContinue

# ---- Result -----------------------------------------------------------
Write-Output ''
if ($failures.Count -eq 0) {
    Write-Output 'ALL TESTS PASSED'
    exit 0
} else {
    Write-Output "$($failures.Count) TEST(S) FAILED:"
    foreach ($f in $failures) { Write-Output "  - $f" }
    exit 1
}
