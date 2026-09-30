# ADR-004: フィード解析ライブラリ mmcdole/gofeed の採用とサニタイズ方針

## ステータス (Status)
**承認済み (Accepted)**

## 文脈と課題 (Context)
ADR-001 において、RSS 0.9x / 1.0 / 2.0 および Atom などの各種 Web フィードを単一バイナリで取得・パースし、統一データモデルに正規化して JSON / CSV 出力することが決定している。

また、システム設計書において以下の要件が明記されている[cite: 1]：
1. **パース仕様**: 各種フィード規格のタグ差異や日付表記の揺れを吸収し、UTC 基準の ISO 8601（RFC 3339）へ変換すること[cite: 1]。
2. **サニタイズ仕様**: `summary` や `content` に含まれる HTML 要素（`<p>`, `<a>`, `<script>` 等）を字句解析レベルで完全に除去してプレーンテキスト化し、連続する不要な空白文字や空行をトリミングすること[cite: 1]。
3. **入出力分離**: フェッチャーが取得した生 XML ストリームを受け取り、内部モデル `[]model.FeedItem` へ変換すること[cite: 1]。

これらを自前実装することなく、堅牢かつ安全に処理するためのライブラリ選定とサニタイズ実装方針の決定が必要となる。

---

## 決定事項 (Decision)

### 1. ライブラリ選定とバージョン固定
* **採用ライブラリ**: **`github.com/mmcdole/gofeed`**
* **バージョン**: **ADR 作成時点における最新安定版（v1.3.0 系）**
  * `go.mod` において最新タグを明示的に指定して管理する。

### 2. フィード解析の責務と連携
* 設計書のパイプライン設計に基づき、HTTP 通信は `internal/fetcher`（`net/http` + `context.Context`）が担当し、取得した `io.Reader`（Raw XML Stream）を `internal/parser` に渡す構成とする[cite: 1]。
* `gofeed.NewParser().Parse(xmlReader)` を利用して解析を行い、通信層とパース層の責務を完全に分離する[cite: 1]。
* 公開日時は `item.PublishedParsed`（未設定時は `item.UpdatedParsed`）から取得し、UTC 基準の `time.Time` として正規化する[cite: 1]。

### 3. サニタイズ処理仕様（設計書準拠）
`model.FeedItem.Summary` に格納するテキストは、以下のステップでサニタイズを実施する[cite: 1]。

1. **優先ソースの決定**:
   * `item.Description` を第 1 候補、存在しない場合は `item.Content` をフォールバックとして採用。
2. **HTML 要素の完全除去**:
   * 正規表現による置換ではなく、HTML 字句解析（`golang.org/x/net/html` のトークナイザー等）を用いて `<script>`, `<style>`, タグ属性を含めた全要素を安全にストリップし、テキストノードのみを抽出する[cite: 1]。
3. **空白・改行の正規化**:
   * 連続する半角・全角空白およびタブを単一の半角スペースに集約する[cite: 1]。
   * 連続する改行を整理し、前後の不要な空白をトリミングする[cite: 1]。

---

## 結果と影響 (Consequences)

### メリット (Positive)
* **規格対応の堅牢性**: RSS 0.9x / 1.0 / 2.0、Atom の構文差異や日付フォーマットの揺れ（RFC 822 / RFC 3339 等）を `gofeed` が自動吸収するため、パースエラーの発生率を最小化できる。
* **クリーンな出力の保証**: 字句解析によるサニタイズ処理を挟むことで、CSV や JSON に HTML タグや不正なスクリプト片が混入することを防止できる[cite: 1]。
* **ストリーム処理親和性**: `io.Reader` 経由での解析が可能なため、設計書で求められているメモリフットプリント（30MB 以下）の抑制に寄与する[cite: 1]。

### デメリット・トレードオフ (Negative / Risks)
* **依存関係の増加**: `gofeed` は内部で XML 解析等のサブパッケージを持つため、標準パッケージのみの構成と比較してビルド生成物のサイズが数 MB 増加する。
* **リッチテキスト情報の喪失**: HTML タグを完全にプレーンテキスト化するため、記事本文中のリンク先 URL や画像 URL は `summary` 内からは参照できなくなる（記事自体の URL は `model.FeedItem.URL` として保持される）[cite: 1]。

---

## 実装イメージ（サニタイズおよびパーサー連携）

### サニタイズ処理 (`internal/parser/sanitize.go`)
```go
package parser

import (
	"bytes"
	"io"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

var (
	spaceRegex = regexp.MustCompile(`[ \t\v\f]+`)
	lineRegex  = regexp.MustCompile(`\n{3,}`)
)

// StripHTML はHTML文字列からタグを除去し、正規化されたプレーンテキストを返す
func StripHTML(src string) string {
	if src == "" {
		return ""
	}

	tokenizer := html.NewTokenizer(strings.NewReader(src))
	var buf bytes.Buffer
	skipContent := false

	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			if tokenizer.Err() == io.EOF {
				break
			}
			return strings.TrimSpace(src) // パース失敗時は元の文字列をフォールバック
		}

		switch tt {
		case html.StartTagToken:
			tn, _ := tokenizer.TagName()
			tagName := string(tn)
			// スクリプトやスタイルシートの内容はテキストも含め除外
			if tagName == "script" || tagName == "style" {
				skipContent = true
			}
			// ブロック要素の区切りとして改行を挿入
			if isBlockElement(tagName) {
				buf.WriteString("\n")
			}
		case html.EndTagToken:
			tn, _ := tokenizer.TagName()
			tagName := string(tn)
			if tagName == "script" || tagName == "style" {
				skipContent = false
			}
			if isBlockElement(tagName) {
				buf.WriteString("\n")
			}
		case html.TextToken:
			if !skipContent {
				buf.Write(tokenizer.Text())
			}
		}
	}

	// 空白・改行の正規化
	text := buf.String()
	text = spaceRegex.ReplaceAllString(text, " ")
	text = lineRegex.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

func isBlockElement(tag string) bool {
	switch tag {
	case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "li", "br", "article", "section":
		return true
	default:
		return false
	}
}
```

### フィード解析処理 (internal/parser/parser.go)
```go
package parser

import (
	"io"
	"time"

	"feed-go/internal/model"
	"[github.com/mmcdole/gofeed](https://github.com/mmcdole/gofeed)"
)

// ParseAndNormalize はRaw XMLをパースして統一FeedItemスライスへ変換する
func ParseAndNormalize(r io.Reader) ([]model.FeedItem, error) {
	fp := gofeed.NewParser()
	feed, err := fp.Parse(r)
	if err != nil {
		return nil, err
	}

	items := make([]model.FeedItem, 0, len(feed.Items))
	for _, item := range feed.Items {
		var published time.Time
		if item.PublishedParsed != nil {
			published = item.PublishedParsed.UTC()
		} else if item.UpdatedParsed != nil {
			published = item.UpdatedParsed.UTC()
		}

		rawSummary := item.Description
		if rawSummary == "" {
			rawSummary = item.Content
		}

		authorName := ""
		if item.Author != nil {
			authorName = item.Author.Name
		}

		items = append(items, model.FeedItem{
			Title:       strings.TrimSpace(item.Title),
			URL:         item.Link,
			PublishedAt: published,
			Summary:     StripHTML(rawSummary),
			Author:      strings.TrimSpace(authorName),
			FeedTitle:   strings.TrimSpace(feed.Title),
		})
	}

	return items, nil
}
```
