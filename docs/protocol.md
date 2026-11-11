# MINI KeyBoard の通信形式

対象は `1189:8890`、Report ID 3 のファームウェアです。提供された Windows 版の `HIDTester.FormMain` にある `Download_Click`、`Send_SwLayer`、`Send_WriteFlash_Cmd`、`Send_WriteFlashLED_Cmd` と `HIDTester.HidLib.WriteDevice` を確認して実装しました。

USB の設定用インターフェースは1、送信先は interrupt OUT `0x02` です。1レポートは先頭の Report ID `03` と64バイトのペイロードから構成されます。以下の表はペイロードのみを示します。残りのバイトは0です。

| 処理 | ペイロード |
| --- | --- |
| レイヤー指定 | `a1 layer` |
| キーボードのヘッダー | `slot type count 00 modifier0 00` |
| キーボードの各操作 | `slot type count index modifier keycode` |
| メディア | `slot type usage_low usage_high` |
| マウス | `slot type buttons 00 00 wheel modifier` |
| 通常の保存 | `aa aa` |
| LED の変更 | `b0 18 mode` |
| LED の保存 | `aa a1` |

`type` の上位4ビットはレイヤー番号、下位4ビットは操作の種類です。キーボードは1、メディアは2、マウスは3、LED は8です。操作のインデックスは1から始まり、キーボードのヘッダーだけが0です。最大5操作を送信します。

キーの HID Usage Page は `0x07`、メディアは Consumer Page の使用コードです。メディアのコードは再生・一時停止 `cd`、前 `b6`、次 `b5`、停止 `b7`、ミュート `e2`、音量上げ `e9`、音量下げ `ea` です。

ノブ1は13・14・15、ノブ2は16・17・18を使用します。順番は左回転・押下・右回転です。

通常保存と LED 保存の2バイト目が異なるため、同じコマンドにまとめてはいけません。未設定の入力にはレポートを送信しません。現在の割り当てを読み出すコマンドは実装していません。

Linux 上で設定用インターフェース1が hidraw に公開されない個体を確認したため、libusb を使用しています。`gousb.SetAutoDetach(true)` は複数のインターフェースのドライバーを解除するため使用しません。
