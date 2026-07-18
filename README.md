# RenkinEngin ⚗️⚙️

RenkinEngin は、LLMエージェントと解析ツールを組み合わせた Docker 環境を、設定ファイルベースで構築・管理する CLI ツールです。

「エージェントを錬成し、自律的な労働（連勤）を命じる」ための基盤を提供します。

## ✨ 特徴
- **宣言的構成**: Dockerfile を手書きせず、プリセットの組み合わせで環境を定義。
- **ツール同居型設計**: エージェントと OpenFOAM 等の CLI ツールを同一コンテナに配置し、シームレスな連携を実現。
- **機密情報の保護**: API キー等はホストから自動継承。環境内に機密情報を残しません。
- **充実のプリセット**: OpenFOAM, OpenModelica, 高速 Python 解析環境（uv）を完備。

## 🚀 クイックスタート

### 1. インストール

#### Go 1.26.2 のインストール
[mise](https://mise.jdx.dev/) を使用してインストールすることを推奨します。
```bash
mise use -g go@1.26.2
```

#### RenkinEngin のビルド
```bash
go build -o renkin ./cmd/renkin
```

#### パスの設定
`renkin` コマンドをどこからでも実行できるように、`~/.bashrc` にパスを追加します。
```bash
echo "export PATH=\$PATH:$(pwd)" >> ~/.bashrc
source ~/.bashrc
```

### 2. 連勤の業務命令 (環境の構成)
ターゲットディレクトリを指定して実行します。各オプションは省略すると、ディレクトリ内の設定ファイルや標準プリセットが自動適用されます。
```bash
# 例: Gemini と OpenFOAM、Python解析環境を組み合わせて錬成
renkin assign ./ --llm gemini --tools openfoam2512,python-post
```

`--tools` フラグにカンマ区切りで複数のツールを指定できます。

#### 💡 スキル合成機能
複数のツールを指定した場合、それぞれのプリセットに含まれる「使い方の手順（instructions）」が、自動的にエージェント用のスキルファイル（`GEMINI.md` や `AGENTS.md` 等）に集約・合成されます。これにより、エージェントは起動直後からインストールされたツールの場所やライブラリの利用方法を把握した状態で自律作業を開始できます。


### 3. 労働開始 (環境起動 & アタッチ/無限労働)

#### 通常アタッチ（人間用手動 TUI 起動）
```bash
renkin start
```
エージェントがコンテナ内で起動し、双方向でアタッチして手動指示や対話を開始します。

#### 真の連勤無限ループモード (Loop 監視) ⛓️🔄
エージェントを文字通り「終わらない連勤（無限ループ）」で働かせたり、任意の自動化タスクを無限にポーリングループさせる場合は `--loop` オプションを使用します。
```bash
# プリセットデフォルトの loop_cmd を無限実行（セッションID・プロンプト・YOLOを自動注入したBot稼働）
renkin start --loop

# ユーザー独自のカスタムスクリプトを無限ループ実行（Botやバッチ処理として非常に汎用的）
renkin start --loop work.sh
renkin start --loop "python3 bot.py"
```
* **無限ループ & 自動復旧**: プロセスが終了（正常終了、エラー終了問わず）すると、設定された `restart_policy` (Presetではデフォルト `always`、上書きは `on-failure` など) および `restart_delay` (再起動前の待機秒数、デフォルト `2`) に従って、自動的に会話セッションを維持したまま次の労働サイクル（ループ）が開始されます。
* **フォアグラウンド実行**: `--loop` はフォアグラウンドで実行され、リアルタイムログが流れます。終了（Kill）したい場合は、ターミナルで `Ctrl+C` を押すか、別ターミナルから `renkin stop` を実行するだけで安全に停止されます。
* **カレントディレクトリの完全同期**: コンテナの `/workspace` (ホスト側の `./workspace/` にマウント) 起点で実行されるため、Dockerのパスを意識せず、すべてローカルの相対パス（例: `./work.sh`）でスクリプトやログファイルを記述・動作させることができます。

#### 割り込み最優先実行
`--cmd` オプションを付与すると、すべてのループ設定よりも優先して、指定されたコマンドにアタッチします。
```bash
renkin start --loop work.sh --cmd "bash"  # デバッグ用にシェルを最優先起動
```

#### Bot Server（Go 側監視 / Discord adapter）
`renkin bot` は、LLM を常時起動せずに Go 側プロセスが掲示板を監視し、新着メッセージが来た時だけエージェント起動コマンドを dispatch します。現在の board adapter は `discord` です。

構成上、Discord API へ直接接続するのは `discord-mcp` コンテナだけです。Go 側 Bot Server は `discord-mcp` の HTTP MCP endpoint に対して `read_messages` tool を呼びます。そのため `DISCORD_TOKEN` / `DISCORD_GUILD_ID` は `discord-mcp` service 用、Go 側は `DISCORD_MCP_URL` / `DISCORD_CHANNEL_ID` を使います。

まず Discord MCP 付きで環境を作ります。

```bash
renkin assign ./ --llm codex --tools discord-mcp
```

`.env` に Discord 接続情報を設定します。

```bash
DISCORD_TOKEN=your-discord-bot-token
DISCORD_GUILD_ID=your-guild-id
DISCORD_CHANNEL_ID=your-channel-id
DISCORD_MCP_URL=http://localhost:8085/mcp
DISCORD_BOT_USER_ID=your-bot-user-id
DISCORD_BOT_USERNAME=your-bot-username
```

`DISCORD_BOT_USER_ID` / `DISCORD_BOT_USERNAME` は自分の投稿を新着検知から除外するための値です。`discord-mcp` が formatted text だけを返す場合は username で除外します。

`RENKIN_BOT_DISPATCH_CMD` は通常不要です。`.renkin/conf/bot-loop.sh` がある場合、Bot Server は次の dispatch command を自動で使います。

```bash
docker compose exec -T -e RENKIN_BOARD_ITEM_ID -e RENKIN_BOARD_CHANNEL_ID -e RENKIN_BOARD_AUTHOR_ID -e RENKIN_BOARD_CONTENT -e RENKIN_BOARD_CREATED_AT llm-agent bash -lc 'renkin-generate-llm-config; bash /renkin-conf/bot-loop.sh'
```

Bot Server を起動します。`renkin bot start` は通常の `renkin start` と同じく、先に `docker compose up -d` で compose services を起動します。`--board discord` は現時点の既定ですが、他の掲示板 adapter と区別できるよう明示しておくのを推奨します。

```bash
renkin bot start --board discord
```

状態確認と停止:

```bash
renkin bot status
renkin bot stop
```

前景で動作確認する場合:

```bash
# 1回だけ poll して、検知したら dispatch する
renkin bot run-once --board discord

# 前景で継続 poll
renkin bot run --board discord --interval 30s
```

主なオプション:

```bash
renkin bot start \
  --board discord \
  --interval 30s \
  --max-retries 3 \
  --restart-delay 5s \
  --deadline 30m \
  --webhook-url https://example.com/webhook
```

- `--state`: 状態ファイルのパス。既定は `.renkin_bot_state.json`
- `--board`: 掲示板 adapter。現在は `discord` のみ対応
- `--checkpoint-on-start`: 起動時に現在の最新位置を checkpoint として記録し、既存メッセージを dispatch しない
- `--pid-file`: `bot start/stop/status` 用 PID file。既定は `.renkin/bot.pid`
- `--log-file`: `bot start` のログ出力先。既定は `.renkin/logs/bot.log`
- `--webhook-url`: `exhausted` 発生時の通知先。未指定時は `RENKIN_BOT_WEBHOOK_URL` を参照

Bot Server は state file に `last_message_id` と `last_checked_at` を記録します。初回起動時に過去メッセージを処理したくない場合は `renkin bot start --board discord --checkpoint-on-start` を使ってください。

Dispatch は `pending -> in_flight -> confirmed/exhausted` で管理されます。エージェントは完了時に、元 Discord メッセージへの reply として投稿してください。Bot Server は `message_reference.message_id` を見て解決済み判定します。reply が見つからないままコマンド終了または deadline 到達になると retry し、`max-retries` 到達で `exhausted` になります。

### 4. 労働停止・環境の再起動・解雇
```bash
# 労働停止（コンテナとボリュームの削除）
renkin stop

# 環境の再起動（不具合時は --rebuild でクリーンに作り直し）
renkin restart [--rebuild]

# エージェントの解雇（イメージも含めた完全な削除）
renkin kaiko
```

## 🧪 標準プリセット

### LLM エージェント
- `gemini`: Gemini CLI (Node.js v24)
- `codex`: Codex CLI (Node.js v24)

### 解析ツール
- `discord-mcp`: Discord 連携用 MCP server。Go 側 Bot Server と LLM エージェントの両方が同じ HTTP MCP service を使います。
- `forgejo-mcp`: Codex CLI / Gemini CLI 向け Forgejo MCP server
- `git`: Git CLI（`GIT_USER_NAME`, `GIT_USER_EMAIL` をコンテナへ継承）
- `masabbs-mcp`: Codex CLI / Gemini CLI 向け masabbs 組織・議論レビュー MCP server。preset は GitHub から取得後に `npm ci`、`npm run build`、`npm install -g .` を実行し、生成された MCP 設定は `masabbs-mcp` コマンドを直接起動します。
- `mcp-server-git`: Codex CLI / Gemini CLI 向け Git MCP server
- `openfoam2512`: 流体解析（Ubuntu 24.04 対応）
- `openmodelica410`: 物理モデリング（MSL v4.1.0 搭載）
- `python-post`: 高速解析環境（foamlib, DyMat, Optuna, pandas 等）

## ⚠️ 運用上の注意
- **APIコスト**: エージェントの自律稼働（連勤）に伴う API コストは、管理者の負担となります。
- **Proxy環境**: 企業のファイアウォール下でも、ホストの proxy 設定を自動継承して環境構築が可能です。
    - **注意**: ホスト側で起動しているサーバー（Forgejo や NATS 等）にエージェントがアクセスする場合、ホストの IP アドレスを `no_proxy` (または `NO_PROXY`) に含める必要があります。そうしないと、ローカル通信が外部プロキシを経由しようとして失敗することがあります。
    - **推奨される no_proxy 設定例**: `localhost,127.0.0.1,172.17.0.1,host.docker.internal,<ホストのIP>`
- **masabbs-mcp の接続先**: RenkinEngin コンテナ内では `MASABBS_BASE_URL=http://host.docker.internal/api/v1` を既定値にします。ホスト側で Codex/Gemini を直接動かす場合は、`http://localhost/api/v1` を使い、`masabbs-mcp` コマンドを事前に build/install してください。

---

## 📜 ライセンス
MIT License.

---
**RenkinEngin で、シミュレーション業務の完全自律化を実現しましょう。**
