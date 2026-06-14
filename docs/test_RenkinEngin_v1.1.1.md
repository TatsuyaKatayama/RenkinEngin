# RenkinEngin テスト仕様書

**バージョン**: 1.1.1
**作成日**: 2026-05-17  
**対象**: spec_RenkinEngin_v1.1.1.md

---

## 1. テストレイヤー

| レイヤー | 対象 | Docker | LLM呼び出し | 実行タイミング |
|---------|------|--------|------------|--------------|
| Unit | 設定パース・プリセット解決・生成物文字列検証 | 不要 | なし | PR・main |
| Integration | Docker build・ツールインストール確認・スキル合成・環境変数チェック | DinD | なし | PR・main |

---

## 2. Unit テスト追加項目 (v1.1.0)

### 2.1 プリセット解決

#### TC-U-100: ツールプリセット解決と Instructions 継承
- プリセットから `instructions` フィールドが正しく読み込まれること
- 複数のツールが指定された場合、それぞれの `instructions` が独立して保持されること

#### TC-U-101: 環境変数の集計 (CollectEnvKeys)
- LLM、プロキシ、全ツールから環境変数キーが重複なく集計されること
- `AuthMode = "browser"` の場合にAPIキーが含まれないこと

---

## 3. Integration テスト追加項目 (v1.1.0)

### 3.1 スキル合成の検証

#### TC-I-100: 複数ツールのスキル合成
- `renkin assign` 実行後、`workspace/` 内のスキルファイル（`GEMINI.md` 等）に全ツールの `instructions` が含まれていること
- ユーザー指定の `skills.md` が `## Base Skills` セクションとして末尾に結合されていること

#### TC-I-101: プリセットを使用した assign
- `--tools` にプリセット名（例: `python-post`）を指定して `assign` が成功すること
- プリセットで定義された `install` コマンドが Dockerfile に反映されること

### 3.2 環境変数チェックの検証

#### TC-I-110: renkin start 時の環境変数未設定警告
- 必要な環境変数がホストに設定されていない場合、`renkin start` が警告を表示すること
- ユーザーが `n` を入力した場合にアボートすること

### 3.3 サブコマンドの検証

#### TC-I-120: renkin tool コマンド
- `renkin tool list` で利用可能なプリセット一覧が表示されること
- `renkin tool <name>` で詳細情報が表示されること

### 3.4 無限ループ（連勤）モードの検証

#### TC-I-130: --loop オプションの起動・パース
- `--loop` または `--loop default` 指定時、Preset の `loop_cmd` テンプレートから `{session_id}` が実行ディレクトリ名 + `-session` に置換されて実行されること。
- `--loop work.sh` 指定時、Preset テンプレートを無視して指定スクリプトがコンテナの `/workspace` 起点で直接実行されること。
- `--cmd bash` を併用した場合、`--loop` の指定に割り込み最優先で `bash` が起動アタッチされること。

#### TC-I-131: restart_policy & restart_delay 自動復旧
- ループ実行中にコンテナ内のエージェントプロセスが終了した際、`restart_policy`（`always` / `on-failure`）および `restart_delay` （再起動前の指定秒数待機、未指定時デフォルト 2秒）に従って、自動的に会話セッションが同一セッションIDで自動復元・ループ再起動すること。
- エラー終了時（終了コード非0）に `restart_policy = "on-failure"` が動作すること。

#### TC-I-132: 構造化ログとセッション区切り
- ループが実行されるたび、エージェントログフォルダ（`.renkin/logs/` 等）の `stdout.log` と `stderr.log` に結果が出力されること。
- 毎セッションの開始・終了ごとに、タイムスタンプおよび終了コードを含んだセパレータ（`=== SESSION START/END ===`）がログファイルに正しく追記されること。

---

## 4. 実行環境の更新

- **Go 1.26.2**: 全てのテストはこのバージョンで実施する
- **GitHub Actions**: `go-version` を `1.26.2` に更新

---

## 5. テストマトリクス（v1.1.0 推奨）

| LLM | ツール構成 | 検証項目 |
|-----|-----------|---------|
| gemini | openfoam2512, python-post | 複数ツールのスキル合成、環境変数継承 |
| codex | git, forgejo-mcp | MCPツールの起動と環境変数 |
