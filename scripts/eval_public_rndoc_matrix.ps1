param(
  [string]$BaseUrl = 'http://localhost:8080',
  [string]$Dataset = 'data/eval_public_rndoc.jsonl',
  [string]$KnowledgeBaseIDs = '',
  [int]$EvalTimeoutSec = 240
)

$ErrorActionPreference = 'Stop'

$baseUrl = $BaseUrl.TrimEnd('/')
$reportDir = 'docs/eval_reports'
$ts = Get-Date -Format 'yyyyMMdd_HHmmss'

New-Item -ItemType Directory -Force -Path $reportDir | Out-Null

$retrievalConfigs = @(
  @{ name='vector_only';       enable_hybrid=$false; enable_rerank=$false; enable_rewrite=$false },
  @{ name='vector_rerank';     enable_hybrid=$false; enable_rerank=$true;  enable_rewrite=$false },
  @{ name='hybrid_rrf';        enable_hybrid=$true;  enable_rerank=$false; enable_rewrite=$false },
  @{ name='hybrid_rrf_rerank'; enable_hybrid=$true;  enable_rerank=$true;  enable_rewrite=$false }
)

$modes = @('pipeline','agent','agentic_rag')
$rows = @()

function Parse-Metric([string]$content, [string]$name) {
  $m = [regex]::Match($content, [regex]::Escape($name) + ':\s*([0-9\.]+)')
  if ($m.Success) { return [double]$m.Groups[1].Value }
  return [double]::NaN
}

Write-Host '== [1/2] run public engineering docs matrix =='
foreach ($rc in $retrievalConfigs) {
  $settings = @{ rag = @{ enable_hybrid = $rc.enable_hybrid; enable_rerank = $rc.enable_rerank; enable_rewrite = $rc.enable_rewrite } } | ConvertTo-Json -Depth 6
  $null = Invoke-RestMethod -Method Put -Uri "$baseUrl/api/v1/settings" -ContentType 'application/json' -Body $settings
  Write-Host "set retrieval config: $($rc.name)"

  foreach ($mode in $modes) {
    $reportPath = "$reportDir/${ts}_public_rndoc_${mode}_$($rc.name).md"
    $args = @('./cmd/eval', '-input', $Dataset, '-mode', $mode, '-base-url', $baseUrl, '-timeout', $EvalTimeoutSec, '-report', $reportPath)
    if ($KnowledgeBaseIDs.Trim()) {
      $args += @('-knowledge-base-ids', $KnowledgeBaseIDs)
    }
    go run @args | Out-Null
    $content = Get-Content $reportPath -Raw

    $rows += [pscustomobject]@{
      mode = $mode
      retrieval = $rc.name
      recall = [Math]::Round((Parse-Metric $content 'Recall@K'), 4)
      hit = [Math]::Round((Parse-Metric $content 'Hit@K'), 4)
      mrr = [Math]::Round((Parse-Metric $content 'MRR@K'), 4)
      ndcg = [Math]::Round((Parse-Metric $content 'nDCG@K'), 4)
      kw = [Math]::Round((Parse-Metric $content 'Answer Keyword Rate'), 4)
      p95 = [Math]::Round((Parse-Metric $content 'P95 Latency (ms)'), 2)
      err = [Math]::Round((Parse-Metric $content 'Error Rate'), 4)
      report = $reportPath
    }
    Write-Host "done: mode=$mode retrieval=$($rc.name)"
  }
}

Write-Host '== [2/2] write summary =='
$summary = "$reportDir/${ts}_public_rndoc_matrix_summary.md"
$sb = New-Object System.Text.StringBuilder
$null = $sb.AppendLine('# Public Engineering Docs Matrix Summary')
$null = $sb.AppendLine('')
$null = $sb.AppendLine("- Base URL: $baseUrl")
$null = $sb.AppendLine("- Dataset: $Dataset")
if ($KnowledgeBaseIDs.Trim()) {
  $null = $sb.AppendLine("- KnowledgeBaseIDs: $KnowledgeBaseIDs")
}
$null = $sb.AppendLine('')
$null = $sb.AppendLine('| Mode | Retrieval | Recall@K | Hit@K | MRR@K | nDCG@K | Keyword Rate | P95(ms) | Error |')
$null = $sb.AppendLine('|---|---|---:|---:|---:|---:|---:|---:|---:|')
foreach ($r in $rows) {
  $null = $sb.AppendLine("| $($r.mode) | $($r.retrieval) | $($r.recall) | $($r.hit) | $($r.mrr) | $($r.ndcg) | $($r.kw) | $($r.p95) | $($r.err) |")
}
Set-Content -Path $summary -Value $sb.ToString() -Encoding UTF8
Write-Host "summary: $summary"