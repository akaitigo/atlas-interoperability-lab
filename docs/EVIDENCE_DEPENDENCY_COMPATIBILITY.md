# Evidence Dependency consumer互換性

Interopは`reference-atlas-core`正式main／CI成功commit `072d7ca77981f51754e824d70c6d4ecd55ea67e5`のEvidence Dependency Graph契約を使用します。`compatibility/evidence-dependency-core.lock.json`がCore Version、Commit、CLI、Certificate検証、portable predicateを固定します。作業中のCore worktreeやbranch tipは判定に使用しません。

`go run ./cmd/atlas-lab evidence-dependency-matrix`はCore commitのGit ObjectからCLIと汎用Definitive fixtureを一時生成し、次の2コマンドをCodex、Claude Code、generic CLIの各consumerについて同じ入力へ実行します。

```text
atlas audit <subject-root> --gate evidence-dependency
atlas certificate verify-definitive <subject-root>
```

Matrixはcurrent closureを受理し、次を全consumerで拒否します。

- `status: stale`
- 入力DigestとBindingだけを更新し、変更観測後に再実行しないdigest-only closure
- 影響outputをrunの再実行対象から外す変更
- 既知outputをGraph、required output、runからまとめて退避する変更
- Scenario Proof構造の縮小
- Closure Plan構造の縮小

共通契約にするのは推移stale、変更観測後の実再実行、現在Input Binding、到達可能な全output、Proof／Closure Plan構造不変です。FixtureのBehavior数、Scenario row数、profile名はSubject固有入力として扱い、consumer共通閾値にはしません。

複数SubjectへのComposition規則は`composition-compatibility.matrix.json`で追加検証します。各Subjectの結果を保持したまま構成全体を判定し、片側のrejectまたはCross-Subject Claim link欠落を他のpassで相殺しません。

Fixtureと全派生caseはOSの一時Directoryだけに作成し、検証終了時に回収します。Repository Evidence、Docker Volume、ユーザーデータは変更または削除しません。
