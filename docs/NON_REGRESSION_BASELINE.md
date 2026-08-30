# Interop Non-Regression Baseline

## 不変条件

Interopや組合せが構成要素の不足を隠して全体を完了扱いすることを禁止します。Baselineは`main@1894452124e722955576a6a3744da7744c68f6f9`のGit Objectから直接読み、working treeの宣言だけでは弱められません。

次をBaselineの同等以上として維持します。

- 5個の独立Scenarioと各Phase／Action ID
- HTTP Status、JSON Assertion、`gte`閾値、Compare、Capture
- 2 Subject、固定Version、Release／Certificate Digest、local／container Profile
- 既存Schema、Oracle、Claim／Graph
- Go Test、Router Eval、Integration Harness、実Fixture Subject
- 正常・拒否・障害・回復・互換性EvidenceとCleanup Receipt
- 既存CI Step／Command
- 正規fleet `repo.yaml`とGitHub allowed／Cloud denied／既存Repository deniedの書込み境界

追加は許可します。削除、`skip`／`disabled`／`optional`化、Scope外退避、Scenario集約、Assertion・閾値・Component・Version・CI縮小、実統合からmock／staticへの置換、失敗または回復Evidence削除は拒否します。

## 置換

置換は`migrations/interop-v1-non-regression.json`に旧IDと旧Pathを記録し、次を全て満たす場合だけ許可します。

- 新IDと新PathがCompositionから実際に参照される。
- 旧Baselineに存在しない、`pass`のlocal／container統合Evidenceが各1件以上ある。
- 旧Baselineに存在しない、`kind: migration`のEvidenceが1件以上ある。
- 全EvidenceがPathとSHA-256 Digestで固定される。

```bash
go run ./cmd/atlas-lab non-regression-gate
```

拒否Fixture Matrixは22件あり、Repository Contract削除・境界緩和、証拠なしMapping、単一ProfileだけのProof、宣伝的な記述、作者評価も拒否します。別の正例Testでは、旧ID Mapping、新Scenario参照、local／containerの統合Proof、Migration Evidenceが揃った同等置換だけを受理します。
