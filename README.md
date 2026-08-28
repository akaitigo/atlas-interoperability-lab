# Atlas Interoperability Lab

`atlas-interoperability-lab`は、完成済みSubject Releaseだけを固定VersionとDigestで組み合わせるStage 2用の中立Lab Frameworkです。個別SubjectのAuthority、機能責任、実装知識を再所有せず、組合せに現れる通信、Identity、Data、Messaging、Deployment、Observability、Security Boundary、Failure Propagation、Recovery、Compatibilityを検証します。

## 完成境界

正本は次の機械可読Chainです。

```text
Composition Manifest
  -> Subject Release + Completion Certificate
  -> Environment Profile
  -> Scenario
  -> Oracle
  -> Evidence
  -> Cross-subject Claim/Evidence Graph
  -> Cleanup Receipt
  -> Publication Gate
  -> Completion Certificate
```

Compositionは`reference-atlas-core`のCommit、SubjectのVersion、Release Manifest Digest、Completion Certificate Digestを固定します。Floating Branch、Source Tree依存、Git submodule、`status != complete`のSubjectはPreflightで拒否します。

## Fixture End-to-End

`fixture-http-source@v1.0.0`と`fixture-http-sink@v1.0.0`は、中立なHTTP/JSON/Bearer/Trace Primitiveだけを実証する完成Fixture Releaseです。次を同じScenarioとOracleで`local`と`container`の両Profileから実行します。

- `normal`: 認証、Message伝送、Data保存、Trace相関
- `rejection`: 無効Identityを拒否し、下流状態が不変
- `failure`: 隔離したSink障害を502とTraceで伝播
- `recovery`: 障害解除後に同じCompositionで正常化
- `compatibility`: 未対応Schema Versionを明示的に拒否

`cloud-live`は主要Claimに不要な任意Profileで、既定では実行しません。

## 実行

前提はGo 1.26です。Container ProfileにはDocker Engineも必要です。

```bash
make check
```

個別実行:

```bash
go run ./cmd/atlas-lab validate
go run ./cmd/atlas-lab preflight --composition compositions/fixture-stage2.json --profile local
go run ./cmd/atlas-lab run --composition compositions/fixture-stage2.json --profile local
go run ./cmd/atlas-lab run --composition compositions/fixture-stage2.json --profile container
go run ./cmd/atlas-lab diagnose --profile container
python3 evals/run.py
go run ./cmd/atlas-lab publication-gate
go run ./cmd/atlas-lab certificate
go run ./cmd/atlas-lab self-audit
```

Cleanupは成功・失敗に関係なくRunnerが実行します。`cleanup/*.receipt.json`は残存Process、Container、Network、Image、Credentialがゼロであることを証明します。

## Publication Gate

Apache-2.0、NOTICE、第三者Manifest、SPDX SBOM、Core Lock、Fixture Release Lock、全Subjectを横断するClaim/Evidence Graph、local/container Evidence、Router Eval、完全Cleanup、秘密Pattern Scanが全てpassするまでCertificateを生成しません。権利不明、秘密候補、未完成Subject、Digest不一致はいずれも公開拒否です。

`self-audit`はRepository契約、Publication Gate、Core Completion Certificate、Core Audit、全CommitのDCO、Clean Worktreeを機械可読JSONで一括判定します。GitHub Remoteや公開可否はこのローカル完成判定とは分離します。

詳細は[Architecture](docs/ARCHITECTURE.md)、[Machine-readable Contract](docs/CONTRACT.md)、[Runbook](operations/RUNBOOK.md)を参照してください。
