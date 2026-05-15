# RenkinEngin 仕様書

**バージョン**: 0.1  
**作成日**: 2026-05-15

---

## 1. 概要

RenkinEnginは、異なるLLMエージェント（Claude Code, Gemini CLI, Codex等）と各種ツール（OpenFOAM, OpenModelica, LightRAG等）を組み合わせたDockerコンテナ環境を、設定ファイルベースで簡単に構築・起動するCLIツールである。連勤エージェントを錬金するエンジンが名前の由来。

### 設計思想

- **設定ファイルによる宣言的構成**: Dockerfile等を手書きせず、設定ファイルの組み合わせでコンテナを定義する
- **LLMとツールの同居**: LLMエージェントとshellツールは同一コンテナ内に配置する
- **MCPツールは別コンテナ**: MCP serverを持つツールはdocker compose の別serviceとして起動する
- **workspaceによるデータ共有**: ホストとコンテナエージェント間のデータやり取りはworkspaceディレクトリを通じて行う
- **環境変数継承**: API KeyはホストOS側で管理し、コンテナに継承させる（設定ファイルには記載しない）

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

---

### 2.2 renkin assign

```bash
renkin assign <target_dir> \
  --docker docker.conf \
  --tools tool_list.toml \
  [--llm llm.conf] \
  [--skills skills.md]
```

#### 引数・オプション

| 引数/オプション | 必須 | 説明 |
|----------------|------|------|
| `<target_dir>` | 必須 | 生成物の出力先ディレクトリ（通常 `./`） |
| `--docker`     | 必須 | Dockerインフラ設定ファイルパス |
| `--tools`      | 必須 | ツール定義ファイルパス |
| `--llm`        | 任意 | LLM設定ファイルパス（省略時はLLMなし） |
| `--skills`     | 任意 | LLMへのinstructionsファイルパス（省略時はskillsなし） |

#### 生成物

```
<target_dir>/
├── Dockerfile
├── docker-compose.yml
├── .env                  # 必要な環境変数のキー名のみ（値なし）
└── workspace/            # ホスト↔エージェント間のデータ共有ディレクトリ
    └── <INSTRUCTIONS>    # --skills指定時のみ生成（LLMに応じてリネーム）
```

---

### 2.3 renkin end

```bash
cd <target_dir>
renkin end
```

カレントディレクトリの`docker-compose.yml`を使い、以下を実行する。

1. `docker compose down` でコンテナ群を停止・削除する

---

### 2.4 renkin start

```bash
cd <target_dir>
renkin start
```

カレントディレクトリの`docker-compose.yml`を使い、以下を実行する。

1. `docker compose up -d` でコンテナ群をバックグラウンド起動
2. ホスト環境変数を`.env`のキー名に基づきコンテナに継承
3. LLMエージェントコンテナに`docker exec -it`でアタッチ（インタラクティブTTY）

LLMが未設定（`--llm`省略）の場合はステップ3をスキップし、コンテナ起動のみ行う。

---

## 3. 設定ファイル仕様

### 3.1 docker.conf

Dockerインフラ全般の設定。ベースイメージとマウント定義を記述する。

```toml
# docker.conf

base_image = "ubuntu:24.04"   # 省略時デフォルト: ubuntu:24.04

[[mount]]
host = "./workspace"
container = "/workspace"

[[mount]]
host = "./data"
container = "/data"
```

#### フィールド定義

| フィールド | 型 | 必須 | 説明 |
|-----------|-----|------|------|
| `base_image` | string | 任意 | LLMコンテナのベースイメージ。省略時は`ubuntu:24.04` |
| `mount[].host` | string | 必須 | ホスト側のパス（相対パス可） |
| `mount[].container` | string | 必須 | コンテナ内のマウント先パス |

---

### 3.2 llm.conf

LLMエージェントの種別・インストール方法・認証方式を記述する。

```toml
# llm.conf

cmd = "claude --dangerously-skip-permissions"   # オプション付き起動コマンド
auth_mode = "browser"   # "browser" または "api_key"（省略時: "api_key"）

install = """
RUN curl -fsSL https://claude.ai/install.sh | sh
"""
```

#### フィールド定義

| フィールド | 型 | 必須 | 説明 |
|-----------|-----|------|------|
| `cmd` | string | 必須 | LLMの起動コマンド（オプション含む）。先頭のコマンド名からLLM種別を自動判定する |
| `auth_mode` | string | 任意 | 認証方式。`"browser"` または `"api_key"`。省略時は `"api_key"` |
| `install` | string | 任意 | Dockerfileに挿入するインストールスニペット。省略時はLLMをインストールしない |

#### 対応LLM（cmdの先頭コマンド名で判定）

| cmd先頭 | instructionsファイル | api_key時の環境変数 | browser認証対応 |
|---------|--------------------|--------------------|---------------|
| `claude` | `CLAUDE.md` | `ANTHROPIC_API_KEY` | ○ |
| `gemini` | `GEMINI.md` | `GEMINI_API_KEY` | ○ |
| `codex`  | `AGENTS.md` | `OPENAI_API_KEY` | ○ |
| `opencode` | `AGENTS.md` | プロバイダー依存（複数可） | プロバイダー依存 |

