#!/bin/sh
set -eu
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_dir=$(dirname -- "$script_dir")
data_dir=${XDG_DATA_HOME:-"$HOME/.local/share"}
bin_dir="$HOME/.local/bin"
install -Dm755 "$project_dir/bin/key-short" "$bin_dir/key-short"
install -Dm644 "$project_dir/packaging/key-short.svg" "$data_dir/icons/hicolor/scalable/apps/key-short.svg"
mkdir -p "$data_dir/applications"
# Quote Exec according to the desktop entry specification (paths may contain spaces).
escaped_bin=$(printf '%s' "$bin_dir/key-short" | sed 's/[\\"`$]/\\&/g; s/%/%%/g')
cat > "$data_dir/applications/key-short.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=Key Short
Comment=MINI KeyBoard のキーとノブを設定
Exec="$escaped_bin"
Icon=key-short
Terminal=false
Categories=Settings;HardwareSettings;
StartupNotify=true
EOF
echo "INFO インストールしました: $bin_dir/key-short"
