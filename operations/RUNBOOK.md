# Runbook

## 通常検証

`make check`を実行します。失敗時は`go run ./cmd/atlas-lab diagnose --profile <local|container>`で、保存済みEvidenceから失敗Class、Scenario、Action、次の操作を確認します。診断後に必要な場合だけ`evidence/runs/<profile>/<scenario>.json`の`actions`を開きます。

完成判定はDCO付きCommit後のClean Worktreeで`go run ./cmd/atlas-lab self-audit`を実行します。全Checkが`pass`でなければローカル完成ではありません。GitHub Remote未設定や公開権限は別のPublication阻害として扱い、ローカル成果を無効化しません。

## 中断時Cleanup

Runnerは同期的に全Resourceを回収します。強制終了でReceiptを生成できなかった場合は、`org.atlas-lab.run` Labelを持つContainer、Network、Imageを列挙し、当該Run IDだけを削除してください。Docker Volume、広いTarget、他ProjectのResource、既存Evidence、ユーザーデータは削除してはいけません。容量整理やPruneはこのRunbookの範囲外です。

## Digest不一致

SubjectのSourceを取り込んで補修しません。新しい完成ReleaseとCertificateをSubject側で発行し、新しいComposition IDとしてVersionとDigestを更新します。

## 未完成Subject

Preflightの拒否が正しい結果です。Completion Gateを迂回したりFixture Certificateを書き換えたりせず、Subject Atlasの完成を待ちます。