#### 認証方式の詳細

**api_key（デフォルト）**

ホストOSの環境変数に設定したAPIキーをコンテナに継承する。`.env`ファイルにキー名を自動生成する。

**browser**

OAuth 2.0ブラウザ認証を使用する。`renkin start`実行時に各LLMのログインコマンドを実行し、ホスト側ブラウザで認証を行う。認証後に得たトークンをコンテナ内にマウントして引き継ぐ。

| cmd先頭 | ブラウザ認証の仕組み | トークン保存場所 |
|---------|-------------------|----------------|
| `claude` | `claude /login` でOAuth → ホスト側`~/.claude/.credentials.json`に保存 | `~/.claude/` をコンテナにマウント |
| `gemini` | `gemini auth login` でOAuth | `~/.config/gemini/` をコンテナにマウント |
| `codex`  | `codex auth login` でOAuth | `~/.codex/` をコンテナにマウント |
| `opencode` | プロバイダーごとのOAuth | プロバイダー依存 |

**コンテナ内でのブラウザ認証について**

コンテナはヘッドレス環境のためブラウザが使えない。ブラウザ認証は**ホスト側で事前に実施**し、生成されたトークンファイルをコンテナにマウントして引き継ぐ方式を採用する。`renkin start`は`auth_mode = "browser"`の場合、ホスト側のトークンファイルの存在確認を行い、未認証の場合はログインコマンドの実行を促す。

**opencodeについての注意**: opencodeはマルチプロバイダー対応のため、必要な環境変数はプロバイダー設定に依存する。`ANTHROPIC_API_KEY`・`OPENAI_API_KEY`・`GEMINI_API_KEY`等、使用するプロバイダーのキーをホスト環境変数として設定する。

---

### 3.3 tool_list.toml

コンテナにインストールするツールを定義する。shellツールとMCPツールの2種類がある。

```toml
# tool_list.toml

# shellツール（LLMと同一コンテナにインストール）
[[tool]]
name = "openfoam"
type = "shell"
install = """
RUN apt-get update && apt-get install -y openfoam2412
"""

[[tool]]
name = "openmodelica"
type = "shell"
install = """
RUN apt-get update && apt-get install -y openmodelica
"""

# MCPツール（docker composeの別serviceとして起動）
[[tool]]
name = "lightrag"
type = "mcp"
image = "lightrag/server:latest"
port = 8080
```

#### フィールド定義（共通）

| フィールド | 型 | 必須 | 説明 |
|-----------|-----|------|------|
| `name` | string | 必須 | ツール名（識別用） |
| `type` | string | 必須 | `"shell"` または `"mcp"` |

#### フィールド定義（type = "shell"）

| フィールド | 型 | 必須 | 説明 |
|-----------|-----|------|------|
| `install` | string | 必須 | Dockerfileに挿入するインストールスニペット |

#### フィールド定義（type = "mcp"）

| フィールド | 型 | 必須 | 説明 |
|-----------|-----|------|------|
| `image` | string | 必須 | MCPサーバーのDockerイメージ |
| `port` | integer | 必須 | MCPサーバーの公開ポート |

---

### 3.4 skills.md

LLMエージェントへのinstructions（システムプロンプト相当）を記述するMarkdownファイル。`renkin assign`の`--skills`引数で指定する。

`renkin assign`はこのファイルをLLMの種別に応じてリネームし、`workspace/`以下に配置する。LLM種別はllm.confの`cmd`の先頭コマンド名で判定する。

| cmd先頭 | リネーム後のファイル名 | コンテナ内パス |
|---------|--------------------|--------------| 
| `claude` | `CLAUDE.md` | `/workspace/CLAUDE.md` |
| `gemini` | `GEMINI.md` | `/workspace/GEMINI.md` |
| `codex` | `AGENTS.md` | `/workspace/AGENTS.md` |
| `opencode` | `AGENTS.md` | `/workspace/AGENTS.md` |

---

## 4. 生成物仕様

### 4.1 Dockerfile

docker.conf・llm.conf・tool_list.tomlの内容を合成して生成する。

```dockerfile
FROM <docker.confのbase_image>

# llm.confのinstallスニペット（--llm指定時のみ）
RUN curl -fsSL https://claude.ai/install.sh | sh

# tool_list.tomlのshellツールをtool定義順に展開
RUN apt-get update && apt-get install -y openfoam2412
RUN apt-get update && apt-get install -y openmodelica

WORKDIR /workspace
```

### 4.2 docker-compose.yml

```yaml
services:
  llm-agent:
    build: .
    stdin_open: true
    tty: true
    env_file: .env
    volumes:
      - ./workspace:/workspace   # docker.confのmount定義から生成
      - ./data:/data

  lightrag:                      # type=mcpのツールは別service
    image: lightrag/server:latest
    ports:
      - "8080:8080"
```

