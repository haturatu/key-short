# 対応環境とビルド

Go 1.26 以降と対象 OS の C コンパイラーを使用します。Fyne と HIDAPI がネイティブ API を使用するため、`GOOS` の変更だけではデスクトップアプリをビルドできません。

| 機能 | Linux | Windows | macOS |
| --- | --- | --- | --- |
| GUI・キー記録・JSON | Fyne / Go | Fyne / Go | Fyne / Go |
| USB 設定 | gousb / libusb | HIDAPI / Windows HID | HIDAPI / IOKit |
| USB ボタン監視 | 対象デバイスの evdev | 対象デバイスの Raw Input | 対象デバイスの IOHIDManager |
| アプリ一覧 | XDG `.desktop` | スタートメニューの `.lnk` | Applications 内の `.app` |
| 前面化 | KWin / X11 | Win32 | AppKit / Accessibility |
| 起動履歴 | PID・開始時刻・ウィンドウ ID | 同左 | 同左 |
| 排他 | flock | LockFileEx | flock |

キー入力からの記録は、フォーカスのある Fyne の記録欄で行います。USB ボタンのバックグラウンド監視とは別の機能です。記録欄を使うためにバックグラウンド入力監視を開始する必要はありません。

Linux の GUI と前面化は実デスクトップで確認しています。Windows/macOS のビルドと自動テストはネイティブランナーで確認します。USB 書き込み、物理ボタンからの起動、OS の権限操作を含む動作は、各 OS の実機での確認が必要です。

## Linux

[README の手順](../README.md#ビルド) に従ってください。hidraw に設定インターフェースが現れない個体への対応を維持するため、Linux は libusb を使用します。udev ルール導入後は USB を抜き差しします。

## Windows

Go と MSYS2 を導入し、MSYS2 の UCRT64 ターミナルでコンパイラーをインストールします。

```sh
pacman -S --needed mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-pkgconf
```

PowerShell でリポジトリのルートへ移動し、ビルドします。MSYS2 の導入先が異なる場合はパスを置き換えてください。

```powershell
$env:PATH = "C:\msys64\ucrt64\bin;$env:PATH"
$env:CGO_ENABLED = "1"
$env:CC = "gcc"
go build -trimpath -o bin/key-short.exe ./cmd/key-short
.\bin\key-short.exe
```

USB は標準 HID でアクセスします。キーボードのドライバーを WinUSB に置き換える手順は不要です。アプリ一覧には、実行ファイルへ解決できるスタートメニューのショートカットを表示します。UWP 専用の起動エントリーや、実行先を取得できないショートカットは対象外です。

コマンド欄は Windows の引数分解規則を使用し、バックスラッシュを保持します。

```text
"C:\Program Files\Chromium\chrome.exe" --remote-debugging-port=9222 --user-data-dir="C:\debug profile"
```

Windows がフォーカス移動を拒否した場合はエラーを表示します。別の権限で起動されたアプリなどが該当します。その代わりにプロセスを重複起動することはありません。

## macOS

Go と Xcode Command Line Tools を導入します。HIDAPI は IOKit / CoreFoundation を使用するため、libusb は不要です。

```sh
xcode-select --install
go build -trimpath -o bin/key-short ./cmd/key-short
./bin/key-short
```

システム設定の「プライバシーとセキュリティ」で、実行する Key Short または起動元のターミナルに以下の権限を付与します。

- **入力監視**: USB ボタンのバックグラウンド監視。
- **アクセシビリティ**: 起動済みウィンドウの最小化解除と前面化。

権限を変更した場合は監視やアプリを開始し直してください。アプリ一覧は `/Applications`、`/System/Applications`、`~/Applications` から `.app` を読み込みます。前面化は記録したウィンドウの所有アプリに対して行います。複数のウィンドウがある場合は、そのアプリのウィンドウを最小化解除して前面へ移動します。

```text
"/Applications/Chromium.app/Contents/MacOS/Chromium" --remote-debugging-port=9222 --user-data-dir="/tmp/key-short-debug"
```

署名、公証、配布用 `.app` バンドルの生成は今回の CI の対象外です。ソースからのネイティブビルドを検証します。

## 設定と履歴

Linux の履歴は `${XDG_STATE_HOME:-~/.local/state}/key-short/`、Windows は通常 `%AppData%\key-short`、macOS は `~/Library/Application Support/key-short/` に保存します。

JSON のキー割り当ては共通です。アプリのパスと `.desktop` / `.lnk` / `.app` は OS 固有のため、移動先の OS でアプリを選び直してください。
