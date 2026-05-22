# RenkinEngin 仕様書

**バージョン**: 1.1.1  
**作成日**: 2026-05-17

---

## 1. 概要

RenkinEnginは、異なるLLMエージェント（Gemini CLI, Codex等）と各種ツール（OpenFOAM, OpenModelica, Python解析環境等）を組み合わせたDockerコンテナ環境を、設定ファイルベースで簡単に構築・起動するCLIツールである。

### 設計思想

- **設定ファイルによる宣言的構成**: Dockerfile等を手書きせず、設定ファイルやプリセットの組み合わせでコンテナを定義する
- **LLMとツールの同居**: LLMエージェントとshellツールは同一コンテナ内に配置する
- **MCPツールは別コンテナ**: MCP serverを持つツールはdocker compose の別serviceとして起動する
- **workspaceによるデータ共有**: ホストとコンテナエージェント間のデータやり取りはworkspaceディレクトリを通じて行う
- **環境変数継承**: API Keyやプロキシ設定はホストOS側から自動継承・チェックを行う
- **スキル合成**: ツールごとの利用手順を自動的にLLM用のスキルファイル（GEMINI.md等）に集約する


---

## 2. CLIインターフェース仕様

### 2.1 サブコマンド一覧

```
renkin <subcommand> [options]
```

| サブコマンド | 説明 |
|-------------|------|
| `assign`    | 設定ファイルからDockerfile・docker-compose.yml等を生成する |
| `start`    | カレントディレクトリのdocker-compose.ymlを起動し、LLMエージェントにアタッチする |
| `end`  | カレントディレクトリのdocker-compose.ymlのコンテナ群を停止・削除する |
| `tool`      | プリセットされているツールの一覧表示や詳細確認を行う |

---

### 2.2 renkin assign

```bash
renkin assign <target_dir> \
  [--docker docker.conf|preset_name] \
  [--tools tool_list.toml|preset_name,...] \
  [--llm llm.conf|preset_name] \
  [--skills skills.md]
```

#### 引数・オプション

| 引数/オプション | 必須 | 説明 |
|----------------|------|------|
| `<target_dir>` | 必須 | 生成物の出力先ディレクトリ |
| `--docker`     | 任意 | Docker設定ファイルパスまたはプリセット名。省略時は `docker.conf` または `default` プリセット |
| `--tools`      | 任意 | ツール定義ファイルパスまたはプリセット名（カンマ区切りで複数指定可）。省略時は `tool_list.toml` |
| `--llm`        | 任意 | LLM設定ファイルパスまたはプリセット名。省略時は `llm.conf` またはLLMなし |
| `--skills`     | 任意 | 追加のLLM instructionsファイルパス。省略時は `skills.md` |

#### 自動構成機能 (Auto-discovery)
引数が省略された場合、`<target_dir>` 内から以下のファイルを自動探索する。
- `docker.conf`
- `llm.conf`
- `tool_list.toml`
- `skills.md`

#### 生成物

```
<target_dir>/
├── Dockerfile
├── docker-compose.yml
├── .env                  # 必要な環境変数のキー名のみ（値なし）
├── .renkin_metadata.toml # 起動時に必要なメタデータ
└── workspace/            # ホスト↔エージェント間のデータ共有ディレクトリ
    └── <INSTRUCTIONS>    # 合成されたスキルファイル（GEMINI.md等）
```

---

### 2.3 renkin start

```bash
renkin start
```

1. **環境変数チェック**: 必要な環境変数がホストに設定されているか確認。不足がある場合、警告を表示し続行を確認する
2. `docker compose up -d` でコンテナ群をバックグラウンド起動
3. LLMエージェントコンテナに`docker exec -it`でアタッチ

---

### 2.4 renkin tool

```bash
renkin tool [list]         # プリセット一覧を表示
renkin tool <preset_name>  # 特定のプリセットの詳細（インストール手順等）を表示
```

---

## 3. 設定ファイル・プリセット仕様

### 3.1 ツールプリセット (presets/tools/*.toml)

各ツールにはインストール手順に加え、エージェント向けの「使い方」を記述できる。

```toml
[[tool]]
name = "python-post"
type = "shell"
install = "RUN apt-get install -y python3..."
instructions = """
Python3を使用して後処理を行います。
利用可能なライブラリ: numpy, pandas, foamlib
"""
environment = ["MY_CUSTOM_VAR"]
```

#### スキル合成 (Skill Synthesis)
`renkin assign` 実行時、指定された全ツールの `instructions` が集計され、最終的なスキルファイル（`GEMINI.md` 等）の冒頭に自動挿入される。その後、ユーザー指定の `skills.md` の内容が結合される。

---

## 4. プロキシサポート

ホスト環境に `HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY` 等が設定されている場合、`renkin assign` は以下を自動的に行う。
- Docker build 時の `--build-arg` への追加（`http_proxy`, `https_proxy`, `no_proxy` 等）
- `llm-agent` サービスおよび各 MCP サービスの環境変数への追加

### 4.1 NO_PROXY 設定の注意点

コンテナ内のエージェントがホストOS上で動作しているサービス（例: Forgejo, NATS, データベース等）と通信する場合、その通信が外部プロキシを経由しないように `NO_PROXY` を適切に設定する必要がある。

- **ホストIPの指定**: ホストの物理IPアドレス、または Docker のゲートウェイIP（通常 `172.17.0.1`）を `NO_PROXY` に含めること。
- **推奨設定**: `localhost,127.0.0.1,172.17.0.1,host.docker.internal` およびホストの実際のIPアドレス。
- **影響**: これが設定されていない場合、ローカルサービスへのリクエストが外部プロキシへ転送され、接続タイムアウトや 503 エラーが発生する原因となる。

---

## 5. 動作環境

- **Go**: 1.26.2
- **Docker**: Docker Engine & Docker Compose V2

---

## 6. 管理メタデータ (.renkin_metadata.toml)

`renkin start` 時のチェックに使用される。
- `llm_cmd`: アタッチ時に実行するコマンド
- `env_keys`: ホスト環境で設定されているべき環境変数のリスト
