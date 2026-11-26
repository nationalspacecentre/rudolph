#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$DIR"

mkdir -p build/package
staging="$(mktemp -d build/package/.tmp.XXXXXX)"
trap 'rm -f "$staging/api.zip" "$staging/authorizer.zip"; rmdir "$staging"' EXIT

echo "Building Linux ARM64 Lambda packages"
for target in api authorizer; do
    bootstrap="build/linux/$target/bootstrap"
    GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -mod=readonly -trimpath -tags lambda.norpc -o "$bootstrap" "./cmd/$target"
    chmod 755 "$bootstrap"

    package="build/package/api_deployment.zip"
    if [ "$target" = authorizer ]; then
        package="build/package/api_authorizer_deployment.zip"
    fi
    zip -q -j "$staging/$target.zip" "$bootstrap"
    mv -f "$staging/$target.zip" "$package"
    echo "  $package"
done

if [ "$(uname)" = Darwin ]; then
    echo "Building universal macOS CLI"
    GOOS=darwin GOARCH=amd64 go build -mod=readonly -o build/macos/cli_amd64 ./cmd/cli
    GOOS=darwin GOARCH=arm64 go build -mod=readonly -o build/macos/cli_arm64 ./cmd/cli
    lipo -create -output build/macos/cli build/macos/cli_amd64 build/macos/cli_arm64
    ln -sf "$DIR/build/macos/cli" "$DIR/rudolph"
else
    echo "Building native CLI"
    go build -mod=readonly -o build/cli/cli ./cmd/cli
fi
