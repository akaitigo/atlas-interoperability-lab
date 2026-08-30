# Composition Evidence Dependency Closure

`evidence/preview/composition-evidence-dependency.json`は、Core正式main commit `072d7ca77981f51754e824d70c6d4ecd55ea67e5`のEvidence Dependency Graph portable predicateをComposition levelへ適用します。Core CLI consumer Matrixは別に維持し、Subject固有件数やprofileを共通閾値へしません。

このFixtureのinputはRepository Contract、Composition／固定Subject Release、Interop Harness、Go Runtime contract、local Profile、container Profileの6 nodeです。`repo.yaml`は正規fleet rolloutの書込み境界として独立nodeへ固定し、変更時は両Runtimeとderived Closureを再実行するまでcurrentへ戻しません。Harness memberは`internal/lab`の全non-test Go fileを機械列挙するため、新しい実行・判定codeをGraph外へ置いたままcurrentを名乗れません。outputは両Profileのbinding、各5 Scenario、summary、Cleanup、複数Subject Compatibility Matrix、Actual Subject Admission／Negative Matrix、Runtime Proof Index、Closure Planの21件です。

3つのrunを分離します。

- local Runtime run: local processの実行窓、OS／architecture／Go version／binary digest、Repository Contractを含む5 input binding、8 output
- container Runtime run: Docker runtime identity、実行窓、Repository Contractを含む5 input binding、8 output
- Composition derived run: 両Profile outputを祖先に持つCompatibility、Proof Index、Closure Plan

`composition-evidence-closure`は正本worktreeでRepository ContractとNon-Regression Gateを先に通し、その後はRepositoryの一時copy内で両Runtimeと固定Core consumer Matrixを実行します。Graph、output digest、11-case Matrixが通った場合だけ19 Graph output、Graph、Matrix結果をpublishします。外部Core／FE Repositoryは固定Git Objectの読取だけに使います。

`make check`のlocal／container E2E、再現性、失敗診断は`checkpoint-runtime`、Publication／Certificate／Core auditは`checkpoint-publication`でtask-owned一時copyへ隔離します。v1 bounded Evidenceを現在Harnessの出力で上書きせず、各runが作成したProcess、Container、Network、Imageと一時copyだけを明示Cleanupして残存0を検証します。

negative Matrixはcurrent正例1件と拒否10件です。stale status、HarnessまたはRepository Contract変更後のdigest-only closure、localまたはcontainerの再実行対象漏れ、output withdrawal、Proof構造縮小、Closure Plan構造縮小、gapを消したDefinitive昇格、保存Bindingと一致しないRuntime Identityを拒否します。

Graphの状態は`incomplete`、`definitive_eligible: false`です。process executableの独立attestationは実process／container観測とMigration Evidenceにより閉鎖済みですが、実Runtime成功は次の不足を閉じません。

- `subject-depth-parity-incomplete`: 各Subjectの18軸Depth Parity
- `subject-v2-certificate-atomic-binding-unavailable`: Subject v2 Certificateとのatomic binding
- `surface-pattern-proof-gaps`: Surface/Pattern固有Proof gap

```bash
go run ./cmd/atlas-lab composition-evidence-closure
go run ./cmd/atlas-lab composition-evidence-audit
```

Docker volume、ユーザーデータ、既存v1 Evidenceは削除しません。Cleanup対象は各runが作成したProcess、Container、Network、Imageだけです。
