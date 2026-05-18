# RenkinEngin テスト仕様書

**バージョン**: 1.1.0
**作成日**: 2026-05-17  
**対象**: spec_RenkinEngin_v1.1.0.md

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
