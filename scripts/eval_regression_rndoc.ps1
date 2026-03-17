param(
  [string]$BaseUrl = 'http://localhost:19090',
  [string]$KbName = 'regression-rndoc-kb',
  [string]$Mode = 'pipeline',
  [string]$Retrieval = 'hybrid_rrf_rerank',
  [int]$EvalTimeoutSec = 180
)

$ErrorActionPreference = 'Stop'

$baseUrl = $BaseUrl
$dataset = 'data/eval_regression_rndoc.jsonl'
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
  $created = Invoke-RestMethod -Method Post -Uri "$baseUrl/api/v1/knowledge-bases" -ContentType 'application/json' -Body (@{ name = $KbName; description = 'regression dataset for engineering docs' } | ConvertTo-Json)
  $kbId = $created.id
  Write-Host "created kb: $KbName -> $kbId"
}

Write-Host "== [1/3] 上传回归语料到知识库: $kbId =="
Get-ChildItem -Path $docDir -File | ForEach-Object {
  $resp = curl.exe -sS -X POST "$baseUrl/api/v1/knowledge-bases/$kbId/documents" -F "file=@$($_.FullName)"
  Write-Host "uploaded: $($_.Name) -> $resp"
  if ($resp -match '"error"') {
    throw "Upload failed for $($_.Name): $resp"
  }
}

$settingsMap = @{
  'vector_only' = @{ enable_hybrid = $false; enable_rerank = $false; enable_rewrite = $false }
  'vector_rerank' = @{ enable_hybrid = $false; enable_rerank = $true; enable_rewrite = $false }
  'hybrid_rrf' = @{ enable_hybrid = $true; enable_rerank = $false; enable_rewrite = $false }
  'hybrid_rrf_rerank' = @{ enable_hybrid = $true; enable_rerank = $true; enable_rewrite = $false }
}

if (-not $settingsMap.ContainsKey($Retrieval)) {
  throw "unknown retrieval config: $Retrieval"
}

$settings = @{ rag = $settingsMap[$Retrieval] } | ConvertTo-Json -Depth 6
$null = Invoke-RestMethod -Method Put -Uri "$baseUrl/api/v1/settings" -ContentType 'application/json' -Body $settings

Write-Host "== [2/3] 执行回归评测 =="
$reportPath = "$reportDir/${ts}_regression_${Mode}_$Retrieval.md"
go run ./cmd/eval -input $dataset -mode $Mode -base-url $baseUrl -timeout $EvalTimeoutSec -report $reportPath | Out-Null

Write-Host "== [3/3] 完成 =="
Write-Host "report: $reportPath"