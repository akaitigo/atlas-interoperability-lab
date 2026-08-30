# Runtime Binding Evidence Preview

このPreviewは、固定Fixture Subject Releaseをlocal processとDocker containerで実行したPlatform Evidenceです。既存v1 Evidenceを上書きせず、Repositoryの隔離copyで正常・拒否・障害・回復・互換性の5 Scenarioを実行し、新しいEvidenceだけを`evidence/preview/runtime-binding/`へ保存します。

各Profileのbinding JSONは、Core正式main commit、Composition Digest、2 SubjectのVersion／Release／Certificate／Artifact Digest、実行開始／完了時刻、実Platform、再現build recipe、Runtime binary Digest、起動後executable attestation、5 Scenario Report、Cleanup Receiptを束縛します。localは起動済みprocessのexecutable pathをOSから解決し、containerは稼働中containerの`/fixture-subject`を`docker cp`でtask-owned一時領域へ採取します。validatorは両Subjectの観測Digestが再現buildのRuntime binary Digestと一致すること、各Scenarioのpass、Profile／Composition一致、Cleanup後のProcess／Container／Network／Image／Credential残存ゼロを再検証します。

この証跡は、固定Release artifactと同じsource path・build recipeから生成したbinaryをsealed Runnerが起動し、起動後process／containerから独立採取したDigestと一致した状態で実Scenarioを通過したことを示します。旧`process-executable-attestation-unavailable`は`migrations/runtime-binding-executable-attestation.json`で旧IDと新Proofを対応付け、attestation削除・Digest driftのnegative testを必須化して閉じます。一方、v1 Fixture CertificateはRuntime binary digestをatomicに含まないため、次のgapは残り、統合成功でも`definitive_eligible: false`を保持します。

- `subject-v2-certificate-atomic-binding-unavailable`

実行方法:

```bash
go run ./cmd/atlas-lab runtime-binding --profile local
go run ./cmd/atlas-lab runtime-binding --profile container
```

実行は一時copy内だけで行います。Runnerが生成したrun固有Process、Container、Network、ImageだけをCleanupし、Docker volume、既存Evidence、ユーザーデータ、外部Repositoryは変更または削除しません。保存Evidenceのgap削除、Definitive昇格、Scenario withdrawal、Release lock drift、Runtime Digest不正はnegative testで拒否します。

`composition-evidence-closure`は両Profileと複数Subject Compatibility Matrixをさらに外側の隔離copyで一括再実行し、全outputが検証できた場合だけ正本Preview Evidenceへ反映します。途中失敗の一時成果はpublishしません。
