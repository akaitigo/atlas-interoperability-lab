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

## Definitive Gate v2 Preview

`make definitive-preview`でMatrix、Router Eval、非破壊Migration、旧v1 Bundleを検証します。`core-v2-draft`が返る間はmainへMergeせず、CertificateをDefinitive completeとして公開しません。失効Certificateは再有効化せず、新Certificateから失効Digestを`supersedes_digest`で参照します。

## Non-Regression拒否

最初に`make non-regression`を実行します。拒否された変更をskip、optional化、Scope変更、mock化、Evidence削除で迂回してはいけません。正当な置換は旧ID Mapping、新Composition参照、local/container相当の2件以上の統合Proof、Migration Evidenceを同じ変更へ含めます。

Depth診断では`go run ./cmd/atlas-lab depth-parity`の`depth_reference_status`、`depth_parity_eligible`、`integration_proofs_valid`を同時に確認します。統合ProofがpassでもSubject別の`subject-depth-parity-incomplete`が1件以上あれば、Certificate更新やScenario再実行だけで昇格させません。
