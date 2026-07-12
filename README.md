# mackerel-plugin-sakura-loadbalancer

さくらのクラウドのロードバランサアプライアンスから、特定の実サーバ（バックエンドサーバ）の監視ステータス、CPS（Connections Per Second）、およびアクティブコネクション数を取得し、Mackerel に投稿するための `mackerel-agent-plugin` です。

さくらのクラウド公式 Go SDK である `sacloud/iaas-api-go` を使用してロードバランサの状態を取得します。

---

## 🚀 特徴

1. **特定実サーバの稼働監視 (正常/異常)**:
   - 指定した実サーバのIPアドレスがロードバランサ内に設定されており、そのヘルスチェックステータスが `UP` であれば `1`（正常）、それ以外（`DOWN` または存在しない）は `0`（異常）としてサマリーメトリクスを報告します。
2. **CPS & アクティブコネクション数の収集**:
   - 監視対象サーバの CPS 値とアクティブコネクション数を集計して報告します。
3. **複数 VIP・ポート対応 (個別メトリクス)**:
   - 同一の実サーバが複数の仮想IP（VIP）およびポートにマッピングされている場合でも、それぞれのインスタンスごとに詳細なステータスと CPS を個別に監視し投稿します。
4. **Mackerel エージェントプラグイン仕様準拠**:
   - `MACKEREL_AGENT_PLUGIN_META=1` が指定された際に、自動的にグラフ定義メタデータ（JSON）を標準出力します。

---

## 📥 インストール方法

Mackerel の `mkr` CLI を使用して、GitHub から直接本プラグインをインストールできます。

```bash
mkr plugin install sh0jitmy/mackerel-plugin-sakura-loadbalancer
```

特定のバージョン（リリース）を指定してインストールする場合は、以下のように指定します。

```bash
mkr plugin install sh0jitmy/mackerel-plugin-sakura-loadbalancer@[version] #v0.0.1等

```

---

## 📝 Mackerel エージェントの設定例

インストール後、`/etc/mackerel-agent/mackerel-agent.conf` に以下のように設定します。

API アクセストークンなどの機密情報は、プラグインの引数ではなく `env` 項目を使用して環境変数として渡すことを推奨します。

```toml
[plugin.metrics.sakura-loadbalancer]
command = ["mackerel-plugin-sakura-loadbalancer", "-lb-id", "123456789012", "-server-ip", "192.168.1.10"]
env = { SAKURACLOUD_ACCESS_TOKEN = "<APIトークン>", SAKURACLOUD_ACCESS_TOKEN_SECRET = "<APIシークレット>", SAKURACLOUD_ZONE = "is1a" }
```

---

## 🛠️ クイックスタート

### 1. ビルド
Go 1.26.5 以上が必要です。

```bash
make build
```
ビルドが成功すると、`bin/mackerel-plugin-sakura-loadbalancer` にバイナリが生成されます。

### 2. 動作確認
環境変数またはコマンドライン引数経由で API 情報を渡して実行します。

```bash
# コマンド引数による実行例
./bin/mackerel-plugin-sakura-loadbalancer \
  -token="<SAKURA_ACCESS_TOKEN>" \
  -secret="<SAKURA_ACCESS_TOKEN_SECRET>" \
  -zone="is1a" \
  -lb-id="123456789012" \
  -server-ip="192.168.1.10"
```

---

## ⚙️ 設定オプション

### コマンドライン引数

| 引数 | 環境変数 | デフォルト値 | 説明 |
| :--- | :--- | :--- | :--- |
| `-token` | `SAKURACLOUD_ACCESS_TOKEN` / `SAKURA_ACCESS_TOKEN` | - | さくらのクラウド API アクセストークン |
| `-secret` | `SAKURACLOUD_ACCESS_TOKEN_SECRET` / `SAKURA_ACCESS_TOKEN_SECRET` | - | さくらのクラウド API アクセストークンシークレット |
| `-zone` | `SAKURACLOUD_ZONE` / `SAKURA_ZONE` | `is1a` | 対象のさくらのクラウドゾーン (例: `is1a`, `is1b`, `tk1a`, `tk1b`) |
| `-lb-id` | - | - | 監視対象ロードバランサのリソースID（数字） **(必須)** |
| `-server-ip` | - | - | 監視対象の実サーバ（バックエンドサーバ）のIPアドレス **(必須)** |
| `-metric-key-prefix` | - | `loadbalancer` | メトリクス名の接頭辞 |

---

## 📊 収集されるメトリクス

本プラグインは、以下のメトリクス群を Mackerel に報告します（プレフィックスが `loadbalancer` の場合）。

### 1. サマリーメトリクス (指定したサーバの総合ステータス)
監視対象のサーバが、ロードバランサ内で稼働している全体のステータスを表します。

- `custom.loadbalancer.target.status.status`: 総合ステータス (`1.0` = 正常/すべてのインスタンスがUP, `0.0` = 異常/いずれかがDOWN、または設定なし)
- `custom.loadbalancer.target.cps.cps`: 監視対象サーバの合計 CPS（接続/秒）
- `custom.loadbalancer.target.active_conn.active_conn`: 監視対象サーバの合計アクティブコネクション数

### 2. インスタンスメトリクス (個別ポート監視用)
同一の実サーバが複数の仮想IPおよびポートにマッピングされている場合、以下のルールで個別のグラフとメトリクスが動的に生成されます。

- `custom.loadbalancer.server.status.<vip_port>`: 特定 VIP＋Port のステータス (`1.0` = UP, `0.0` = DOWN)
- `custom.loadbalancer.server.cps.<vip_port>`: 特定 VIP＋Port の CPS
- `custom.loadbalancer.server.active_conn.<vip_port>`: 特定 VIP＋Port のアクティブコネクション数

> *※ `<vip_port>` は、仮想IPアドレスのドットをアンダースコアに変換し、ポート番号を連結したものです (例: `192_0_2_1_80`)*

---

## 開発コマンド

Makefile に定義されている以下のコマンドを使用して品質管理やテストを行います。

| コマンド | 説明 |
| :--- | :--- |
| `make fmt` | ソースコードのフォーマットおよびリンターによる自動修正 |
| `make lint` | `golangci-lint` を使用した静的解析の実行 |
| `make tidy` | 依存関係 (`go.mod` / `go.sum`) の整理 |
| `make vulncheck` | `govulncheck` を使用した脆弱性診断の実行 |
| `make test` | データ競合検知 (`-race`) およびカバレッジ（80%以上）測定付きテストの実行 |
| `make build` | `bin/mackerel-plugin-sakura-loadbalancer` へのコンパイルの実行 |
| `make self-eval` | リポジトリが品質要件を満たしているかの自己評価の実行 (`REQUIREMENTS.md` の更新) |
| `make clean` | ビルド成果物やテストキャッシュのクリーンアップ |

---

## 📄 ライセンス

本プラグインは **Apache License 2.0** のもとで公開されています。
詳細については、[LICENSE](LICENSE) ファイルを参照してください。
