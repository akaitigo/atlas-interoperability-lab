# 複数Subject Composition互換性 Preview

`tests/fixtures/composition-compatibility.matrix.json`は、Core正式main commit `072d7ca77981f51754e824d70c6d4ecd55ea67e5`から生成した完成済みFixtureを`source`と`sink`の独立probe instanceとして検証します。各instanceに同じCore CLI GateとDefinitive Certificate検証を実行し、一方だけがstale、digest-only closure、再実行対象漏れ、output退避、Proof／Closure Plan構造縮小になった場合はComposition全体を拒否します。sourceがstale、sinkがdigest-only closureの同時失敗caseでは、両Subjectを個別に`failed_subjects`へ残し、片方だけの失敗へ集約する実装を拒否します。

Cross-Subject Claim/Evidence Graphは別の判定です。Subject probeのCore Gateがすべてpassでも、Claim linkから1つのSubjectを外したnegative fixtureは拒否されます。probe identityはComposition SubjectのCertificateへAtomic Bindingされていないため、`binding_state: explicit-gap`と`subject-probe-atomic-binding-gap`を記録します。全Core GateとClaim linkがpassしても互換性は`incomplete`、`definitive_eligible: false`です。各構成Subjectの18軸Depth Parity、Surface/Pattern row、Core v2 draftのGapも継承し、統合成功でSubject不足を相殺しません。

`compatibility/subject-binding-candidates.lock.json`はActual Subject候補を固定Git commitとCertificate digestで記録します。local Gateは各Git objectをbyte/digest照合し、公開CIはowner署名で固定された拒否metadataだけを検証します。RabbitMQ、PostgreSQL、Zero Trustの現候補はv1-onlyまたは署名Release未主張であり、Core v2 Certificate、固定Release Digest、Depth／Surface完了、Atomic Authority Bindingがないため個別に拒否します。`tests/fixtures/subject-binding-admission.matrix.json`はv1昇格、署名・Release・Depth・Surface・Authorityの虚偽昇格、候補分母縮小、fixture置換、複数不足の集約を拒否します。

`go run ./cmd/atlas-lab composition-compatibility-matrix`は27 consumer/case結果を`evidence/preview/composition-compatibility.matrix.json`へ保存します。`go run ./cmd/atlas-lab subject-binding-admission`はActual Subjectの個別拒否と14 negative caseを保存します。`go run ./cmd/atlas-lab preview-publication-gate`は既存v1 Publication baseline、Depth Gap継承、複数Subject Matrix、Actual Subject admission、Router v2 Eval、中立表現を横断します。これはdraft Preview Gateであり、v1 Completion Certificateの内容や状態を変更しません。

Core Fixture、mutation対象、Claim Graph negative fixtureは実行ごとに一時ディレクトリへ作成して完全に除去します。外部Repository、既存Evidence、Docker volume、ユーザーデータは変更または削除しません。
