# Architecture

## 所有する責任

Labが所有するのはComposition、Scenario順序、Environment Adapter、外部Observableに対するOracle、Evidence束縛、Cleanup、組合せCertificateです。SubjectのAuthority解釈、Capability実装、内部状態、単体Completionは所有しません。

RunnerはRelease Manifestの`launch`契約だけを読み、`go-source` AdapterでFixture Artifactを起動します。Scenario EngineはHTTP Action、JSON Assertion、Value Compareだけを解釈し、Subject名や業務知識をHard-codeしません。

`graphs/fixture-stage2.claim-evidence.json`は各検証軸についてClaim、Composition内の全固定Subject、Scenario、local/container Evidenceを直接接続します。これによりSubject単体のClaimを複製せず、複数Subject間で成立する結論の根拠を機械検証できます。

## Profile

- `local`: loopback上の子Process。終了後にBinary、Log、Credentialを削除します。
- `container`: Run専用Docker Networkと`scratch` Image。Sourceだけをloopbackへpublishし、終了後にContainer、Network、Imageを削除します。
- `cloud-live`: 任意かつ未実装。主要Claimに使わず、明示Opt-in、専用Account、Budget、Provider Inventoryが揃うまで拒否します。

## 決定論

Logical Trace ID、Message ID、Action、期待値はScenarioで固定します。実行時CredentialとResource名は毎回変えますがEvidenceへ含めません。Scenario ReportのDigestは同じ入力、Release、Profile、Harnessで一致します。

## Security Boundary

Fixture CredentialはProcess／Container環境だけに渡します。Action ResultはStatus、Assertion数、Verdictだけを記録しHeaderやBodyをEvidenceへ写しません。Failure Injection APIは実行時Admin Credentialを要求し、Dockerでは内部NetworkのSinkへSource経由でのみ到達します。

## 診断と自己監査

診断器は保存済みAction ResultとCleanup Receiptだけを読み、Status、Data、到達性、境界状態、Cleanupの失敗Classと次の操作を返します。Request Body、Header、Credentialは出力しません。自己監査はRepository契約、Publication Gate、Certificate、Core Audit、DCO、Clean Worktreeを一括判定します。
