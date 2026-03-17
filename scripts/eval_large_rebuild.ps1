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

Write-Host '== Step 2: create large benchmark KB =='
$now=Get-Date -Format 'yyyyMMdd_HHmmss'
$newKbName="public-go-large-$now"
$createBody=@{name=$newKbName;description='large public go benchmark corpus'} | ConvertTo-Json
$newKb=Invoke-RestMethod -Method Post -Uri "$base/api/v1/knowledge-bases" -ContentType 'application/json' -Body $createBody
$kbId=$newKb.id
Write-Host "new kb: $kbId ($newKbName)"

Write-Host '== Step 3: import URLs (large corpus) =='
$urls=@(
  @{url='https://go.dev/doc/install';title='Go Install'},
  @{url='https://go.dev/doc/tutorial/getting-started';title='Go Getting Started'},
  @{url='https://go.dev/doc/tutorial/create-module';title='Go Create Module'},
  @{url='https://go.dev/doc/tutorial/workspaces';title='Go Workspaces'},
  @{url='https://go.dev/doc/tutorial/add-a-test';title='Go Add A Test'},
  @{url='https://go.dev/doc/tutorial/fuzz';title='Go Fuzzing'},
  @{url='https://go.dev/doc/modules/managing-dependencies';title='Go Managing Dependencies'},
  @{url='https://go.dev/doc/effective_go';title='Effective Go'}
)

foreach($u in $urls){
  $body=@{url=$u.url;title=$u.title;enable_multimodal=$false} | ConvertTo-Json
  $r=Invoke-RestMethod -Method Post -Uri "$base/api/v1/knowledge-bases/$kbId/documents/url" -ContentType 'application/json' -Body $body
  Write-Host "imported: $($u.url) chunks=$($r.chunk_count) status=$($r.status)"
}

Write-Host '== Step 4: build larger eval set from live refs =='
$items=@(
  @{id='lq1';q='How do I install Go on Linux?';k=@('/usr/local/go','PATH','go version');cat='public-go-large'},
  @{id='lq2';q='How can I verify the Go installation?';k=@('go version');cat='public-go-large'},
  @{id='lq3';q='What command removes a previous Go installation on Linux?';k=@('rm -rf /usr/local/go');cat='public-go-large'},
  @{id='lq4';q='How do I run a Go source file from the command line?';k=@('go run');cat='public-go-large'},
  @{id='lq5';q='What does go mod init do?';k=@('go.mod','module');cat='public-go-large'},
  @{id='lq6';q='How do I add a dependency in a Go module?';k=@('go get','require');cat='public-go-large'},
  @{id='lq7';q='What file records checksums of module dependencies?';k=@('go.sum');cat='public-go-large'},
  @{id='lq8';q='How do I update a dependency to a newer version?';k=@('go get','@latest');cat='public-go-large'},
  @{id='lq9';q='How do you write a basic Go test file?';k=@('_test.go','testing');cat='public-go-large'},
  @{id='lq10';q='How do I run all tests in a Go module?';k=@('go test ./...');cat='public-go-large'},
  @{id='lq11';q='What is fuzz testing in Go used for?';k=@('fuzz','testing');cat='public-go-large'},
  @{id='lq12';q='How do I start fuzzing for a specific test?';k=@('go test','-fuzz');cat='public-go-large'},
  @{id='lq13';q='What is a Go workspace and which file defines it?';k=@('go.work','workspace');cat='public-go-large'},
  @{id='lq14';q='How do I initialize a Go workspace?';k=@('go work init');cat='public-go-large'},
  @{id='lq15';q='How do I add modules into an existing workspace?';k=@('go work use');cat='public-go-large'},
  @{id='lq16';q='In Effective Go, what naming style is recommended for exported identifiers?';k=@('MixedCaps','exported');cat='public-go-large'},
  @{id='lq17';q='What is the recommended way to format Go code?';k=@('gofmt');cat='public-go-large'},
  @{id='lq18';q='How should errors generally be handled in Go?';k=@('error','if err');cat='public-go-large'},
  @{id='lq19';q='What command tidies module dependencies?';k=@('go mod tidy');cat='public-go-large'},
  @{id='lq20';q='How can I inspect module dependencies in a project?';k=@('go list','-m');cat='public-go-large'}
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

$outPath='data/eval_public_go_large.jsonl'
$out | Set-Content -Path $outPath -Encoding UTF8
$ts=Get-Date -Format 'yyyyMMdd_HHmmss'
$snapshot="data/eval_public_go_large_$ts.jsonl"
Copy-Item $outPath $snapshot -Force
Write-Host "saved: $outPath"
Write-Host "snapshot: $snapshot"
Write-Host "kb_id=$kbId"