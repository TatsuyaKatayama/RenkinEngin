# RenkinEngin テスト仕様書

**バージョン**: 0.1
**作成日**: 2026-05-14  
**対象**: RenkinEngin_spec.md v0.1-draft

---

## 1. テスト方針

### 1.1 基本方針

- LLMエージェントの実際の応答はテストスコープ外（APIコスト・非決定性のため）
- ツールのインストール確認はツールごとに個別の確認コマンドを定義する
- MCPの登録確認はlistコマンドのみ
- LLMの起動確認はhelpコマンド（LLM非呼び出し）のみ
- Docker操作はDocker-in-Docker（DinD）でCI上でも実施する

### 1.2 テストレイヤー

| レイヤー | 対象 | Docker | LLM呼び出し | 実行タイミング |
|---------|------|--------|------------|--------------|
| Unit | 設定パース・生成物文字列検証 | 不要 | なし | PR・main |
| Integration | Docker build・ツールインストール確認 | DinD | なし | PR・main |
| E2E | コンテナ起動・MCP登録・LLM help確認 | DinD | なし | main・nightly |

### 1.3 テストマトリクス

PRおよびmain mergeでは代表的な組み合わせのみ実行し、nightlyでフルマトリクスを実行する。

**代表組み合わせ（PR/main）**

| LLM | ツール（shell） | ツール（MCP） |
|-----|--------------|-------------|
| claude | openfoam | lightrag |
| opencode | openmodelica | lightrag |

**フルマトリクス（nightly）**

| LLM | ツール（shell） | ツール（MCP） |
|-----|--------------|-------------|
| claude | openfoam, openmodelica, python | lightrag |
| gemini | openfoam, openmodelica, python | lightrag |
| codex | openfoam, openmodelica, python | lightrag |
| opencode | openfoam, openmodelica, python | lightrag |

---

## 2. Unit テスト

### 2.1 設定ファイルパース

#### TC-U-001: llm.conf パース（正常系）

```go
func TestLLMConfParse(t *testing.T) {
    input := `
cmd = "claude --dangerously-skip-permissions"
auth_mode = "api_key"
install = """
RUN curl -fsSL https://claude.ai/install.sh | sh
"""
`
    conf, err := ParseLLMConf(input)
    assert.NoError(t, err)
    assert.Equal(t, "claude --dangerously-skip-permissions", conf.Cmd)
    assert.Equal(t, "api_key", conf.AuthMode)
    assert.Contains(t, conf.Install, "curl -fsSL")
}
```

**確認項目**
- cmd・auth_mode・installが正しくパースされること
- auth_mode省略時のデフォルトが`api_key`になること
- install省略時にnilまたは空文字になること

#### TC-U-002: llm.conf パース（異常系）

| ケース | 入力 | 期待結果 |
|-------|------|---------|
| cmdなし | auth_modeのみ | エラー返却 |
| 不正なauth_mode | `auth_mode = "ssh"` | エラー返却 |
| TOMLシンタックスエラー | `cmd = ` | パースエラー |

#### TC-U-003: tool_list.toml パース（正常系）

```go
func TestToolListParse(t *testing.T) {
    input := `
[[tool]]
name = "openfoam"
type = "shell"
install = "RUN apt-get install -y openfoam2412"

[[tool]]
name = "lightrag"
type = "mcp"
image = "lightrag/server:latest"
port = 8080
`
    tools, err := ParseToolList(input)
    assert.NoError(t, err)
    assert.Len(t, tools, 2)
    assert.Equal(t, "shell", tools[0].Type)
    assert.Equal(t, "mcp", tools[1].Type)
    assert.Equal(t, 8080, tools[1].Port)
}
```

**確認項目**
- shellツール・MCPツールが正しくパースされること
- type=shellでinstallなしの場合エラーになること
- type=mcpでimageまたはportなしの場合エラーになること

#### TC-U-004: docker.conf パース（正常系）

**確認項目**
- base_image省略時のデフォルトが`ubuntu:24.04`になること
- mountのhost・containerが正しくパースされること
- mountが複数定義できること

---

### 2.2 Dockerfile生成

#### TC-U-010: Dockerfile生成（LLMあり・shellツールあり）

