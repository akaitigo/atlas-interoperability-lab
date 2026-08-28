# Core Definitive Gate v2 Preview

## 状態

この契約は`feature/definitive-gate-v2`専用の準備実装です。Core v2は未確定であり、`2.0.0-draft.1`、`status: draft`として固定します。Labの`main`、既存v1 Composition、Evidence、Completion Certificateをv2へ昇格・上書きしません。

Definitive評価とMigrationの前提としてInterop Non-Regression Gateを必ず実行します。既存統合範囲を縮小して全SubjectがDefinitiveに見えるよう加工することはできません。またComposition内の1 Subjectでも`subject-definitive`未達ならInterop全体をDefinitiveへ昇格しません。

## Depth Parity

`frontend-behavior-atlas@deadad18b6588d2c907170a451c3b5cea5ea4192`の`FE_DEPTH_REFERENCE.json`をGit ObjectとSHA-256で固定します。正本は18軸中`satisfied=1`、`partial=17`、`status=incomplete`です。Labへ本文を複製せず、`depth/fe-depth-reference.lock.json`から外部Git Objectを検証します。

`depth/fixture-subjects.depth-parity.preview.json`は各構成Subjectの18軸状態を分離して宣言します。`depth/fixture-stage2.integration-proofs.preview.json`はInterop固有10軸をlocal／containerの実Scenario Evidenceへ接続します。後者が全件passでも前者の不足は解消されず、`subject-depth-parity-incomplete`と`integration-proof-cannot-promote-depth-gap`により`incomplete`を保持します。

`depth/fe-scenario-contract.lock.json`は同CommitのScenario Indexと実Browser統合結果を固定します。統合Scenario 10/10 passとSurface/Pattern 850 rowは別の分母です。850 rowには専用Artifactがありますが、Pattern固有Evidenceは429、Runtime Identityは170、Capture Evidenceは259、明示gapは421、Atomic Authority Bindingは0、completion eligibleは0です。GateはIndex内の850個のDigestとProof本文を全件照合し、各rowに固有Evidence、Runtime Identity、Atomic Authority Binding、または明示gapを要求します。共通の統合Traceだけを全rowの専用Proofとして扱わず、`surface-pattern-proof-gaps`と`integrated-trace-not-component-proof`により状態を昇格させません。

Migrationは各Subjectの18軸、850 Surface/Pattern rowの個別Closure、v2 Certificate更新を未完了条件として返します。旧v1 Bundleは上書きせず、Interop Proofや統合TraceをSubject Depthまたは個別rowの代替Evidenceとして流用しません。

Core v2確定後は、確定Commit、正式Schema、正式Certificate検証器へLockを更新し、このPreview SchemaとのCompatibility差分をMigrationとして記録します。それまでは`definitive-complete`を発行しません。

## 分離するCompletion Class

| Class | 意味 | `excluded` / `infeasible` | `partial` | v1 Certificate |
|---|---|---|---|---|
| `bounded-complete` | 固定Coverage Epoch内の完了 | 警告付きで許容 | 不許可 | 旧Bundleとして検証可能 |
| `subject-definitive` | Subject全体でv2追加Gateを満たす候補 | 不許可 | 不許可 | 不許可 |

`subject-definitive`で全構成要素がv2、active、有効期間内、Coverageが`covered`だけの場合も、Core v2確定前の実効状態は`definitive-candidate`です。条件を満たさない構成は`bounded-complete`または`incomplete`へ降格し、`state-downgraded`を返します。

## Fixture Matrix

`tests/fixtures/definitive-gate-v2.matrix.json`は次を固定入力として検証します。

- 全v2 definitive構成（候補止まり）
- bounded + excluded
- definitive + excluded / infeasible / partial
- v1-only Certificate
- v1/v2 Certificate混在
- Certificate失効
- 失効Digestを指定した更新
- bounded + partial

```bash
go run ./cmd/atlas-lab definitive-matrix
go run ./cmd/atlas-lab definitive-migrate
go run ./cmd/atlas-lab definitive-preview-audit
go run ./cmd/atlas-lab legacy-v1-check
python3 evals/run_definitive_v2.py
```

Migrationは`writes_performed: false`で更新計画だけを返します。旧v1 BundleはCompositionに固定されたCore v1 Commitを一時展開して検証するため、現在のCore branchや未完成v2作業ツリーから独立しています。

Preview結果は`evidence/preview/`へ保存し、v1の`evidence/completion-certificate.json`およびPublication Gateへ混入させません。Preview Auditはfeature branch、Matrix、Migration、Router、旧v1 Bundle、Core draft Lockを一括判定します。
