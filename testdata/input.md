# Proposal: Pretty IslandsをPreactからHonoへ移行する

**提案日**　2026年10月28日  
**対象**　Pretty Islandsフレームワーク  
**基準文書**　`pretty-islands-framework.md`（2026年10月28日改訂版、3.4節「PreactのVDOMが担う役割」を含む最新版）  
**状態**　設計変更提案。Hono版の実装・性能・Cloudflare本番環境での動作は未検証。

## 1. 提案の要旨

Pretty IslandsのHTMLレンダリングとHTTP処理を、**Preact JSX／Suspense／Workers Native `fetch()` による手組みRouter・Middleware**から、**Honoの`html`ヘルパー／Hono Router・Middleware／SSEによる非同期HTML断片配信**へ移行する。

移行後の基本構成は、**Vite + Hono（`html`、Router、Middleware、Streaming Helper）+ Cloudflare Workers/workerd + DSD + Custom Elements + pnpm 12管理のNode.js**とする。Hono JSX、`hono/jsx/dom`、Preact、ページ全体のHydration、独自VDOMは採用しない。

Server Islandsは、HTMLドキュメントの初期応答で領域とfallbackを描画し、後続の**SSE（Server-Sent Events）**で完成した断片を順不同に届け、ブラウザ上の小さな共通クライアントで所定の領域へ挿入する方式へ変更する。HTML生成は常にサーバー側の`hono/html`が担い、ブラウザはHTMLの**生成**ではなく**挿入とCustom Elementの起動**だけを担当する。

この提案はフレームワークの共通能力を定めるものであり、個別サイトがServer Islandsを使うことを義務づけない。`static`、`cache`、`dynamic`の3モード、CloudflareのWorkers Cacheによる共有キャッシュ境界、AI実装・CI検証・人間承認という原則は維持する。

## 2. なぜ変更するのか

### 2.1. PreactのVDOMをブラウザで利用していない

現行のPretty IslandsではPreactはSSR専用であり、ブラウザではHydrationもVDOM差分更新も行わない。JSXからVNodeを生成し、SSRレンダラーが走査してHTMLを出力する方式は正しく機能するが、この用途で必須の中間表現ではない。`hono/html`はTagged Template Literalを使ってHTML断片を構成するため、PreactのVNodeツリーやSSRレンダラーを必要としない。

ここでの論点は「Honoのほうが常に高速」という断定ではなく、**必要な責任と依存関係を減らせる**という設計上の見通しである。CPU使用量、割り当てメモリ、TTFB、HTML生成速度の差は、同じページ・同じデータをworkerdで比較して判断する。

### 2.2. JSXではなくHTMLそのものを中心に書ける

Honoの`html`は文字列補間を自動エスケープし、安全なHTML断片を関数で組み合わせられる。HTML、`<template shadowrootmode="open">`、Custom Element、フォーム、CSSはブラウザが解釈する表記に近い形で記述できる。Preact用のDSD属性型拡張も不要になる。

### 2.3. RouterとMiddlewareを標準機構へ任せられる

現行要綱ではHTTP入口をNative `fetch()`と通常の関数で実装している。その方法は小規模で明快だが、ルート数、認証方式、HTTPメソッド、SSEエンドポイント、エラーハンドリングが増えると、実装の重複と順序制御が課題となる。HonoのRouterとMiddlewareはこれらを小さなAPIで統合できる。

これはCloudflare Workersの実行環境をHonoに「置換」することではない。Honoは**Workersの`fetch()`上で動くHTTPアプリケーション層**であり、workerd、Bindings、Static Assets、Workers Cacheは引き続きCloudflareの機能として利用する。

## 3. 移行前後の責任分担

