$ErrorActionPreference = "Continue"
Set-Location "e:\learngo\eino_agent"

$evalExe   = ".\eval.exe"
$input     = "data/eval_benchmark_2026.jsonl"
$baseUrl   = "http://localhost:19093"
$kbIds     = "d320146a-d52a-4531-a0ea-16627f723af3"
$timeout   = 300
$reportDir = "docs/eval_reports"

if (-not (Test-Path $reportDir)) { New-Item -ItemType Directory -Path $reportDir -Force | Out-Null }

$combos = @(
    @{ Mode = "pipeline"; Strategy = "vector" },
    @{ Mode = "pipeline"; Strategy = "hybrid" },
    @{ Mode = "pipeline"; Strategy = "hybrid_rerank" },
    @{ Mode = "pipeline"; Strategy = "full" },
    @{ Mode = "agentic"; Strategy = "vector" },
    @{ Mode = "agentic"; Strategy = "hybrid" },
    @{ Mode = "agentic"; Strategy = "hybrid_rerank" },
    @{ Mode = "agentic"; Strategy = "full" }
)

$total = $combos.Count
$idx   = 0

foreach ($c in $combos) {
    $idx++
    $mode     = $c.Mode
    $strategy = $c.Strategy
    $report   = "$reportDir/benchmark_${mode}_${strategy}.md"
    $logFile  = "$reportDir/${mode}_${strategy}_log.txt"

    Write-Host "`n[$idx/$total] Running: mode=$mode strategy=$strategy" -ForegroundColor Cyan
    Write-Host "  Report: $report"
    Write-Host "  Started: $(Get-Date -Format 'HH:mm:ss')"

    $proc = Start-Process -FilePath $evalExe `
        -ArgumentList @("-input", $input, "-base-url", $baseUrl, "-knowledge-base-ids", $kbIds, "-mode", $mode, "-strategy", $strategy, "-timeout", "$timeout", "-report", $report) `
        -WorkingDirectory "e:\learngo\eino_agent" `
        -RedirectStandardOutput $logFile `
        -RedirectStandardError "$reportDir/${mode}_${strategy}_err.txt" `
        -PassThru -NoNewWindow

    $proc.WaitForExit()
    $exitCode = $proc.ExitCode

    Write-Host "  Finished: $(Get-Date -Format 'HH:mm:ss') ExitCode=$exitCode"

    if ($exitCode -ne 0) {
        Write-Host "  ERROR! Check $logFile" -ForegroundColor Red
        $errContent = Get-Content "$reportDir/${mode}_${strategy}_err.txt" -ErrorAction SilentlyContinue
        if ($errContent) { Write-Host "  stderr: $($errContent -join ' ')" -ForegroundColor Red }
    } else {
        $logTail = Get-Content $logFile -ErrorAction SilentlyContinue | Select-Object -Last 8
        foreach ($line in $logTail) { Write-Host "  $line" }
    }
}

Write-Host "`n========== ALL BENCHMARKS COMPLETE ==========" -ForegroundColor Green
Write-Host "Reports in $reportDir/"
Get-ChildItem "$reportDir/benchmark_*.md" | ForEach-Object { Write-Host "  $($_.Name)" }
