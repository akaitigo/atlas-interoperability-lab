# Machine-readable Contract

`schemas/*.schema.json`を形式契約、Goのstrict decoderとcross-file Preflightを実行契約とします。

- `composition.schema.json`: Core Lock、10検証軸、固定Subject Release、Profile、Scenario
- `subject-release.schema.json`: 完成状態、Artifact Digest、Launch Interface、Certificate
- `scenario.schema.json`: Setup、Execute、Verify、CleanupとHTTP／Compare Action
- `environment.schema.json`: 必須Runtime、Isolation、Cleanup、Cloud依存
- `oracle.schema.json`: Observableと判定Rule
- `cleanup.schema.json`: Profile別の残存ResourceとCredential
- `claim-evidence-graph.schema.json`: 各ClaimとComposition内の全Subject、Scenario、Required Evidenceの横断接続
- `certificate.schema.json`: Core、Composition、Harness、Evidence、Skill、Publication GateのDigest束縛

PathはRepository Root相対です。Digestは`sha256:<64 lowercase hex>`、VersionはSemVerの`vMAJOR.MINOR.PATCH`です。Release Manifest自体とCompletion Certificateの両方をCompositionからDigest固定するため、内容の差替えはPreflightで拒否されます。

`diagnose`と`self-audit`は`schema_version`、`verdict`、機械識別用の英語Code／Check名、日本語Detailを持つJSONを標準出力へ返します。診断出力に秘密値やRequest Payloadを含めません。
