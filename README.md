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
* **無限ループ & 自動復旧**: プロセスが終了（正常終了、エラー終了問わず）すると、設定された `restart_policy` (Presetではデフォルト `always`) に従って、自動的に会話セッションを維持したまま次の労働サイクル（ループ）が即時に開始されます。
* **フォアグラウンド実行**: `--loop` はフォアグラウンドで実行され、リアルタイムログが流れます。終了（Kill）したい場合は、ターミナルで `Ctrl+C` を押すか、別ターミナルから `renkin stop` を実行するだけで安全に停止されます。
* **カレントディレクトリの完全同期**: コンテナの `/workspace` (ホスト側の `./workspace/` にマウント) 起点で実行されるため、Dockerのパスを意識せず、すべてローカルの相対パス（例: `./work.sh`）でスクリプトやログファイルを記述・動作させることができます。

#### 割り込み最優先実行
`--cmd` オプションを付与すると、すべてのループ設定よりも優先して、指定されたコマンドにアタッチします。
```bash
renkin start --loop work.sh --cmd "bash"  # デバッグ用にシェルを最優先起動
```

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
- `forgejo-mcp`: Codex CLI / Gemini CLI 向け Forgejo MCP server
- `git`: Git CLI（`GIT_USER_NAME`, `GIT_USER_EMAIL` をコンテナへ継承）
- `mcp-server-git`: Codex CLI / Gemini CLI 向け Git MCP server
- `openfoam2512`: 流体解析（Ubuntu 24.04 対応）
- `openmodelica410`: 物理モデリング（MSL v4.1.0 搭載）
- `python-post`: 高速解析環境（foamlib, DyMat, Optuna, pandas 等）

## ⚠️ 運用上の注意
- **APIコスト**: エージェントの自律稼働（連勤）に伴う API コストは、管理者の負担となります。
- **Proxy環境**: 企業のファイアウォール下でも、ホストの proxy 設定を自動継承して環境構築が可能です。
    - **注意**: ホスト側で起動しているサーバー（Forgejo や NATS 等）にエージェントがアクセスする場合、ホストの IP アドレスを `no_proxy` (または `NO_PROXY`) に含める必要があります。そうしないと、ローカル通信が外部プロキシを経由しようとして失敗することがあります。
    - **推奨される no_proxy 設定例**: `localhost,127.0.0.1,172.17.0.1,host.docker.internal,<ホストのIP>`

---

## 📜 ライセンス
MIT License.

---
**RenkinEngin で、シミュレーション業務の完全自律化を実現しましょう。**
