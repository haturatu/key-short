# Key Short

USB ID `1189:8890` の MINI KeyBoard を設定する Linux 用デスクトップアプリです。Go と Fyne で実装し、libusb で設定データを送信します。12キーと2つのノブに対応しています。

![Key Short のキー・ノブ設定画面](docs/screenshot.png)

## 機能

- キーと、各ノブの左回転・押下・右回転への割り当て。
- Ctrl、Shift、Alt、Super などを組み合わせたショートカット。
- 最大5操作の連続入力、F1〜F24、日本語キーボードの一部のキー。
- 音量、ミュート、再生などのメディア操作。
- マウスクリック、ホイール、Ctrl とホイールの組み合わせ。
- 3レイヤーの設定、LED モード0〜2の指定。
- JSON の保存・読み込み、送信するバイト列の確認。
- CLI によるデバイス検出、送信データの確認、設定の書き込み。
- インストール済みアプリの選択、引数付きコマンドの実行。
- 起動済みウィンドウの再利用と前面化。KDE Plasma の Wayland と X11 に対応。
- USB ボタンの常駐監視、トレイ常駐、前回保存した JSON の復元。

## ビルド

Go 1.24 以降、C コンパイラー、pkg-config、libusb、OpenGL、X11 の開発ライブラリーを使用します。Wayland 環境では XWayland を使用します。

Artix Linux / Arch Linux:

```sh
sudo pacman -S --needed go base-devel pkgconf libusb libglvnd libxcursor libxrandr libxinerama libxi libxxf86vm ttf-droid
make build
```

Debian / Ubuntu:

```sh
sudo apt install build-essential pkg-config libusb-1.0-0-dev libgl1-mesa-dev xorg-dev fonts-droid-fallback
make build
```

Debian / Ubuntu では Go 1.24 以降を別途用意してください。日本語の表示には Droid Sans Fallback または Noto Sans JP の TTF を使用します。別のフォントを使う場合は `FYNE_FONT=/path/to/font.ttf ./bin/key-short` として起動してください。

## USB のアクセス権限

最初に udev ルールをインストールし、USB デバイスを抜き差しします。

```sh
sudo ./scripts/install-udev.sh
```

ルールは `/etc/udev/rules.d/70-key-short.rules` に配置されます。`1189:8890` の USB デバイスと入力イベントに限って `uaccess` を付与します。Artix の elogind または systemd-logind による、ローカルセッションへのアクセス権限の付与を想定しています。ランチャー機能の追加前にルールを導入していた場合も、このスクリプトを再実行してください。

リモートセッションや elogind/logind を使用しない環境では、管理者が USB デバイスへのアクセス権限を設定してください。アプリは一般ユーザーで起動します。

## 起動と操作

```sh
./bin/key-short
```

1. 対象の USB デバイスを選択します。接続後は「再検出」で一覧を更新します。
2. レイヤーとキー、またはノブの操作を選択します。
3. 操作の種類と割り当てを指定し、「設定に追加」を押します。
4. 「JSON に保存」で設定をファイルに保存します。
5. 「送信データを確認」で内容を確認し、「USB に書き込む」を押します。
6. 実際のキー・ノブ操作で結果を確認します。

初期状態の設定は空です。追加した項目だけを送信します。「書き込み対象から外す」はアプリ内の設定から項目を取り除きます。本体の割り当ては消去しません。空のキー操作を送る場合は `keyboard` の値に `none` を指定できますが、無効化の挙動は本体のファームウェアに依存します。

アプリを閉じる前に JSON に保存してください。次回の GUI 起動時に、最後に保存・読み込みした JSON を開きます。未保存の編集内容は復元しません。

アプリメニューに登録する場合:

```sh
make install
```

実行ファイルは `~/.local/bin/key-short`、デスクトップエントリーは `${XDG_DATA_HOME:-~/.local/share}/applications/key-short.desktop` に配置されます。

## アプリランチャー

1. キーを選び、操作を「ランチャー」にします。
2. 「アプリを選択」でインストール済みアプリを選ぶか、「コマンドラインで実行」で引数付きコマンドを入力します。
3. 再利用したい場合は「起動済みなら、そのウィンドウを前面にする」を有効にします。無効の場合は押すたびに起動コマンドを実行します。
4. 「起動をテスト」で起動と前面化を確認します。この操作は USB への書き込みを必要としません。
5. 「設定に追加」、JSON の保存、USB への書き込みを行い、「ランチャー監視を開始」を押します。

「閉じてもトレイに常駐」を有効にすると、設定ウィンドウを閉じた後も監視を続けます。設定を変更すると監視を停止します。設定変更後は監視を開始し直してください。保存済みの設定で監視を有効にして終了した場合は、次の GUI 起動時に監視を再開します。

コマンド例、起動履歴の扱い、対応環境は [アプリランチャーの使い方](docs/launcher.md) を参照してください。サンプルは [examples/app-launcher.json](examples/app-launcher.json) にあります。

## 割り当ての記法

| 種類 | 例 | 動作 |
| --- | --- | --- |
| `keyboard` | `ctrl+c` | Ctrl+C |
| `keyboard` | `ctrl+shift+s` | Ctrl+Shift+S |
| `keyboard` | `ctrl+c,ctrl+v` | 2つのショートカットを順に送信 |
| `keyboard` | `f13` | F13 |
| `keyboard` | `henkan` | 変換キー |
| `media` | `volume_up` | 音量を上げる |
| `media` | `play_pause` | 再生・一時停止 |
| `mouse` | `left_click` | 左クリック |
| `mouse` | `ctrl+wheel_down` | Ctrl を伴う下スクロール |

