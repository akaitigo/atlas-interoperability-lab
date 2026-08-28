# Core Definitive Gate v2 Preview

## 状態

この契約は`feature/definitive-gate-v2`専用の準備実装です。Core v2は未確定であり、`2.0.0-draft.1`、`status: draft`として固定します。Labの`main`、既存v1 Composition、Evidence、Completion Certificateをv2へ昇格・上書きしません。

Definitive評価とMigrationの前提としてInterop Non-Regression Gateを必ず実行します。既存統合範囲を縮小して全SubjectがDefinitiveに見えるよう加工することはできません。またComposition内の1 Subjectでも`subject-definitive`未達ならInterop全体をDefinitiveへ昇格しません。

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