| 領域 | 現行 | 提案後 |
|---|---|---|
| HTML生成 | Preact JSX + `preact-render-to-string` | **`hono/html`**。Tagged Template Literalと関数による部品化 |
| JSX | Preact JSX | **HTML生成には使用しない**。TSXを使う必要をなくす |
| Server Islands | Preact Suspense Streaming | **Hono html + SSE + DOM断片挿入** |
| 非同期順不同表示 | Suspenseの差し替えスクリプト | SSEイベントと独立した小さなブラウザ用ランタイム |
| Router | Native `fetch()`内の通常の関数 | **Hono Router** |
| Middleware | 通常のTS関数を手動合成 | **Hono Middleware**。適用順とスコープを宣言 |
| エラー処理 | Native `fetch()`の例外処理 | Hono `app.onError()`。ストリーム開始後はSSEエラーイベント |
| `static` | Preactによる事前生成HTML | **同じ`hono/html`部品をNode.jsビルド時に文字列化** |
| `cache` | 完成HTMLをWorkers Cacheで共有 | **同じ`hono/html`部品で完成HTMLを返す**。SSEしない |
| `dynamic` | Preact SSR、必要ならSuspense | **Honoで初期HTML**。Server Islands利用時のみSSE |
| Client Islands | DSD + Custom Elements | **維持**。SSR用コンポーネントだけ`hono/html`へ書き換え |
| Vite / Cloudflare | Vite、`@cloudflare/vite-plugin`、workerd | **維持** |
| テストと環境 | Vitest、Cloudflare Vitest integration、pnpm 12 | **維持**。Hono固有の境界テストを追加 |

### 構成図

```text
AIのクラウド環境 / GitHub Actions
  pnpm 12 → プロジェクト固定のNode.js → Vite / Vitest / 型・Lint・Format
                              │
                    Cloudflare Vite Plugin
                              │
                           workerd
                              │
              Hono Router + Hono Middleware
               ├─ static  → ビルド時に生成したHTML → Static Assets
               ├─ cache   → 完成HTML → Cached EntryPoint → Workers Cache
               └─ dynamic → Hono htmlによる初期HTML（Cache OFF）
                                └─ 必要なページだけ /fragments SSE（Cache OFF）
                                          │
ブラウザ                                  │
  ├─ 初期HTMLをそのまま表示               │
  ├─ Server Islands用の小さな外部JS ←────┘
  │      └─ 完成したHTML断片を指定位置に挿入
  └─ DSD + Custom Elementsが挿入済みDOMに動作を追加
```

## 4. Hono JSXを採用しない理由

Honoの`hono/jsx`もSSRに使え、`hono/jsx/streaming`には実験的なSuspenseがある。ただしサーバー側のHono JSXは`JSXNode`という中間ノードを生成し、その構造をHTMLへ直列化する。これはPreactより簡潔な実装であり得るが、今回の「SSR用VNode相当の中間表現を避ける」という目的を直接は満たさない。

一方、`hono/html`はHTML断片を直接構成し、DSDの記述や関数合成とも相性がよい。したがって**HonoのHTTP機能は採用するが、Hono JSXとJSX Suspenseは採用しない**。`hono/jsx/dom`もClient Islandsの設計と重複するため使わない。

代償も明記する。Tagged Template LiteralはJSXと異なり、HTML属性・構造の型チェックや補完をTSXコンポーネントの型検査に全面的には任せられない。テンプレートのHTML検証、URLポリシー、DSDの構造テスト、型付きPropsは別の検査で補う。コンポーネントの条件分岐や繰り返しもTS関数として明示的に組み立てる。

```ts
import { html } from 'hono/html'

type SearchProps = { placeholder: string }

export const Search = ({ placeholder }: SearchProps) => html`
  <site-search>
    <template shadowrootmode="open">
      <form action="/search" method="get">
        <input type="search" name="q" placeholder="${placeholder}">
        <button type="submit">検索</button>
      </form>
    </template>
  </site-search>
`

export const Page = () => html`
  <!doctype html>
  <html lang="ja">
    <body><main>${Search({ placeholder: 'キーワード' })}</main></body>
  </html>
`
```

この例の通常の補間文字列はHonoによってエスケープされる。`raw()`や`<script>`等の直接記述については第8章の制約を適用する。

## 5. Server IslandsをHono html + SSEへ置き換える

### 5.1. 非同期処理と表示順序の関係

ページの上部にA（800ms）、その下にB（100ms）がある場合、両方のデータ取得を同時に開始しても、**完成HTMLを文書上のA→Bの順にストリームへ追加するだけ**なら、Bの表示はAの完了を待つ。HTMLは上から順に解析され、後から送ったバイト列で既に送信したDOM位置を直接書き換えられないからである。