修飾キーは `ctrl`、`shift`、`alt`、`super`、`rctrl`、`rshift`、`ralt`、`rsuper` です。マウス操作では `ctrl`、`shift`、`alt` を使用できます。

記号は `comma`、`dot`、`slash`、`semicolon`、`equal` などの HID キー名で指定します。文字列の入力や日本語テキストのマクロではありません。出力される文字は OS のキーボード配列に依存します。キーボードマクロに任意の待ち時間は指定できません。アプリの起動には「ランチャー」を使用します。

## CLI

デバイスを検出します。この操作は本体に設定を書き込みません。

```sh
./bin/key-short -list
```

サンプル設定の送信データを表示します。USB デバイスがなくても使用できます。

```sh
./bin/key-short -dry-run examples/volume-and-shortcuts.json
```

設定を本体に書き込みます。`-device` の値は `-list` の結果で置き換えてください。対象が1台の場合は `-device` を省略できます。

```sh
./bin/key-short -device 002:003 -apply examples/volume-and-shortcuts.json
```

CLI の `-apply` は確認画面を表示せずに書き込みます。サンプルはキー1・2と両方のノブを上書きします。

GUI を開かずにランチャーを常駐させる場合:

```sh
./bin/key-short -listen examples/app-launcher.json
```

USB を使わず、指定スロットの起動・前面化を実行する場合:

```sh
./bin/key-short -launch examples/app-launcher.json -layer 1 -slot 2
```

指定の設定で GUI を開く場合:

```sh
./bin/key-short -profile examples/app-launcher.json
```

`-verbose` で送信データを含む DEBUG ログを有効にできます。DEBUG と INFO は標準出力、WARN と ERROR は標準エラー出力に送ります。処理を継続できない場合は ERROR を出力し、終了コード1を返します。

## 設定ファイル

```json
{
  "version": 1,
  "device": "1189:8890",
  "led": null,
  "bindings": [
    { "layer": 1, "slot": 1, "kind": "keyboard", "value": "ctrl+c" },
    { "layer": 1, "slot": 18, "kind": "media", "value": "volume_up" }
  ]
}
```

`led: null` は LED を変更しません。0 は消灯、1 と2はファームウェアのプリセットです。

| 入力番号 | 操作 |
| --- | --- |
| 1〜12 | キー1〜12 |
| 13 / 14 / 15 | ノブ1の左回転 / 押下 / 右回転 |
| 16 / 17 / 18 | ノブ2の左回転 / 押下 / 右回転 |

画面上のキー配置は番号の一覧です。実機の物理配置や回転方向と対応しない場合は、1項目ずつ割り当てて確認してください。

## 通信と検証状況

提供された `Release/MINI KeyBoard.exe` の通信処理を解析し、Report ID 3、65バイトのレポート、レイヤー指定、キーボード・メディア・マウス・LED の保存処理を確認しました。設定用インターフェース1の interrupt OUT エンドポイント `0x02` に送信します。キーボード・マウス用インターフェースのドライバー解除や USB リセットは行いません。

この環境では `1189:8890` の接続と USB ディスクリプターを確認しています。本体への設定書き込みと、12キー・2ノブの操作結果は未検証です。USB のアクセス権限を付与してから実機で確認してください。レイヤー2・3や LED の対応状況はファームウェアによって異なります。

ランチャーは、KDE Plasma の Wayland セッションでテスト用アプリの起動と前面化を確認しました。X11 バックエンドも XWayland 上のテスト用ウィンドウで確認しています。現在は USB デバイスが未接続のため、物理ボタンからの起動は未検証です。

本体の現在の割り当てを読み出す機能はありません。送信成功は USB 転送の完了を表し、保存内容を読み戻して照合した結果ではありません。途中で通信が失敗すると一部だけ設定されることがあります。その場合は接続し直し、JSON から再送信してください。

関連する公開資料:

- [Fyne のビルド手順](https://docs.fyne.io/started/)
- [gousb の API](https://pkg.go.dev/github.com/google/gousb)
- [同じデバイスのプロトコル調査](https://github.com/jgt87/Macropad/blob/main/custom/PROTOCOL.md)
- [libusb による実装例](https://github.com/devkev/hid-minikb-libusb)

元の Windows バイナリーと ZIP は参照用として元の場所に残し、Git の管理対象から除外しています。逆コンパイルしたコードはアプリに含めていません。

## 開発時の検証

```sh
make test
make check
make build
```

テストでは提供された Windows 版の送信形式と期待するフレームを照合し、複数操作、ノブ2、レイヤー、LED の保存コマンド、不正な入力、JSON の保存・読み込みを検証します。テストは USB に設定を書き込みません。

ランチャーのテストでは、引数と引用符の保持、起動履歴の永続化、PID の再利用、連打、入力イベント、監視プロセスの排他、GUI からの設定を検証します。通常のテストは実際のアプリを起動しません。

実デスクトップでの統合テストは、テスト専用ウィンドウを開いて前面化します。

```sh
KEY_SHORT_TEST_DESKTOP=1 go test -p 1 ./internal/window ./internal/launcher -run 'TestDesktopBackends|TestNativeLaunchAndPersistentFocus' -v -count=1
```
