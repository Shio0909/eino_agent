$ErrorActionPreference = 'Stop'

$baseUrl = 'http://localhost:8080'
$requestTimeoutSec = 45
$resumeTs = ''   # 可填已有时间戳（例如 20260227_135057）进行断点续跑
$ts = Get-Date -Format 'yyyyMMdd_HHmmss'
if ($resumeTs -ne '') { $ts = $resumeTs }
$reportDir = 'docs/eval_reports'
New-Item -ItemType Directory -Force -Path $reportDir | Out-Null

$datasets = @(
  @{ name = 'clean'; path = 'data/eval_public_go_clean.jsonl'; rounds = 3 },
  @{ name = 'large'; path = 'data/eval_public_go_large.jsonl'; rounds = 3 }
)

$modes = @('pipeline', 'agentic')

function Parse-Metric {
  param(
    [string]$Content,
    [string]$MetricName
  )

  $pattern = [regex]::Escape($MetricName) + ':\s*([0-9\.]+)'
  $m = [regex]::Match($Content, $pattern)
  if ($m.Success) { return [double]$m.Groups[1].Value }
  return [double]::NaN
}

function Avg {
  param([double[]]$arr)
  if ($null -eq $arr -or $arr.Count -eq 0) { return [double]::NaN }
  return ($arr | Measure-Object -Average).Average
}

function Std {
  param([double[]]$arr)
  if ($null -eq $arr -or $arr.Count -le 1) { return 0.0 }
  $mean = Avg -arr $arr
  $sum = 0.0
  foreach ($x in $arr) { $sum += [Math]::Pow(($x - $mean), 2) }
  return [Math]::Sqrt($sum / $arr.Count)
}

$rows = @()

foreach ($d in $datasets) {
  if (-not (Test-Path $d.path)) {
    Write-Host "skip missing dataset: $($d.path)"
    continue
  }

  foreach ($mode in $modes) {
    $recall = @()
    $precision = @()
    $hit = @()
    $mrr = @()
    $ndcg = @()
    $kw = @()
    $p95 = @()
    $err = @()

    for ($i = 1; $i -le $d.rounds; $i++) {
      $reportPath = "$reportDir/${ts}_$($d.name)_${mode}_r$i.md"

      if (Test-Path $reportPath) {
        Write-Host "skip existing: dataset=$($d.name) mode=$mode round=$i"
        $content = Get-Content $reportPath -Raw
        $recall += Parse-Metric -Content $content -MetricName 'Recall@K'
        $precision += Parse-Metric -Content $content -MetricName 'Precision@K'
        $hit += Parse-Metric -Content $content -MetricName 'Hit@K'
        $mrr += Parse-Metric -Content $content -MetricName 'MRR@K'
        $ndcg += Parse-Metric -Content $content -MetricName 'nDCG@K'
        $kw += Parse-Metric -Content $content -MetricName 'Answer Keyword Rate'
        $p95 += Parse-Metric -Content $content -MetricName 'P95 Latency (ms)'
        $err += Parse-Metric -Content $content -MetricName 'Error Rate'
        continue
      }

      Write-Host "run: dataset=$($d.name) mode=$mode round=$i"
      $sw = [System.Diagnostics.Stopwatch]::StartNew()
      go run ./cmd/eval -input $d.path -mode $mode -base-url $baseUrl -timeout $requestTimeoutSec -report $reportPath
      $sw.Stop()
      Write-Host "done: dataset=$($d.name) mode=$mode round=$i elapsed=$([Math]::Round($sw.Elapsed.TotalSeconds,1))s"

      if (-not (Test-Path $reportPath)) {
        Write-Host "warn: missing report after run: $reportPath"
        continue
      }

      $content = Get-Content $reportPath -Raw
      $recall += Parse-Metric -Content $content -MetricName 'Recall@K'
      $precision += Parse-Metric -Content $content -MetricName 'Precision@K'
      $hit += Parse-Metric -Content $content -MetricName 'Hit@K'
      $mrr += Parse-Metric -Content $content -MetricName 'MRR@K'
      $ndcg += Parse-Metric -Content $content -MetricName 'nDCG@K'
      $kw += Parse-Metric -Content $content -MetricName 'Answer Keyword Rate'
      $p95 += Parse-Metric -Content $content -MetricName 'P95 Latency (ms)'
      $err += Parse-Metric -Content $content -MetricName 'Error Rate'
    }

    $rows += [pscustomobject]@{
      dataset = $d.name
      mode = $mode
      rounds = $d.rounds
      recall_avg = [Math]::Round((Avg -arr $recall), 4)
      recall_std = [Math]::Round((Std -arr $recall), 4)
      precision_avg = [Math]::Round((Avg -arr $precision), 4)
      hit_avg = [Math]::Round((Avg -arr $hit), 4)
      mrr_avg = [Math]::Round((Avg -arr $mrr), 4)
      ndcg_avg = [Math]::Round((Avg -arr $ndcg), 4)
      kw_avg = [Math]::Round((Avg -arr $kw), 4)
      p95_avg = [Math]::Round((Avg -arr $p95), 2)
      p95_std = [Math]::Round((Std -arr $p95), 2)
      err_avg = [Math]::Round((Avg -arr $err), 4)
    }
  }
}

$summaryPath = "$reportDir/${ts}_paradigm_matrix_summary.md"
$sb = New-Object System.Text.StringBuilder
$null = $sb.AppendLine('# Paradigm Matrix Evaluation Summary')
$null = $sb.AppendLine('')
$null = $sb.AppendLine("- Time: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')")
$null = $sb.AppendLine("- Base URL: $baseUrl")
$null = $sb.AppendLine('- Datasets: clean(5), large(20)')
$null = $sb.AppendLine('- Rounds: 3 runs per (dataset, mode)')
$null = $sb.AppendLine('')
$null = $sb.AppendLine('| dataset | mode | rounds | Recall(avg+/-std) | Precision(avg) | Hit(avg) | MRR(avg) | nDCG(avg) | KW(avg) | P95(avg+/-std, ms) | Error(avg) |')
$null = $sb.AppendLine('|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|')

foreach ($r in $rows) {
  $line = '| {0} | {1} | {2} | {3} +/- {4} | {5} | {6} | {7} | {8} | {9} | {10} +/- {11} | {12} |' -f `
    $r.dataset, $r.mode, $r.rounds, $r.recall_avg, $r.recall_std, $r.precision_avg, $r.hit_avg, $r.mrr_avg, $r.ndcg_avg, $r.kw_avg, $r.p95_avg, $r.p95_std, $r.err_avg
  $null = $sb.AppendLine($line)
}

$null = $sb.AppendLine('')
$null = $sb.AppendLine('## Notes')
$null = $sb.AppendLine('')
$null = $sb.AppendLine('- Prefer large dataset numbers in interviews; keep clean set as smoke regression.')
$null = $sb.AppendLine('- If recall is saturated, show precision/keyword-rate/latency together to avoid one-metric bias.')
$null = $sb.AppendLine('- Add BEIR subset results for external benchmark comparability.')

Set-Content -Path $summaryPath -Value $sb.ToString() -Encoding UTF8
Write-Host "summary: $summaryPath"
