param(
  [string]$BaseUrl = 'http://localhost:8080',
  [string]$KbName = 'public-rndoc-benchmark',
  [string]$UrlManifest = 'data/public_rndoc_urls.json',
  [string]$SeedPath = 'data/eval_public_rndoc_seed.jsonl',
  [string]$OutputPath = 'data/eval_public_rndoc.jsonl',
  [switch]$PurgeAllKBs
)

$ErrorActionPreference = 'Stop'

function Read-JsonLines([string]$path) {
  $items = @()
  Get-Content -Path $path | ForEach-Object {
    $line = $_.Trim()
    if ($line) {
      $items += ($line | ConvertFrom-Json)
    }
  }
  return $items
}

function Get-ReferenceIds($resp) {
  $ids = @()
  if ($resp.references) {
    $ids += @($resp.references | ForEach-Object { $_.id })
  }
  if ($resp.sources) {
    $ids += @($resp.sources | ForEach-Object { if ($_.doc_id) { $_.doc_id } elseif ($_.id) { $_.id } })
  }
  return @($ids | Where-Object { $_ } | Select-Object -Unique | Select-Object -First 3)
}

$base = $BaseUrl.TrimEnd('/')
$reportDir = 'docs/eval_reports'
New-Item -ItemType Directory -Force -Path $reportDir | Out-Null

if ($PurgeAllKBs) {
  Write-Host '== Step 1/5: purge all KBs =='
  $kbResp = Invoke-RestMethod -Method Get -Uri "$base/api/v1/knowledge-bases"
  $kbs = @($kbResp.knowledge_bases)
  foreach ($kb in $kbs) {
    Invoke-RestMethod -Method Delete -Uri "$base/api/v1/knowledge-bases/$($kb.id)" | Out-Null
    Write-Host "deleted kb: $($kb.id) $($kb.name)"
  }
} else {
  Write-Host '== Step 1/5: keep existing KBs (for isolated benchmark, prefer -PurgeAllKBs in a dedicated env) =='
}

Write-Host '== Step 2/5: create or reuse benchmark KB =='
$kbList = Invoke-RestMethod -Method Get -Uri "$base/api/v1/knowledge-bases"
$existing = $null
if ($kbList -and $kbList.knowledge_bases) {
  $existing = $kbList.knowledge_bases | Where-Object { $_.name -eq $KbName } | Select-Object -First 1
}
if ($existing) {
  $kbId = $existing.id
  Write-Host "use existing kb: $KbName -> $kbId"
} else {
  $createBody = @{ name = $KbName; description = 'public engineering docs benchmark corpus' } | ConvertTo-Json
  $newKb = Invoke-RestMethod -Method Post -Uri "$base/api/v1/knowledge-bases" -ContentType 'application/json' -Body $createBody
  $kbId = $newKb.id
  Write-Host "created kb: $KbName -> $kbId"
}

Write-Host '== Step 3/5: import public documents via URL =='
$urls = Get-Content -Path $UrlManifest -Raw | ConvertFrom-Json
foreach ($u in $urls) {
  $body = @{ url = $u.url; title = $u.title; enable_multimodal = $false } | ConvertTo-Json
  try {
    $r = Invoke-RestMethod -Method Post -Uri "$base/api/v1/knowledge-bases/$kbId/documents/url" -ContentType 'application/json' -Body $body
    Write-Host "imported: [$($u.domain)] $($u.title) chunks=$($r.chunk_count) status=$($r.status)"
  } catch {
    Write-Host "import failed: [$($u.domain)] $($u.title) -> $($_.Exception.Message)"
  }
}

Write-Host '== Step 4/5: build eval set from live references =='
$seedItems = Read-JsonLines $SeedPath
$output = @()
foreach ($it in $seedItems) {
  $body = @{ message = $it.question; mode = 'pipeline'; knowledge_base_ids = @($kbId) } | ConvertTo-Json
  try {
    $resp = Invoke-RestMethod -Method Post -Uri "$base/api/v1/chat" -ContentType 'application/json' -Body $body
    $refs = Get-ReferenceIds $resp
  } catch {
    $refs = @()
    Write-Host "query failed: $($it.id) -> $($_.Exception.Message)"
  }

  $obj = [ordered]@{
    id = $it.id
    question = $it.question
    gold_docs = @($refs)
    answer_keywords = @($it.answer_keywords)
    category = $it.category
    expected_answer = $it.expected_answer
    judge_rule = $it.judge_rule
    manual_label = $it.manual_label
  }
  $output += (($obj | ConvertTo-Json -Compress -Depth 6))
  Write-Host "$($it.id) gold_docs=$($refs.Count)"
}

$output | Set-Content -Path $OutputPath -Encoding UTF8
$ts = Get-Date -Format 'yyyyMMdd_HHmmss'
$snapshot = $OutputPath.Replace('.jsonl', "_$ts.jsonl")
Copy-Item $OutputPath $snapshot -Force

Write-Host '== Step 5/5: done =='
Write-Host "saved: $OutputPath"
Write-Host "snapshot: $snapshot"
Write-Host "kb_id=$kbId"
