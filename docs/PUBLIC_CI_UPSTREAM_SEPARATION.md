# Public CI Upstream Separation

`frontend-behavior-atlas`は公開CIから取得しません。visibility変更、secret投入、private checkout、FE payloadの別Repositoryへの複製は行いません。

ローカル`make verify`は固定commit `deadad18b6588d2c907170a451c3b5cea5ea4192`の実Git objectを読み、FE Depth Reference、Scenario Index、Integration Results、850個の専用Proofをbyte／digest検証します。その結果から生成したlocal reportは、Interopで追跡するlock、derived preview、workflow、`repo.yaml`のDigestを列挙します。public attestationはreport digestと`repo.yaml` digestを固定し、SSH署名を付与します。

公開CIの`public-ci-gate`は署名、固定commit／source path／digest、report、lock、preview、workflow、`repo.yaml`に加え、validator本体、CLI routing、allowed-signers trust root、DCO range verifier、Gitless隔離copyのBaseline reference root検証、Actual Subject拒否lock／schema／matrixをfail-closed検証します。Actual Subjectのpublic検証は、署名確認後にlocal生成済みreport／matrixと決定論的期待値をbyte比較するread-only処理です。後続の`composition-evidence-audit`が同じtracked outputのGraph digestを検証し、public固有payloadで同じEvidence IDを置換しません。署名・Digest改ざん、trust root追加、routing迂回、DCO verifier緩和、reference root検証の改変、各既存Gate command削除、private clone再導入、2 Gateの同一copy再統合、Subject拒否境界改変を28 negative fixtureで拒否します。

Local E2E、Container E2E、Runtime Binding、Failure Diagnosticsには、署名HEADから作成した別々のtask-owned copyを割り当てます。先行Gateの生成outputで後続GateのNon-regression Baselineを汚染せず、汚染されたcopyは`artifact-regressed`として拒否します。

この分離の境界は次のとおりです。

- `upstream_live_verification: unavailable`
- `local_gate_required: true`
- `completion_effect: none`
- `distribution_gap_effect: none`
- `authorization_effect: none`

署名attestationは実upstream照合の代替ではなく、FEをlive取得済み、current、definitiveとは扱いません。ローカル実Git object照合がない実行からCompletion状態を昇格させません。
