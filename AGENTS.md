# Repository instructions

- `reference-atlas-core`の固定Commitと、このRepository内の`schemas/`およびComposition契約を正本にする。
- 完成済みSubject ReleaseだけをVersion、Release Digest、Completion Certificate Digestで参照する。Source Tree、submodule、Default Branchへ依存しない。
- Subject固有のAuthority、機能責任、実装知識をLabへ複製しない。
- 利用者向け文書とCLIメッセージは日本語、Schema KeyとIDは英語を保つ。
- Failure InjectionはFixtureまたは明示許可された隔離環境だけで行う。
- `status: complete`はlocalとcontainerのE2E、完全Cleanup、Router Eval、Publication Gate、Certificate、Core auditが全てpassした後だけ宣言する。