Preact Suspenseは初期HTMLにA・Bのfallbackを配置し、完成した断片と差し替えスクリプトを後から送り、ブラウザで本来の位置に戻す。**順不同表示の本質は、後着の断片を元のDOM位置へ差し替えるクライアント処理**である。

### 5.2. SSEは「不可欠」ではなく、今回選ぶプロトコル

**順不同表示にSSEが論理的に必須というわけではない。** 単一HTMLレスポンスに差し替えスクリプトを混ぜる方法、`fetch()`のReadableStreamから独自形式のフレームを読む方法、WebSocket等でも実現できる。SSEを採用する理由は、HTML本文と非同期更新を分離し、HTTPの一方向イベント、名前付きイベント、ブラウザの`EventSource`、Honoの`streamSSE()`を使ってプロトコルを単純にできるためである。

したがって、正確な表現は「**順不同表示にはクライアント側の挿入処理が必要であり、この提案では完成したHTML断片を配信する手段としてSSEを選ぶ**」である。

また、SSEはHTMLとして自動表示されない。ブラウザはJSON等としてイベントデータを受け取り、外部JSがHTML断片を解析・挿入する。これはクライアントでUIをレンダリングするCSRではなく、サーバー生成HTMLを後から挿入する方式である。

### 5.3. 配信手順

1. `GET /page`が必要な基本内容、A・Bのプレースホルダー、外部SSEクライアントJSを持つ初期HTMLを返す。基本内容は初期SSRで確定させる。
2. ブラウザが初期HTMLを解析し、外部JSが**同一オリジンのSSEエンドポイント**へ接続する。
3. SSEエンドポイントでA・Bの処理を**並列に開始**する。ページの初期リクエストで開始したPromiseが自動的に共有されるわけではない点に注意する。
4. Bが100msで完成すれば、`fragment`イベントでBのHTMLと固定IDを送る。クライアントはBのプレースホルダーへ挿入する。
5. Aが800msで完成すれば、Aを挿入する。全件処理後に`done`を送信し、EventSourceを閉じる。
6. 部品単位の失敗は`fragment-error`で通知し、失敗箇所だけ既定の表示へ切り替える。

```text
初期HTML :  [Header] [A: 読み込み中] [B: 読み込み中] [Footer]
100ms    :  SSE fragment(B) → Bだけ完成表示
800ms    :  SSE fragment(A) → Aも完成表示
完了     :  SSE done → 接続を閉じる
```

イベント形式の概念例は次のとおり。

```text
event: fragment
data: {"id":"popular","html":"<section>人気記事</section>"}

event: fragment
data: {"id":"recommended","html":"<section>おすすめ記事</section>"}

event: done
data: {}

```

Honoの`streamSSE()`を利用して送信する。複数のPromiseが同時に完了しても、SSEへの書き込みは**完了イベントをキューで直列化**してフレームの混線を防ぐ。クライアントは既知のIDのみ受け付け、同じIDへの重複適用、古いページからのイベント、接続切断後の書き込みを防ぐ。

### 5.4. 通信・運用上の代償

- HTMLとSSEは**別々のHTTPリクエスト**になる。EventSource開始には初期HTMLと外部JSの読み込みが必要で、Suspenseの1レスポンス内Streamingより更新開始が遅れる可能性がある。
- 非同期部品のデータ取得はSSEリクエストの寿命で管理する。初期HTML側で同じ取得を行うと二重計算になり得る。
- EventSourceは切断時に再接続を試みる。有限のSSR更新では`done`で明示的に閉じ、再接続による再実行や二重表示を設計・検査する。ページ遷移やAbort時はキャンセル可能なI/Oを中断する。
- 通常のEventSourceは任意のAuthorizationヘッダーを渡すAPIを持たない。認証は同一オリジンのCookieとサーバー側の再認証を基本とし、秘密トークンをURLクエリに置かない。CORS、SameSite、権限検査、キャッシュ禁止をSSEにも適用する。
- 重要コンテンツ、SEO対象の本文、JS無効でも必須の内容は初期SSRで提供する。SSEが使えない時の代替として、同じサーバーレンダラーによる完成HTMLの非Streaming経路も検討する。
- SSEイベントとDOM差し替えの自作部分は**小規模であっても専用の共通コード**になる。「独自Islands Runtimeを一切持たない」から「固定IDによる限定的な断片更新だけを持つ」へ規約を修正する。

