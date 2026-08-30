# Runtime Binding Evidence Preview

このPreviewは、固定Fixture Subject Releaseをlocal processとDocker containerで実行したPlatform Evidenceです。既存v1 Evidenceを上書きせず、Repositoryの隔離copyで正常・拒否・障害・回復・互換性の5 Scenarioを実行し、新しいEvidenceだけを`evidence/preview/runtime-binding/`へ保存します。

各Profileのbinding JSONは、Core正式main commit、Composition Digest、2 SubjectのVersion／Release／Certificate／Artifact Digest、実行開始／完了時刻、実Platform、再現build recipe、Runtime binary Digest、5 Scenario Report、Cleanup Receiptを束縛します。validatorは参照ArtifactのDigestだけでなく、各Scenarioのpass、Profile／Composition一致、Cleanup後のProcess／Container／Network／Image／Credential残存ゼロを再検証します。

この証跡が示すのは、固定Release artifactと同じsource path・build recipeから生成したbinaryを用いてsealed Runnerが実Scenarioを通過したことです。起動後process／containerから独立に採取したexecutable attestationではありません。またv1 Fixture CertificateはRuntime binary digestをatomicに含みません。このため次のgapを削除できず、統合成功でも`definitive_eligible: false`を保持します。

- `process-executable-attestation-unavailable`
- `subject-v2-certificate-atomic-binding-unavailable`

実行方法:

```bash
go run ./cmd/atlas-lab runtime-binding --profile local
go run ./cmd/atlas-lab runtime-binding --profile container
```

実行は一時copy内だけで行います。Runnerが生成したrun固有Process、Container、Network、ImageだけをCleanupし、Docker volume、既存Evidence、ユーザーデータ、外部Repositoryは変更または削除しません。保存Evidenceのgap削除、Definitive昇格、Scenario withdrawal、Release lock drift、Runtime Digest不正はnegative testで拒否します。

`composition-evidence-closure`は両Profileと複数Subject Compatibility Matrixをさらに外側の隔離copyで一括再実行し、全outputが検証できた場合だけ正本Preview Evidenceへ反映します。途中失敗の一時成果はpublishしません。