### 4.3 .env

コンテナに渡す環境変数のキー名のみを列挙する。値はホストOSの環境変数から継承する。

```
# .env (値なし・キー名のみ)
ANTHROPIC_API_KEY=
GEMINI_API_KEY=
OPENAI_API_KEY=
```

どのキーを列挙するかはllm.confの`cmd`の先頭コマンド名から自動判定する。

| cmd先頭 | 列挙する環境変数キー |
|---------|-------------------|
| `claude` | `ANTHROPIC_API_KEY` |
| `gemini` | `GEMINI_API_KEY` |
| `codex`  | `OPENAI_API_KEY` |
| `opencode` | `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GEMINI_API_KEY`（全プロバイダー分） |

---

## 5. ツール接続方式

LLMエージェントがツールを利用する際の接続方式は以下の通り。

| 接続方式 | 条件 | 説明 |
|---------|------|------|
| Shell tools | `type = "shell"` | LLMのshell実行機能経由でCLIコマンドを直接呼び出す |
| MCP server  | `type = "mcp"`  | LLMのMCPクライアント経由でMCPサーバーに接続する |

MCPツールはdocker compose内の別serviceとして起動し、LLMエージェントコンテナからネットワーク経由でアクセスする。

---

## 6. ディレクトリ構成とワークスペース

### ホスト側

```
<target_dir>/
├── docker.conf
├── llm.conf
├── tool_list.toml
├── skills.md
├── Dockerfile             ← renkin assign が生成
├── docker-compose.yml     ← renkin assign が生成
├── .env                   ← renkin assign が生成
└── workspace/             ← renkin assign が生成
    └── CLAUDE.md          ← skills.mdをリネーム（LLM種別に応じる）
```

### コンテナ内

```
/workspace/                ← ホストの./workspaceをマウント
└── CLAUDE.md
```

ユーザーとコンテナ内エージェント間のデータ授受は`workspace/`ディレクトリを通じて行う。

---

## 7. API Key・環境変数管理

### api_key認証（デフォルト）

- API KeyはホストOSの環境変数として管理する
- 設定ファイル（llm.conf等）には一切記載しない
- `renkin assign`は`.env`ファイルにキー名のみを生成する
- `renkin start`は`docker compose up`時に`--env-file .env`でホスト環境変数を継承させる

```bash
# ホストで事前に設定
export ANTHROPIC_API_KEY=sk-ant-...

# start時に自動継承
renkin start
```

### browser認証（auth_mode = "browser"）

- ホスト側で事前にOAuth認証を実施し、生成されたトークンファイルをコンテナにマウントして引き継ぐ
- `renkin start`実行時にトークンファイルの存在を確認し、未認証の場合は認証手順を表示する
- `.env`へのAPIキー列挙はスキップされる

```bash
# ホスト側で事前に認証（初回のみ）
claude /login   # ブラウザが開き~/.claude/.credentials.jsonに保存される

# start時にトークンファイルを自動マウント
renkin start
```

---

## 付録A: 最小構成の例

OpenFOAM + Claude Codeの構成例。

```toml
# docker.conf
base_image = "ubuntu:24.04"

[[mount]]
host = "./workspace"
container = "/workspace"
```

```toml
# llm.conf
cmd = "claude --dangerously-skip-permissions"
install = """
RUN curl -fsSL https://claude.ai/install.sh | sh
"""
```

```toml
# tool_list.toml
[[tool]]
name = "openfoam"
type = "shell"
install = """
RUN apt-get update && apt-get install -y openfoam2412
"""
```

```bash
# セットアップ
renkin assign ./ --docker docker.conf --llm llm.conf --tools tool_list.toml --skills skills.md

# 起動
renkin start

# 停止
renkin end
```

---

## 付録C: LLMなし構成の例

ツールコンテナのみ（LLMなし）の構成例。ユーザーが直接コンテナ内のツールを利用する場合。

```toml
# docker.conf
base_image = "ubuntu:24.04"

[[mount]]
host = "./workspace"
container = "/workspace"
```

```toml
# tool_list.toml
[[tool]]
name = "openfoam"
type = "shell"
install = """
RUN apt-get update && apt-get install -y openfoam2412
"""
```

```bash
# セットアップ（--llm・--skills省略）
renkin assign ./ --docker docker.conf --tools tool_list.toml

# 起動（LLMアタッチなし・コンテナ起動のみ）
renkin start

# 停止
renkin end
```

Claude Code + OpenFOAM + OpenModelica + LightRAG(MCP)の構成例。

```toml
# tool_list.toml
[[tool]]
name = "openfoam"
type = "shell"
install = """
RUN apt-get update && apt-get install -y openfoam2412
"""

[[tool]]
name = "openmodelica"
type = "shell"
install = """
RUN apt-get update && apt-get install -y openmodelica
"""

[[tool]]
name = "lightrag"
type = "mcp"
image = "lightrag/server:latest"
port = 8080
```
