$ErrorActionPreference='Stop'
$base='http://localhost:8080'

Write-Host '== Step 1: purge all KBs =='
$kbResp=Invoke-RestMethod -Method Get -Uri "$base/api/v1/knowledge-bases"
$kbs=@($kbResp.knowledge_bases)
foreach($kb in $kbs){
  try {
    Invoke-RestMethod -Method Delete -Uri "$base/api/v1/knowledge-bases/$($kb.id)" | Out-Null
    Write-Host "deleted kb: $($kb.id) $($kb.name)"
  } catch {
    Write-Host "delete failed: $($kb.id) $($_.Exception.Message)"
  }
}

Write-Host '== Step 2: create clean KB =='
$now=Get-Date -Format 'yyyyMMdd_HHmmss'
$newKbName="public-go-clean-$now"
$createBody=@{name=$newKbName;description='clean public go benchmark corpus'} | ConvertTo-Json
$newKb=Invoke-RestMethod -Method Post -Uri "$base/api/v1/knowledge-bases" -ContentType 'application/json' -Body $createBody
$kbId=$newKb.id
Write-Host "new kb: $kbId ($newKbName)"

Write-Host '== Step 3: import URLs =='
$urls=@(
  @{url='https://go.dev/doc/install';title='Go Install'},
  @{url='https://go.dev/doc/tutorial/getting-started';title='Go Getting Started'}
)
foreach($u in $urls){
  $body=@{url=$u.url;title=$u.title;enable_multimodal=$false} | ConvertTo-Json
  $r=Invoke-RestMethod -Method Post -Uri "$base/api/v1/knowledge-bases/$kbId/documents/url" -ContentType 'application/json' -Body $body
  Write-Host "imported: $($u.url) chunks=$($r.chunk_count) status=$($r.status)"
}

Write-Host '== Step 4: build clean eval set from live refs =='
$items=@(
  @{id='pubc_q1';q='How do I install Go on Linux?';k=@('/usr/local/go','PATH','go version');cat='public-go-clean'},
  @{id='pubc_q2';q='How can I verify Go installation?';k=@('go version');cat='public-go-clean'},
  @{id='pubc_q3';q='What command can remove a previous Go installation on Linux?';k=@('rm -rf /usr/local/go');cat='public-go-clean'},
  @{id='pubc_q4';q='How do I run the first Go program after installation?';k=@('go run','hello.go');cat='public-go-clean'},
  @{id='pubc_q5';q='What does go mod init do in the getting started tutorial?';k=@('go.mod','module');cat='public-go-clean'}
)
$out=@()
foreach($it in $items){
  $body=@{message=$it.q; mode='pipeline'} | ConvertTo-Json
  $resp=Invoke-RestMethod -Method Post -Uri "$base/api/v1/chat" -ContentType 'application/json' -Body $body
  $refs=@()
  if($resp.references){
    $refs=@($resp.references | ForEach-Object { $_.id } | Select-Object -Unique | Select-Object -First 2)
  }
  $obj=[ordered]@{id=$it.id;question=$it.q;gold_docs=$refs;answer_keywords=$it.k;category=$it.cat}
  $out += (($obj | ConvertTo-Json -Compress -Depth 6))
  Write-Host "$($it.id) gold_docs=$($refs.Count)"
}
$outPath='data/eval_public_go_clean.jsonl'
$out | Set-Content -Path $outPath -Encoding UTF8
$ts=Get-Date -Format 'yyyyMMdd_HHmmss'
$snapshot="data/eval_public_go_clean_$ts.jsonl"
Copy-Item $outPath $snapshot -Force
Write-Host "saved: $outPath"
Write-Host "snapshot: $snapshot"
Write-Host "kb_id=$kbId"
