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

検証軸とCanonical Pathの索引が必要な場合だけ [references/index.json](references/index.json) を読みます。