## 6. 三つの配信モードは維持する

### `static`

Node.jsで`hono/html`の関数を実行して完成HTMLを生成し、Viteの資産manifestに基づいてハッシュ付きJS・CSSを組み込み、Cloudflare Static Assetsへ配置する。SSEは原則不要。コンテンツ変更時は再ビルドする。

### `cache`

Hono RouterがCache OFFのGatewayでURL正規化とアクセス制御を行い、共有可能な公開ページだけをCache ONの`CachedRenderer` EntryPointへ渡す。`CachedRenderer`は`hono/html`で**非同期処理をすべて完了させたHTML**を返す。SSEとStreamingは使用しない。TTLはCloudflareのキャッシュ応答ヘッダーで制御する。

**Honoの`hono/cache`ミドルウェアやCloudflare Cache APIを、新しいWorkers Cache設定と混同しない。** 本提案の標準は引き続きEntryPoint単位の**Workers Cache**であり、Cache HIT時は対象EntryPointのHono handler自体が動かない。GatewayのMiddlewareはHIT前に動くが、Cache対象EntryPoint内のMiddlewareはHIT時には動かない。

### `dynamic`

Hono Routerが毎リクエスト応答する。`Cache-Control: private, no-store`を付け、Cache OFF EntryPointで実行する。Server Islandsの有無はページごとに選び、利用する時のみSSEエンドポイントと外部JSを付ける。SSEも`no-store`とし、ユーザーごとの認証・権限を改めて検査する。

ページの`mode`、EntryPointのCache ON/OFF、実際の課金は別レイヤーとして扱う、という現行要綱の原則を維持する。Static AssetsとGatewayの照合、ルート衝突のビルド検査も引き継ぐ。

## 7. Hono Router・Middlewareへ移行する意義

Native `fetch()`は残るが、その中のルーティングと共通処理をHonoへ委譲する。主な利点は、メソッド別・パス別の宣言、パラメータの取得、`app.route()`による機能単位の分割、`app.use()`のスコープ指定、リクエスト前後の処理、エラー応答、Bindingsの型付け、テスト用の`app.request()`である。

```ts
import { Hono } from 'hono'
import { html } from 'hono/html'
import { streamSSE } from 'hono/streaming'

type Bindings = { /* D1・KV・R2等、プロジェクトで定義 */ }
const app = new Hono<{ Bindings: Bindings }>()

app.use('*', async (c, next) => {
  // ここにはリクエストIDや共通ヘッダーなどを置く
  await next()
})

app.get('/articles', (c) =>
  c.html(html`<h1>記事一覧</h1>`, 200, {
    'Cache-Control': 'private, no-store',
  }),
)

app.get('/articles/fragments', (c) => {
  c.header('Cache-Control', 'private, no-store')
  return streamSSE(c, async (stream) => {
    // ここで独立した断片の生成を開始し、完了順にwriteSSEする
    await stream.writeSSE({ event: 'done', data: '{}' })
  })
})

app.onError((error, c) => {
  console.error(error)
  return c.text('Internal Server Error', 500)
})

export default app
```

上は**HTTP入口とSSEのAPI形状を示す概念例**であり、正規化、認証、エラー通知、断片生成は未実装。現行の型付きルートmanifestを廃止する必要はなく、`mode`・`ttl`・`auth`等の宣言からHonoのルート登録と検査を生成する設計も可能である。ただし単なるHonoルートとmanifestの**二重管理**は避け、どちらを正とするかを決める。

