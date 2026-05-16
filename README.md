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
```bash
go build -o renkin ./cmd/renkin
export PATH=$PATH:$(pwd)
```

### 2. 連勤の業務命令 (環境の構成)
ターゲットディレクトリを指定して実行します。各オプションは省略すると、ディレクトリ内の設定ファイルや標準プリセットが自動適用されます。
```bash
# 例: Gemini と Python解析環境を組み合わせて錬成
renkin assign ./ --llm gemini --tools python-post
```

### 3. 労働開始 (環境起動 & アタッチ)
```bash
renkin start
```
エージェントがコンテナ内で起動し、自律的な労働を開始します。

## 🧪 標準プリセット

### LLM エージェント
- `gemini`: Gemini CLI (Node.js v24)
- `codex`: Codex CLI (Node.js v24)

### 解析ツール
- `openfoam2512`: 流体解析（Ubuntu 24.04 対応）
- `openmodelica410`: 物理モデリング（MSL v4.1.0 搭載）
- `python-post`: 高速解析環境（foamlib, DyMat, Optuna, pandas 等）

## ⚠️ 運用上の注意
- **APIコスト**: エージェントの自律稼働（連勤）に伴う API コストは、管理者の負担となります。
- **Proxy環境**: 企業のファイアウォール下でも、ホストの proxy 設定を自動継承して環境構築が可能です。

---

## 📜 ライセンス
MIT License.

---
**RenkinEngin で、シミュレーション業務の完全自律化を実現しましょう。**
