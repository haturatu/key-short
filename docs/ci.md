# CI と依存関係更新

Fyne、USB、ウィンドウ制御はネイティブ API を使用するため、OS 共通のロジックテストに加えて、各 OS のコンパイラーと SDK を使ったビルドを最低限の CI に含めています。

| チェック | 対象 | 目的 |
| --- | --- | --- |
| `gofmt` | Linux | 整形差分を検出 |
| `go mod verify` | 3 OS | モジュールの整合性を検証 |
| `go mod tidy` 後の差分 | Linux | 依存定義とチェックサムの不足を検出 |
| `go vet -tags ci ./...` | 3 OS | OS 固有コードも含む静的検査 |
| `go test -tags ci ./...` | 3 OS | ロジックと GUI 操作を検証 |
| `go build` | 3 OS | CGO とネイティブ SDK で実行ファイルをビルド |
| `go test -race -tags ci ./...` | Linux | 入力監視や履歴更新の競合を検出 |
| `go tool govulncheck -tags ci ./...` | Linux | 到達可能な既知の Go 脆弱性を検出 |

`ci` タグは Fyne のテスト用ソフトウェアドライバーを選びます。製品ビルドではこのタグを付けず、実際のデスクトップドライバーと USB API をコンパイルします。USB デバイスや画面のないランナーでもテストでき、製品ビルドのコンパイル漏れも確認できます。

ランナーは `ubuntu-24.04`、`windows-2025`、`macos-15` です。Go は `actions/setup-go` の `stable`、Windows は MSYS2 UCRT64 GCC、macOS は Xcode の clang を使用します。

USB 実機、OS の許可操作、実デスクトップの前面化テストは通常の CI から除外します。Linux の実デスクトップ統合テストは README のオプトイン方式を維持しています。毎回の自動配布・成果物アップロードは含めていません。

## トリガーと権限

作業ブランチへの push、pull request、手動実行に加え、毎週金曜 09:00（Asia/Tokyo）にも実行します。ジョブは `contents: read` とし、秘密情報を使用しません。Actions は完全なコミット SHA に固定します。ジョブには30分の制限を設け、同じ参照の古い実行は新しい実行でキャンセルします。

## Dependabot

Go Modules と GitHub Actions の更新を **毎週金曜 09:00、Asia/Tokyo** に確認します。Go の minor / patch 更新をまとめ、Actions の更新もまとめます。同時 PR 数は各エコシステム5件まで、コミットのプレフィックスは `chore` です。

govulncheck も `go.mod` の `tool` として管理するため更新対象になります。自動マージは行わず、CI の結果を確認して適用します。設定はデフォルトブランチへ置くことで有効になります。指定時刻は更新確認の予定であり、GitHub の混雑などで実際の開始が遅れる場合があります。

## 調査した公式資料

- [Dependabot のオプション](https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference)
- [GitHub Actions の安全な利用](https://docs.github.com/en/actions/reference/security/secure-use)
- [Fyne のビルド要件](https://docs.fyne.io/started/)
- [Windows Raw Input](https://learn.microsoft.com/en-us/windows/win32/inputdev/raw-input)
- [Apple の HID アクセス API](https://github.com/apple-oss-distributions/IOKitUser/blob/main/hidsystem.subproj/IOHIDLib.h)