```go
func TestDockerfileGeneration(t *testing.T) {
    cfg := Config{
        Docker: DockerConf{BaseImage: "ubuntu:24.04"},
        LLM:    LLMConf{Cmd: "claude", Install: "RUN curl -fsSL https://claude.ai/install.sh | sh"},
        Tools: []Tool{
            {Name: "openfoam", Type: "shell", Install: "RUN apt-get install -y openfoam2412"},
        },
    }
    dockerfile, err := GenerateDockerfile(cfg)
    assert.NoError(t, err)
    assert.Contains(t, dockerfile, "FROM ubuntu:24.04")
    assert.Contains(t, dockerfile, "RUN curl -fsSL https://claude.ai/install.sh | sh")
    assert.Contains(t, dockerfile, "RUN apt-get install -y openfoam2412")
    assert.Contains(t, dockerfile, "WORKDIR /workspace")
}
```

**確認項目**

| 確認内容 | 期待値 |
|---------|-------|
| FROMの内容 | docker.confのbase_image |
| LLMスニペットの位置 | shellツールスニペットより前 |
| shellツールスニペットの順序 | tool_list.tomlの定義順 |
| WORKDIRの位置 | 最終行付近 |
| MCPツールのinstallが含まれないこと | Dockerfileに`lightrag`が出ないこと |

#### TC-U-011: Dockerfile生成（LLMなし）

**確認項目**
- LLMのinstallスニペットが出力されないこと
- toolsのみでDockerfileが生成されること

#### TC-U-012: Dockerfile生成（toolsなし）

**確認項目**
- FROMとWORKDIRのみのDockerfileが生成されること

---

### 2.3 docker-compose.yml生成

#### TC-U-020: compose生成（shellツール＋MCPツール）

**確認項目**

| 確認内容 | 期待値 |
|---------|-------|
| llm-agentサービスの存在 | あり |
| MCPツールの別サービス化 | `lightrag`サービスが独立していること |
| volumes定義 | docker.confのmount定義と一致すること |
| stdin_open / tty | true |
| env_file | `.env`を参照していること |
| MCPサービスのports | tool_list.tomlのportと一致すること |

#### TC-U-021: compose生成（MCPツールなし）

**確認項目**
- llm-agentサービスのみ生成されること

---

### 2.4 .env生成

#### TC-U-030: .env生成（claude・api_key）

**確認項目**
- `ANTHROPIC_API_KEY=`が含まれること
- 値が空であること（キー名のみ）

#### TC-U-031: .env生成（opencode・api_key）

**確認項目**
- `ANTHROPIC_API_KEY=`・`OPENAI_API_KEY=`・`GEMINI_API_KEY=`が含まれること

#### TC-U-032: .env生成（auth_mode = browser）

**確認項目**
- .envにAPIキーが含まれないこと（空ファイルまたはコメントのみ）

---

### 2.5 skillsファイルのリネーム

#### TC-U-040: skillsリネーム

| cmd先頭 | 期待するファイル名 |
|---------|----------------|
| claude  | CLAUDE.md |
| gemini  | GEMINI.md |
| codex   | AGENTS.md |
| opencode | AGENTS.md |

---

### 2.6 LLM種別判定

#### TC-U-050: cmd先頭からのLLM種別判定

| cmd | 期待する種別 |
|-----|-----------|
| `claude --dangerously-skip-permissions` | claude |
| `gemini` | gemini |
| `codex -c` | codex |
| `opencode` | opencode |
| `unknown-llm` | エラー |

---

## 3. Integration テスト

Docker buildまでを確認する。DinD環境で実行。

### 3.1 Docker build成功確認

#### TC-I-001: 最小構成でbuildが通ること

```
fixtures/minimal/
├── docker.conf     (base_image = "ubuntu:24.04")
├── llm.conf        (cmd = "echo hello", install = "RUN echo installed")
└── tool_list.toml  (toolなし)
```

**手順**
1. `renkin assign`で生成物を作成
2. `docker build`を実行
3. exit code 0を確認

#### TC-I-002: shellツールのインストール確認

各ツールについて、buildしたイメージ上でインストール確認コマンドを実行する。

| ツール | 確認コマンド | 期待する結果 |
|-------|------------|------------|
| openfoam | `openfoam2412 -help` | exit 0、helpテキスト出力 |
| openmodelica | `omc --version` | exit 0、バージョン文字列出力 |
| python | `python3 --version` | exit 0、`Python 3.x.x`出力 |

