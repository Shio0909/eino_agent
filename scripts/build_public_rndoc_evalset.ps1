param(
  [string]$BaseUrl = 'http://localhost:8080',
  [string]$KbName,
  [string]$SeedPath,
  [string]$OutputPath
)

$ErrorActionPreference = 'Stop'

if (-not $KbName) {
  throw 'KbName is required.'
}
if (-not $SeedPath) {
  throw 'SeedPath is required.'
}
if (-not $OutputPath) {
  throw 'OutputPath is required.'
}

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
$kbList = Invoke-RestMethod -Method Get -Uri "$base/api/v1/knowledge-bases"
$existing = $null
if ($kbList -and $kbList.knowledge_bases) {
  $existing = $kbList.knowledge_bases | Where-Object { $_.name -eq $KbName } | Select-Object -First 1
}

if (-not $existing) {
  throw "Knowledge base not found: $KbName"
}

$kbId = $existing.id
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

Write-Host "saved: $OutputPath"
Write-Host "snapshot: $snapshot"
Write-Host "kb_id=$kbId"