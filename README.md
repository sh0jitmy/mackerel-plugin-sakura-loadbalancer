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

`mkr plugin install` でインストールした場合、プラグインバイナリはデフォルトで以下のパスに配置されます：

- `/opt/mackerel-agent/plugins/bin/mackerel-plugin-sakura-loadbalancer`

インストール後、`/etc/mackerel-agent/mackerel-agent.conf` に以下のようにプラグインの実行設定を追加します。API アクセストークンなどの機密情報は、コマンド引数ではなく `env` 項目を使用して環境変数として渡すことを推奨します。

```toml
[plugin.metrics.sakura-loadbalancer]
command = ["/opt/mackerel-agent/plugins/bin/mackerel-plugin-sakura-loadbalancer", "-lb-id", "123456789012", "-server-ip", "192.168.1.10"]
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

### 2. 動作確認と切り分け手順 (デバッグログモード)

手動実行時に詳細な調査・切り分けを行うため、`--debug` フラグ（または環境変数 `DEBUG=true`）が利用可能です。デバッグログは標準エラー出力（stderr）に出力されるため、Mackerel プラグインの動作を妨げません。

#### 実行手順
```bash
# デバッグログ付きで手動実行する例
./bin/mackerel-plugin-sakura-loadbalancer \
  --token="<SAKURA_ACCESS_TOKEN>" \
  --secret="<SAKURA_ACCESS_TOKEN_SECRET>" \
  --zone="is1a" \
  --lb-id="123456789012" \
  --server-ip="192.168.1.10" \
  --debug
```

#### 各ケースにおける出力結果

##### ケースA：正常検出時（指定サーバがロードバランサで稼働中）
- **標準出力（stdout）**: Mackerel用の正常値（`status` が `1`）が出力されます。
  ```text
  loadbalancer.target.status	1.000000	1718000000
  loadbalancer.target.cps	45.000000	1718000000
  loadbalancer.target.active_conn	5.000000	1718000000
  loadbalancer.server.status.192_0_2_1_80	1.000000	1718000000
  loadbalancer.server.cps.192_0_2_1_80	45.000000	1718000000
  loadbalancer.server.active_conn.192_0_2_1_80	5.000000	1718000000
  ```
- **標準エラー出力（stderr/デバッグログ）**:
  ```text
  2026/07/12 19:33:38 [DEBUG] Starting metrics fetch for LoadBalancer ID: 123456789012, Zone: is1a, Target Server IP: 192.168.1.10
  2026/07/12 19:33:38 [DEBUG] Successfully fetched status from API. Found 1 VIP configurations.
  2026/07/12 19:33:38 [DEBUG] Checking VIP: 192.0.2.1, Port: 80 (number of servers: 1)
  2026/07/12 19:33:38 [DEBUG]   - Server IP: 192.168.1.10, Status: up, CPS: 45.00, ActiveConn: 5.00
  2026/07/12 19:33:38 [DEBUG]     => Matches target server! Storing individual metrics. Status: 1.0
  2026/07/12 19:33:38 [DEBUG] Target server found on LoadBalancer. Target all UP: true, Total CPS: 45.00, Total ActiveConn: 5.00
  ```

##### ケースB：異常検出時（指定サーバがDOWN、またはロードバランサに設定されていない）
- **標準出力（stdout）**: 異常値（`status` が `0`）が出力されます。
  ```text
  loadbalancer.target.status	0.000000	1718000000
  loadbalancer.target.cps	0.000000	1718000000
  loadbalancer.target.active_conn	0.000000	1718000000
  ```
- **標準エラー出力（stderr/デバッグログ - DOWN時）**:
  ```text
  2026/07/12 19:33:38 [DEBUG] Starting metrics fetch for LoadBalancer ID: 123456789012, Zone: is1a, Target Server IP: 192.168.1.10
  2026/07/12 19:33:38 [DEBUG] Successfully fetched status from API. Found 1 VIP configurations.
  2026/07/12 19:33:38 [DEBUG] Checking VIP: 192.0.2.1, Port: 80 (number of servers: 1)
  2026/07/12 19:33:38 [DEBUG]   - Server IP: 192.168.1.10, Status: down, CPS: 0.00, ActiveConn: 0.00
  2026/07/12 19:33:38 [DEBUG]     => Matches target server! Storing individual metrics. Status: 0.0
  2026/07/12 19:33:38 [DEBUG] Target server found on LoadBalancer. Target all UP: false, Total CPS: 0.00, Total ActiveConn: 0.00
  ```
- **標準エラー出力（stderr/デバッグログ - 設定自体が存在しない場合）**:
  ```text
  2026/07/12 19:33:38 [DEBUG] Starting metrics fetch for LoadBalancer ID: 123456789012, Zone: is1a, Target Server IP: 192.168.1.10
  2026/07/12 19:33:38 [DEBUG] Successfully fetched status from API. Found 1 VIP configurations.
  2026/07/12 19:33:38 [DEBUG] Checking VIP: 192.0.2.1, Port: 80 (number of servers: 1)
  2026/07/12 19:33:38 [DEBUG]   - Server IP: 192.168.1.99, Status: up, CPS: 10.00, ActiveConn: 1.00
  2026/07/12 19:33:38 [DEBUG] Target server 192.168.1.10 was NOT configured on this LoadBalancer. Setting target status to 0.0
  ```

##### ケースC：設定不足・APIエラーによる動作不良
- **標準出力（stdout）**: 何も出力されません（Mackerelエージェントは値の収集をスキップします）。
- **標準エラー出力（stderr/デバッグログ - トークン等の未指定）**:
  ```text
  Sakura Cloud Access Token and Secret are required (via --token and --secret flags or SAKURACLOUD_ACCESS_TOKEN and SAKURACLOUD_ACCESS_TOKEN_SECRET env vars)
  exit status 1
  ```
- **標準エラー出力（stderr/デバッグログ - API通信エラーや認証情報の誤り）**:
  ```text
  2026/07/12 19:33:38 [DEBUG] Starting metrics fetch for LoadBalancer ID: 123456789012, Zone: is1a, Target Server IP: 192.168.1.10
  2026/07/12 19:33:38 [DEBUG] API call failed: sacloud: Resource not found: ... (または 401 Unauthorized など)
  failed to output metrics: failed to get load balancer status: ...
  exit status 1
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
