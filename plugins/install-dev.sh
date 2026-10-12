#!/usr/bin/env bash
# Install the veil demo plugins into micro-dev's config dir only.
# Your stable micro (~/.config/micro) is never touched.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
dest="${MICRO_DEV_CONFIG:-$HOME/.config/micro-dev}/plug"
mkdir -p "$dest"

for p in "$here"/*/; do
    name="$(basename "$p")"
    rm -rf "${dest:?}/$name"
    cp -r "$p" "$dest/$name"
    rm -rf "${dest:?}/$name/spec" # tests aren't plugin code (micro would ignore them anyway)
    echo "installed $name -> $dest/$name"
done
