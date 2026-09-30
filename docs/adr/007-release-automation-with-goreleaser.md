# ADR-007: GoReleaserおよびGitHub Actionsによるマルチプラットフォーム配布・リリース自動化

## ステータス (Status)
**承認済み (Accepted)**

## 文脈と課題 (Context)
要件定義書およびシステム設計書において、外部ランタイムを不要とする完全静的バイナリ（`CGO_ENABLED=0`）をマルチプラットフォーム（Linux, macOS, Windows / amd64, arm64）向けに提供することが定められている[cite: 1, 3, 4, 5]。  
これらを手動または単純なシェルスクリプトでビルド・配布する場合、以下の課題があった：
1. **配布作業の人的コスト**: 各OS・アーキテクチャ向けのクロスコンパイル、適切なアーカイブ形式（`.tar.gz` / `.zip`）への圧縮、README/LICENSEの同梱作業が煩雑[cite: 1, 3, 4]。
2. **完全性と安全性の保証**: 配布バイナリの改ざん検知や検証のためのSHA-256チェックサム生成が属人化しやすい。
3. **CI/CDパイプラインとの統合**: GitHub Releases へのドラフト作成、アセット添付、リリースノート生成の自動化標準が必要であった。

## 決定事項 (Decision)
Goプロジェクトにおける標準的リリース自動化ツールである **GoReleaser（v2系）** を採用し、**GitHub Actions** と連携したリリースパイプラインを構築する。

1. **配布パッケージ仕様**:
   * **対象プラットフォーム**:
     * Linux: `linux/amd64`, `linux/arm64`[cite: 1, 3, 4]
     * macOS: `darwin/amd64`, `darwin/arm64`（Apple Silicon）[cite: 1, 3, 4]
     * Windows: `windows/amd64`[cite: 1, 3, 4]
   * **アーカイブ形式**:
     * Linux / macOS: `.tar.gz`
     * Windows: `.zip`
   * **同梱ファイル**: バイナリ本体、`README.md`、`LICENSE`
   * **チェックサムファイル**: `checksums.txt`（SHA-256）を全アーカイブに対して自動生成
2. **ビルドオプション**:
   * `CGO_ENABLED=0`（完全静的リンク）[cite: 1, 3, 4]
   * `-ldflags="-s -w -X main.version={{.Version}}"` によるデバッグシンボル除去とバージョン情報の埋め込み[cite: 1, 4]
3. **ワークフロー自動化**:
   * リポジトリにセマンティックバージョニング形式のタグ（`v*.*.*`）がプッシュされた契機で GitHub Actions の Release ワークフローが起動し、GoReleaser が自動実行される構成とする。

## 結果と影響 (Consequences)
### メリット (Positive)
* **リリース作業の完全自動化**: Gitタグのプッシュのみで全OS向けのアセットビルド、チェックサム付与、GitHub Releasesへの公開が完結する。
* **完全性と透明性の担保**: 自動生成されたSHA-256ハッシュにより、エンドユーザーが安全にダウンロードファイルの正当性を検証できる。
* **軽量・安全なバイナリ**: シンボル削除（`-s -w`）と CGO 無効化により、外部共有ライブラリに依存しない可搬性の高いバイナリが生成される[cite: 1, 3, 4]。

### デメリット・トレードオフ (Negative / Risks)
* **設定ファイルの保守**: `.goreleaser.yaml` の記法・バージョン変更に対する継続的な保守が必要。
* **GitHub Actions 権限管理**: ワークフローに対して GitHub Releases への書き込み権限（`contents: write`）を適切に構成する必要がある。