```go
func TestToolInstallation(t *testing.T) {
    tests := []struct {
        tool    string
        command []string
        fixture string
    }{
        {"openfoam", []string{"openfoam2412", "-help"}, "fixtures/openfoam"},
        {"openmodelica", []string{"omc", "--version"}, "fixtures/openmodelica"},
        {"python", []string{"python3", "--version"}, "fixtures/python"},
    }

    for _, tt := range tests {
        t.Run(tt.tool, func(t *testing.T) {
            imageID := buildImage(t, tt.fixture)
            output, exitCode := runInContainer(t, imageID, tt.command)
            assert.Equal(t, 0, exitCode)
            assert.NotEmpty(t, output)
        })
    }
}
```

#### TC-I-003: LLMのインストール確認

| LLM | 確認コマンド | 期待する結果 |
|-----|------------|------------|
| claude | `claude --version` | exit 0、バージョン文字列出力 |
| gemini | `gemini --version` | exit 0、バージョン文字列出力 |
| codex | `codex --version` | exit 0、バージョン文字列出力 |
| opencode | `opencode --version` | exit 0、バージョン文字列出力 |

---

### 3.2 workspace生成確認

#### TC-I-010: workspaceディレクトリの生成

**確認項目**
- `renkin assign`後に`workspace/`が生成されること
- `--skills`指定時にworkspace/以下にリネームされたファイルが存在すること
- `--skills`省略時にworkspace/以下にinstructionsファイルが存在しないこと

---

### 3.3 生成物の整合性確認

#### TC-I-020: compose.ymlのvolumes定義とhost側ディレクトリの整合

**確認項目**
- docker.confに定義されたmountのhost側パスが生成後に存在すること
- `docker compose config`でバリデーションエラーが出ないこと

---

## 4. E2E テスト

コンテナを実際に起動し、動作を確認する。DinD環境で実行。

### 4.1 renkin start確認

#### TC-E-001: startでコンテナが起動すること

**手順**
1. `renkin assign`で生成
2. `renkin start`を非インタラクティブモードで実行（LLM未設定fixture使用）
3. `docker compose ps`でコンテナがrunning状態であることを確認

#### TC-E-002: renkin endでコンテナが停止すること

**手順**
1. TC-E-001の続き
2. `renkin end`を実行
3. `docker compose ps`でコンテナが存在しないことを確認

---

### 4.2 MCPサービス確認

#### TC-E-010: MCPサービスの起動確認

**手順**
1. lightragを含むfixture構成でstart
2. コンテナ内からlightragのエンドポイントに疎通確認

```go
func TestMCPServiceUp(t *testing.T) {
    // lightragのHTTPエンドポイントにGET
    resp, err := http.Get("http://localhost:8080/health")
    assert.NoError(t, err)
    assert.Equal(t, 200, resp.StatusCode)
}
```

#### TC-E-011: LLMコンテナからMCPサービスへの疎通確認

**手順**
1. llm-agentコンテナ内から`curl lightrag:8080/health`を実行
2. 200レスポンスを確認

#### TC-E-012: MCP登録確認（claude）

**手順**
1. llm-agentコンテナ内で`claude mcp list`を実行
2. lightragがリストに含まれることを確認

```go
func TestMCPRegistration(t *testing.T) {
    output := execInContainer(t, "llm-agent", []string{"claude", "mcp", "list"})
    assert.Contains(t, output, "lightrag")
}
```

| LLM | MCP確認コマンド |
|-----|--------------|
| claude | `claude mcp list` |
| gemini | `gemini mcp list` |
| codex | `codex mcp list` |
| opencode | `opencode mcp list` |

---

### 4.3 LLM起動確認

LLMを実際に呼び出さず、helpコマンドで起動バイナリの動作のみ確認する。

#### TC-E-020: LLM helpコマンド確認

| LLM | 確認コマンド | 期待する結果 |
|-----|------------|------------|
| claude | `claude --help` | exit 0、usageテキスト出力 |
| gemini | `gemini --help` | exit 0、usageテキスト出力 |
| codex | `codex --help` | exit 0、usageテキスト出力 |
| opencode | `opencode --help` | exit 0、usageテキスト出力 |