Cache OFFのGatewayとCache ONのNamed EntryPointはCloudflare固有の境界として保持する。`export default app`は通常のHTTP入口には使えるが、Named EntryPointと`ctx.exports`を要する構成では**Honoアプリを各EntryPointの`fetch()`から呼ぶ薄いAdapter**を作る。これはRouterを再実装することではない。`ctx.exports`利用時の互換性フラグ、EntryPoint別キャッシュ、実際のルーティングはPoCで確認する。

Honoの`app.onError()`は応答開始前の例外処理に有効だが、**SSEストリーム開始後の失敗をHTTP 500へ巻き戻すことはできない**。断片単位のエラーイベント、ログ、タイムアウトを別途実装する。

**比較上の注意**。HTTPのルート数がごく少なければNative `fetch()`のままでも十分に明快である。Honoへの移行メリットは単に「速い」ことではなく、今後のページ・API・認証・SSEエンドポイント増加に対して規約化されたルートとMiddlewareの管理ができる点にある。Middlewareを無差別に積み重ねると、逆に処理の見通しが悪くなる。

## 8. エスケープとSSEのHTML挿入に関するセキュリティ境界

**Honoの自動エスケープは必要条件だが、SSEでブラウザにHTMLを挿入する安全性を単独で保証するものではない。** レイヤーごとに明確な規約を置く。

1. **HTMLテキスト・通常属性**。外部データは`html\`...${value}...\``の通常補間だけで渡す。自分でエスケープ済みの文字列を`raw()`で入れない。HTML断片を合成する際も、手書きの文字列連結で安全印を偽造しない。
2. **`raw()`とテンプレート本文**。`raw()`はエスケープを迂回する。テンプレート内に直接書いたHTMLや`<script>`も自動サニタイズされない。未信頼入力、外部CMSのHTML、リッチテキストを無検査で通さない。
3. **URL・CSS・JSという別文脈**。HTML属性をエスケープしても`href="javascript:..."`等の危険なURLスキームは検証されない。URLは許可するスキーム・相対パスを明示し、イベントハンドラー属性と動的インラインScriptは原則禁止する。
4. **SSEエンベロープ**。断片HTMLはSSEの`data`へJSONとして格納し、`writeSSE()`へ渡す。手動で`data: ${html}\n\n`と連結しない。JSONを解析してから、既知の断片IDのみに適用する。
5. **DOM挿入**。`innerHTML`はDeclarative Shadow DOMを構築しない。DSDを含む断片には`setHTMLUnsafe()`等のDSD対応APIが必要だが、**`setHTMLUnsafe()`は既定ではサニタイズしないXSS sink**である。許可済みサーバーテンプレートのみを挿入する、必要に応じてSanitizer/Trusted Typesを導入する、外部HTMLは別途サニタイズする、という多層防御をとる。SanitizerがDSDやCustom Elementsの必要な要素を残すかは実ブラウザで検査する。
6. **CSP**。SSE受信とDOM更新を外部module JSにまとめ、`script-src 'self'`等でインライン差し替えスクリプトを不要にする。Client Island内の`<style>`のCSP hash、`style-src`、Trusted Typesとの整合性は維持・再テストする。
7. **認証・キャッシュ**。SSE URLへ秘密情報を載せず、同一オリジン・Cookie認証を基本にする。接続ごとにユーザー権限を検査し、レスポンスを`no-store`にする。SSEは読み取り専用GETとし、状態変更は別の適切なAPIで扱う。

最小の受信処理は次のような形になる。ただし、ここでは**断片がすべて信頼できるサーバーテンプレートから生成され、IDが許可されている**ことを前提とし、製品コードではポリシー・Sanitizer・認証・接続管理を補う。

```js
const allowedIds = new Set(['popular', 'recommended'])
const source = new EventSource('/articles/fragments')

source.addEventListener('fragment', (event) => {
  const payload = JSON.parse(event.data)
  if (!allowedIds.has(payload.id) || typeof payload.html !== 'string') return
  const target = document.getElementById(payload.id)
  if (!target || target.dataset.completed === 'true') return
  // DSDを含む可能性があるためinnerHTMLは使わない。
  // payload.htmlは信頼できるサーバーテンプレートからの生成に限定する。
  target.setHTMLUnsafe(payload.html)
  target.dataset.completed = 'true'
})

source.addEventListener('fragment-error', (event) => {
  // 必要に応じて失敗した部分だけエラー状態へ変更する。
})
source.addEventListener('done', () => source.close())
```

