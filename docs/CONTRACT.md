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

v2準備契約は`composition-v2-preview.schema.json`と`subject-certificate-v2-preview.schema.json`へ分離します。`status: draft`と`2.0.0-draft.1`を必須にし、Core v2確定前の`definitive-complete`生成を構造的に禁止します。v1正本SchemaとCertificateは変更しません。

`non-regression-baseline.schema.json`は歴史的Baseline Commit、対象Path、必須Profileを固定します。`non-regression-migration.schema.json`は置換時の旧ID Mapping、新Path、2件以上の統合Proof、1件以上のMigration Evidenceを要求します。

Depth Parityは`fe-depth-reference-lock.schema.json`、`fe-scenario-contract-lock.schema.json`、`subject-depth-parity.schema.json`、`integration-depth-proofs.schema.json`へ分離します。外部Reference、各Subjectの18軸状態、Interop固有のlocal／container Proof、FE統合10 Scenario、Surface/Pattern 850 rowを混同しません。各rowのDigest、固有Evidence、Runtime Identity、Atomic Authority Bindingまたは明示gapを検証し、421 gap／completion eligible 0の状態では統合成功からSubjectまたはInterop完成を推論しません。

Evidence Dependency consumer互換性は`evidence-dependency-core-lock.schema.json`と`evidence-dependency-consumer-matrix.schema.json`へ分離します。Core正式main commit、CLI Gate、Definitive Certificate検証を固定し、consumer名によらず同じstale／closure結果を要求します。Subject固有の件数とprofileはMatrix共通閾値に含めません。

Composition互換性Previewは`composition-compatibility-matrix.schema.json`を使用します。各Subject probe instanceのEvidence Dependency Gate／Certificate、Composition SubjectとのBinding状態、Cross-Subject Claim/Evidence link、Composition互換状態、継承Gapを別fieldで記録し、集約結果だけによるDefinitive昇格を禁止します。

Runtime Binding Previewは`runtime-binding-evidence.schema.json`を使用します。local／containerごとに実Platform、Runtime binary digest、固定Subject Release、5 Scenario Report、Cleanup ReceiptをDigest固定します。build recipeによる観測と実process／container executableのattestationは区別し、Subject v2 Certificateとのatomic bindingを含む未閉鎖gapを必須fieldとして保持します。

Composition-level closureは`composition-evidence-dependency.schema.json`、`composition-evidence-dependency-matrix.schema.json`、`runtime-binding-proof-index.schema.json`、`composition-evidence-closure-plan.schema.json`、`subject-binding-admission.schema.json`を使用します。Core portable predicate、全input member、全required output、local／container／derived run、Runtime Identity、Proof／Closure構造baseline、Actual Subject候補の個別拒否を分離します。Lab固有の2 ProfileやScenario数をCore共通閾値へ持ち上げず、このComposition自身のclosureとして検証します。
