#!/bin/sh
set -eu
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_dir=$(dirname -- "$script_dir")
if [ ! -x "$project_dir/bin/key-short-install" ]; then
    echo 'ERROR make install を実行してください。' >&2
    exit 1
fi
exec "$project_dir/bin/key-short-install" install