## 9. Client Islandsをどうするか

Client Islandsの基本構造は**変更しない**。SSR用のHono `html`関数と、ブラウザ用Custom Elementを一組として配置する。

- SSR部品は`*.ts`の`html`関数に変更する。`<site-search><template shadowrootmode="open">...</template></site-search>`を返し、JSがなくてもフォームが機能するHTMLを作る。
- ブラウザ側は`*.element.ts`のCustom Elementとし、既存のDOMを保持したままイベント、状態、動作を追加する。Hydrationは行わない。
- SSEで届く断片に**新しい**Client Islandが含まれる場合、DSD対応パーサーで構築し、定義済みCustom Elementのupgradeに任せる。早期起動、ShadowRoot内部の構築完了、再接続、解放は従来どおり検証する。
- **既に稼働中のClient IslandをSSEで置き換えない**。その内部に差し替え対象を設けることも標準では認めない。更新が必要ならCustom Elementの専用APIまたは別枠の静的プレースホルダーとして明示する。
- ページにClient IslandもServer Islandもなければ、ブラウザへフレームワークJSを送らない。Server Islandを使うページにはSSE受信の小さな外部JSだけを追加する。
- DSD内のCSSを`<style>`として生成する既存方針は維持。ただしHTML挿入APIとSanitizerの相互作用、重複Style、CSP hashを確認する。

## 10. 移行順序と受け入れ条件

### フェーズ1　Hono htmlによるHTML生成への置換

- 代表的な`static`・`cache`・`dynamic`のページを同一の`hono/html`関数で生成する。
- テキスト・属性・URL、フォーム、DSD、CSS、エラー時HTTPステータスについて出力を比較する。
- VNode/Preactへの依存を切り、TSXの型拡張とPreact固有のAST検査を整理する。新しいHTML・import境界検査を負例テスト付きで実装する。

### フェーズ2　Server IslandsをSSEへ移行

- 二つ以上の非同期断片でA=遅い、B=速いケースを再現し、Bを先に表示できることをブラウザで測る。
- エスケープ、CSP、DSD、Client Islandのupgrade、接続切断、再接続、失敗、タイムアウト、同時完了の順序を検査する。
- 基本コンテンツがSSEなしでも閲覧できることを確認する。
- 初期HTMLとSSEを合わせた表示速度・リクエスト数・CPU・メモリ・送信バイト数をPreact Suspense版と比較する。

### フェーズ3　Router・Middlewareの移行

- Honoのルート宣言と既存のページ`mode`宣言の対応を一元化する。
- 認証、URL正規化、セキュリティヘッダー、ログ、404/405、例外処理の適用順をテストする。
- Cache OFF Gateway→Cache ON Named EntryPointへの呼び出し、Static Assetsとのルート衝突を確認する。

### フェーズ4　CI、Preview、本番

- `pnpm install --frozen-lockfile`からビルドとworkerd内のVitestが再現する。
- `createTestHarness()`でビルド成果物とStatic AssetsのHTTP結合テストを実施する。
- Cloudflare実環境でCache HIT/MISS、認証境界、Static Assetsの配信・課金を確認する。
- AIがPRを作り、CIと人間のPreview確認を経てマージ・デプロイする既存運用は維持する。

**受け入れ条件**は、①3モードを維持、②SSEによる順不同表示が実証される、③DSD/Custom Elementsが問題なく起動する、④エスケープ・CSP・認証・キャッシュ分離がテストで説明できる、⑤workerdとCloudflare本番で動作する、⑥Preact除去の実益（依存量・コード量・計測値）を確認できること、とする。

## 11. 代替案・残るリスク

この提案は決定事項ではなく、PreactのSuspenseが提供していた完成済みの非同期境界管理を**明示的に小さな共通処理へ移す**変更である。SSEの追加リクエスト、認証、切断、再接続、HTML挿入という運用負担があるため、単なるパッケージ削除ではない。

