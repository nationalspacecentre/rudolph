#!/usr/bin/env bash
set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."
export LC_ALL=C

bootstrap="$(mktemp)"
trap 'rm -f "$bootstrap"' EXIT

for archive in build/package/{api,api_authorizer}_deployment.zip; do
    unzip -tq "$archive"
    if [[ "$(unzip -Z1 "$archive")" != bootstrap ]]; then
        echo "$archive must contain exactly one root-level bootstrap" >&2
        exit 1
    fi

    permissions="$(unzip -Z -l "$archive" | awk '$NF == "bootstrap" { print $1 }')"
    if [[ "$permissions" != -rwxr-xr-x ]]; then
        echo "$archive must store bootstrap as a 0755 regular file" >&2
        exit 1
    fi

    unzip -p "$archive" bootstrap > "$bootstrap"
    binary="$(file -b "$bootstrap")"
    if [[ "$binary" != *"ELF 64-bit"*"ARM aarch64"*"statically linked"* ]]; then
        echo "$archive must contain a static Linux ARM64 executable (found $binary)" >&2
        exit 1
    fi
    echo "$archive: executable Linux ARM64 bootstrap"
done
