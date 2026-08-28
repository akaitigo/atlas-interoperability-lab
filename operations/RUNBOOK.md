# Runbook

## 通常検証

`make check`を実行します。失敗時は最初に`evidence/runs/<profile>/<scenario>.json`の`actions`を確認し、次に`cleanup/<profile>.receipt.json`が`pass`か確認します。

## 中断時Cleanup

Runnerは同期的に全Resourceを回収します。強制終了でReceiptを生成できなかった場合は、`org.atlas-lab.run` Labelを持つContainer、Network、Imageを列挙し、当該Run IDだけを削除してください。広いTargetや他ProjectのResourceを削除してはいけません。

## Digest不一致

SubjectのSourceを取り込んで補修しません。新しい完成ReleaseとCertificateをSubject側で発行し、新しいComposition IDとしてVersionとDigestを更新します。

## 未完成Subject

Preflightの拒否が正しい結果です。Completion Gateを迂回したりFixture Certificateを書き換えたりせず、Subject Atlasの完成を待ちます。
