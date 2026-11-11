#!/bin/sh
set -eu

if [ "$(id -u)" -ne 0 ]; then
    echo 'ERROR sudo ./scripts/install-udev.sh として実行してください。' >&2
    exit 1
fi
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
install -m 0644 "$script_dir/../packaging/70-key-short.rules" /etc/udev/rules.d/70-key-short.rules
udevadm control --reload-rules
echo 'INFO udev ルールを導入しました。USB デバイスを抜き差ししてください。'
