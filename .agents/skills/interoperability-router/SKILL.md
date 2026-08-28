---
name: interoperability-router
description: 固定済みSubject ReleaseのStage 2相互運用について、該当するComposition、Scenario、Environment、Oracle、Evidenceへ案内する。Subject単体の実装や未完成Releaseの統合には使わない。
---

# Interoperability Router

利用者の問いを `communication`、`identity`、`data`、`messaging`、`deployment`、`observability`、`security-boundary`、`failure-propagation`、`recovery`、`compatibility` の検証軸へRouteしてください。

1. `scripts/route.py --query '<問い>'` を実行し、返されたCanonical Pathだけを開く。
2. 実行を求められた場合は、対象Compositionの固定ReleaseをPreflightしてから提示されたScenarioを実行する。
3. 結論は対応するOracleとEvidenceに束縛する。Subject固有の知識は各Subject Atlasへ戻す。
4. `coverage_gap` または未完成Subjectが返された場合、機能を推測せず拒否理由を示す。
5. 実行失敗を扱う場合は返却された`diagnostic_command`を実行し、秘密を含まない診断Code、Scenario、Action、次の操作を提示する。
6. 完成判定を求められた場合は`self_audit_command`を実行し、Core Audit、Publication Gate、Certificate、DCO、作業ツリーの全結果に束縛する。
7. `bounded/epoch-complete`と`subject-definitive`を同義に扱わない。Definitive Gate v2の問いには返却されたPreview Commandを使い、`core-v2-draft`中は`definitive-candidate`を`definitive-complete`と表現しない。
8. `excluded`、`infeasible`、`partial`、v1-only、混在Certificate、失効を検出したら、返却された警告と降格状態を保持する。Migrationは旧v1 Bundleを上書きせず更新計画だけを提示する。
9. Interopの実行・完成・移行判断より先に`non_regression_command`を通す。既存Scenario、Component、Contract、Test、Failure／Recovery Evidence、CIを削除・無効化・縮小・mock化して不足を隠す変更は拒否し、置換には旧ID Mappingと同等以上の統合ProofとMigration Evidenceを要求する。
10. `depth_reference`と`subject_depth_parity`を読み、各構成Subjectの18軸が全て`satisfied`でない場合はInterop固有ProofやScenario成功で補完しない。`subject-depth-parity-incomplete`と`integration-proof-cannot-promote-depth-gap`を保持する。

検証軸とCanonical Pathの索引が必要な場合だけ [references/index.json](references/index.json) を読みます。