最大の分岐は**SSEクライアントを実装・保守する負担が、VNode/Preact依存を外す利益より小さいか**である。もしSSEで表示開始が明確に遅くなる、複雑なネストや再試行を要する、重要コンテンツまでJS依存になるのであれば、Preact Suspenseを維持する方が合理的である。反対に、Server Islandsを「固定IDの独立したHTML断片を一度だけ差し替える」用途に限定できれば、独自処理は小さく保てる。

また、Hono Router・MiddlewareはPreactの撤去とは別の設計判断であり、段階を分けて検証できる。`hono/html`だけを採用してNative `fetch()`を続けることも可能だが、長期的な共通基盤としてはHonoのHTTP層へ寄せるメリットがある、というのが本提案の立場である。

## 12. 提案する決定

1. **HTML生成の標準をPreact JSXから`hono/html`に変更する。Hono JSXは使用しない。**
2. **HonoのRouter・MiddlewareをHTTPの標準入口とし、CloudflareのBindings・EntryPoint・Workers Cacheは引き続きCloudflare側の責任とする。**
3. **Server Islandsの順不同表示を、初期HTMLのfallback + `streamSSE()` + 外部JSによる限定的なDOM断片挿入に変更する。** SSE自体は必須技術ではなく、今回の選択であることを明記する。
4. **Client IslandsはDSD + Custom Elementsのまま維持し、SSR生成部分だけをHono `html`へ変更する。**
5. **Vite、Cloudflare Vite Plugin、workerd、Vitest、pnpm 12管理のNode.js、CI・AI Land、3配信モードは維持する。**
6. **まずPoCで検証し、テスト合格と実測で移行を確定する。** 本文中の概念コードを無検証の本番実装と見なさない。

---

## 参考資料

### 現行設計（提案の基準）

- `pretty-islands-framework.md`、2026年10月28日最新版。特に1章、3.4節、5〜13章、16〜20章。

### 公式仕様・一次資料

- [Hono HTML Helper](https://hono.dev/docs/helpers/html)　`html`、`raw`、HTML断片の関数合成。
- [Hono Streaming Helper](https://hono.dev/docs/helpers/streaming)　`stream()`、`streamSSE()`、ストリーム開始後のエラーの扱い。
- [Hono Routing](https://hono.dev/docs/api/routing)　Router、登録順、HTTPメソッド。
- [Hono Middleware](https://hono.dev/docs/guides/middleware)　`app.use()`、型付きMiddleware、例外処理。
- [Hono App API](https://hono.dev/docs/api/hono)　`app.fetch()`、`app.route()`、`app.onError()`。
- [Hono JSX](https://hono.dev/docs/guides/jsx)　`hono/jsx/streaming`のSuspenseは`hono/html`とは別の機能。
- [Cloudflare Hono Guide](https://developers.cloudflare.com/workers/framework-guides/web-apps/more-web-frameworks/hono/)　HonoとCloudflare Vite Pluginの併用。
- [Cloudflare Workers Cache Configuration](https://developers.cloudflare.com/workers/cache/configuration/)　EntryPoint単位のCacheとGateway構成。
- [Cloudflare Workers Context API](https://developers.cloudflare.com/workers/runtime-apis/context/)　`ctx.exports`と必要なcompatibility flag。
- [Cloudflare Workers Streams](https://developers.cloudflare.com/workers/runtime-apis/streams/)　ReadableStreamによるレスポンス配信。
- [MDN EventSource](https://developer.mozilla.org/en-US/docs/Web/API/EventSource/EventSource)　SSE受信APIの性質と認証オプション。
- [MDN setHTMLUnsafe](https://developer.mozilla.org/en-US/docs/Web/API/Element/setHTMLUnsafe)　DSD対応とXSS sinkとしての注意。
- [MDN innerHTML](https://developer.mozilla.org/en-US/docs/Web/API/Element/innerHTML)　DSDを構築しない挙動。

**検証状況の区別**　HonoのAPI、Cloudflare Cache、EventSource、DOMの仕様は上記公式資料で確認した。Pretty Islands固有のHono版構成、SSE断片挿入、Named EntryPoint連携、性能・課金・本番動作は本提案では未検証であり、10章のテストで確認する。
