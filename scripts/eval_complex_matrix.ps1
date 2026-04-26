param(
  [string]$BaseUrl = 'http://localhost:19090',
  [string]$KbName = 'complex-kb',
  [int]$EvalTimeoutSec = 240
)

$ErrorActionPreference = 'Stop'

$baseUrl = $BaseUrl
$workspace = 'e:\learngo\eino_agent'
$dataset = 'data/eval_complex.jsonl'
$reportDir = 'docs/eval_reports'
$docDir = 'data/benchmark_complex/docs'
$kbId = ''
$ts = Get-Date -Format 'yyyyMMdd_HHmmss'

New-Item -ItemType Directory -Force -Path $reportDir | Out-Null

$kbList = Invoke-RestMethod -Method Get -Uri "$baseUrl/api/v1/knowledge-bases"
$existing = $null
if ($kbList -and $kbList.knowledge_bases) {
  $existing = $kbList.knowledge_bases | Where-Object { $_.name -eq $KbName } | Select-Object -First 1
}
if ($existing) {
  $kbId = $existing.id
  Write-Host "use existing kb: $KbName -> $kbId"
} else {
  $created = Invoke-RestMethod -Method Post -Uri "$baseUrl/api/v1/knowledge-bases" -ContentType 'application/json' -Body (@{ name = $KbName; description = 'complex eval dataset' } | ConvertTo-Json)
  $kbId = $created.id
  Write-Host "created kb: $KbName -> $kbId"
}

Write-Host "== [1/4] 上传复杂语料到知识库路径: $kbId =="
Get-ChildItem -Path $docDir -File | ForEach-Object {
  $resp = curl.exe -sS -X POST "$baseUrl/api/v1/knowledge-bases/$kbId/documents" -F "file=@$($_.FullName)"
  Write-Host "uploaded: $($_.Name) -> $resp"
  if ($resp -match '"error"') {
    throw "Upload failed for $($_.Name): $resp"
  }
}

$retrievalConfigs = @(
  @{ name='vector_only';      enable_hybrid=$false; enable_rerank=$false; enable_rewrite=$false },
  @{ name='vector_rerank';    enable_hybrid=$false; enable_rerank=$true;  enable_rewrite=$false },
  @{ name='hybrid_rrf';       enable_hybrid=$true;  enable_rerank=$false; enable_rewrite=$false },
  @{ name='hybrid_rrf_rerank';enable_hybrid=$true;  enable_rerank=$true;  enable_rewrite=$false }
)

$modes = @('pipeline','agentic')
$rows = @()

function Parse-Metric([string]$content,[string]$name){
  $m = [regex]::Match($content, [regex]::Escape($name) + ':\s*([0-9\.]+)')
  if($m.Success){ return [double]$m.Groups[1].Value }
  return [double]::NaN
}

Write-Host "== [2/4] 跑矩阵评测（模式 x 检索策略） =="
foreach($rc in $retrievalConfigs){
  $settings = @{ rag = @{ enable_hybrid = $rc.enable_hybrid; enable_rerank = $rc.enable_rerank; enable_rewrite = $rc.enable_rewrite } } | ConvertTo-Json -Depth 6
  $null = Invoke-RestMethod -Method Put -Uri "$baseUrl/api/v1/settings" -ContentType 'application/json' -Body $settings
  Write-Host "set retrieval config: $($rc.name)"

  foreach($mode in $modes){
    $reportPath = "$reportDir/${ts}_complex_${mode}_$($rc.name).md"
    go run ./cmd/eval -input $dataset -mode $mode -base-url $baseUrl -timeout $EvalTimeoutSec -report $reportPath | Out-Null
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

Write-Host "== [3/4] 生成总览报告 =="
$summary = "$reportDir/${ts}_complex_matrix_summary.md"
$sb = New-Object System.Text.StringBuilder
$null = $sb.AppendLine('# Complex RAG Matrix Summary')
$null = $sb.AppendLine('')
$null = $sb.AppendLine("- Base URL: $baseUrl")
$null = $sb.AppendLine("- Dataset: $dataset")
$null = $sb.AppendLine('')
$null = $sb.AppendLine('| Mode | Retrieval | Recall@K | Hit@K | MRR@K | nDCG@K | Keyword Rate | P95(ms) | Error |')
$null = $sb.AppendLine('|---|---|---:|---:|---:|---:|---:|---:|---:|')
foreach($r in $rows){
  $null = $sb.AppendLine("| $($r.mode) | $($r.retrieval) | $($r.recall) | $($r.hit) | $($r.mrr) | $($r.ndcg) | $($r.kw) | $($r.p95) | $($r.err) |")
}
$null = $sb.AppendLine('')
$null = $sb.AppendLine('## Interpretation Tips')
$null = $sb.AppendLine('- Focus on relative changes in Keyword Rate, MRR, and nDCG, not perfect 100% scores.')
$null = $sb.AppendLine('- Conflict and noisy questions usually reveal the biggest gap between pipeline and agentic modes.')
$null = $sb.AppendLine('- If all scores are high, increase conflicting document versions and noisy negative examples.')
Set-Content -Path $summary -Value $sb.ToString() -Encoding UTF8

Write-Host "== [4/4] 完成 =="
Write-Host "summary: $summary"
