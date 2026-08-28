# Machine-readable Contract

`schemas/*.schema.json`を形式契約、Goのstrict decoderとcross-file Preflightを実行契約とします。

- `composition.schema.json`: Core Lock、10検証軸、固定Subject Release、Profile、Scenario
- `subject-release.schema.json`: 完成状態、Artifact Digest、Launch Interface、Certificate
- `scenario.schema.json`: Setup、Execute、Verify、CleanupとHTTP／Compare Action
- `environment.schema.json`: 必須Runtime、Isolation、Cleanup、Cloud依存
- `oracle.schema.json`: Observableと判定Rule
- `cleanup.schema.json`: Profile別の残存ResourceとCredential
- `certificate.schema.json`: Core、Composition、Harness、Evidence、Skill、Publication GateのDigest束縛

PathはRepository Root相対です。Digestは`sha256:<64 lowercase hex>`、VersionはSemVerの`vMAJOR.MINOR.PATCH`です。Release Manifest自体とCompletion Certificateの両方をCompositionからDigest固定するため、内容の差替えはPreflightで拒否されます。