---

### 4.4 workspaceのデータ共有確認

#### TC-E-030: ホスト→コンテナのファイル共有

**手順**
1. ホスト側の`workspace/test.txt`にテキストを書き込む
2. コンテナ内で`cat /workspace/test.txt`を実行
3. 同じ内容が読めることを確認

#### TC-E-031: コンテナ→ホストのファイル共有

**手順**
1. コンテナ内で`echo result > /workspace/output.txt`を実行
2. ホスト側で`workspace/output.txt`が存在し内容が一致することを確認

---

## 5. CI パイプライン仕様

### 5.1 パイプライン構成

```yaml
# .github/workflows/ci.yml

name: CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  unit:
    name: Unit Tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - run: go test ./tests/unit/... -v

  integration:
    name: Integration Tests
    runs-on: ubuntu-latest
    services:
      docker:
        image: docker:dind
        options: --privileged
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3
      - run: go test ./tests/integration/... -v -timeout 20m

  e2e:
    name: E2E Tests
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    services:
      docker:
        image: docker:dind
        options: --privileged
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - run: go test ./tests/e2e/... -v -timeout 30m
```

---

### 5.2 nightly パイプライン（フルマトリクス）

```yaml
# .github/workflows/nightly.yml

name: Nightly Full Matrix

on:
  schedule:
    - cron: "0 1 * * *"   # 毎日 01:00 UTC

jobs:
  matrix:
    name: "${{ matrix.llm }} x ${{ matrix.tool }}"
    runs-on: ubuntu-latest
    services:
      docker:
        image: docker:dind
        options: --privileged
    strategy:
      fail-fast: false
      matrix:
        llm: [claude, gemini, codex, opencode]
        tool: [openfoam, openmodelica, python, lightrag]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - run: |
          go test ./tests/integration/... \
            -v \
            -timeout 30m \
            -run "TestToolInstallation/${{ matrix.tool }}" \
            -llm "${{ matrix.llm }}"
```

---

### 5.3 実行タイミングまとめ

| テスト | PR | main merge | nightly |
|-------|-----|-----------|---------|
| Unit | ✓ | ✓ | ✓ |
| Integration（代表） | ✓ | ✓ | - |
| Integration（フルマトリクス） | - | - | ✓ |
| E2E（代表） | - | ✓ | - |
| E2E（フルマトリクス） | - | - | ✓ |

---

## 6. テストフィクスチャ

```
tests/fixtures/
├── minimal/
│   ├── docker.conf
│   ├── llm.conf         # install = "RUN echo noop"
│   └── tool_list.toml   # ツールなし
├── openfoam/
│   ├── docker.conf
│   └── tool_list.toml   # openfoamのみ
├── openmodelica/
│   ├── docker.conf
│   └── tool_list.toml
├── python/
│   ├── docker.conf
│   └── tool_list.toml
├── lightrag/
│   ├── docker.conf
│   └── tool_list.toml   # lightrag MCP
├── claude-openfoam/
│   ├── docker.conf
│   ├── llm.conf         # cmd = "claude --help"
│   └── tool_list.toml
└── opencode-lightrag/
    ├── docker.conf
    ├── llm.conf         # cmd = "opencode --help"
    └── tool_list.toml
```

---

## 7. ツール確認コマンド定義

各ツールの確認コマンドは以下の通り定義する。新しいツールを追加する際はこのテーブルに追記する。

| ツール | type | 確認コマンド | 成功条件 |
|-------|------|------------|---------|
| icoFoam | shell | `icoFoam -help` | exit 0 |
| openmodelica | shell | `omc --version` | exit 0 + バージョン文字列 |
| python | shell | `python3 --version` | exit 0 + `Python 3` |
| lightrag | mcp | `curl -s http://lightrag:8080/health` | HTTP 200 |
| claude | llm | `claude --version` | exit 0 |
| gemini | llm | `gemini --version` | exit 0 |
| codex | llm | `codex --version` | exit 0 |
| opencode | llm | `opencode --version` | exit 0 |

---

## 8. 未対応・今後の課題

- browser認証（auth_mode = "browser"）のE2Eテストは手動確認（CI上でブラウザ認証不可のため）
- ツール確認コマンドはツール追加のたびにテーブルへの追記が必要
