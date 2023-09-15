#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

### input (calculated by gardenlet using github.com/google/go-containerregistry)
# URL to the layer blob (sha256 reference) in the registry that contains the node-agent binary (final layer)
blob_url="{{ .layerURL }}"
blob_digest="${blob_url##*sha256:}"
# path to the node-agent binary in the layer filesystem (determined by entrypoint in image metadata)
blob_binary_path="{{ .binaryPath }}"
target_directory="/usr/local/bin"

### prepare directory
tmp_dir="$(mktemp -d)"
blob="$tmp_dir/gardener-node-agent.tar.gz"
trap 'rm -rf "$tmp_dir"' EXIT

### download + verify
echo "> Downloading blob from $blob_url"
curl -Lv "$blob_url" -o "$blob"

echo "> Verifying checksum of downloaded blob"
echo "$blob_digest $blob" | sha256sum --check

### extract + move
echo "> Extracting gardener-node-agent binary"
tar -xzvf "$blob" -C "$tmp_dir" "$blob_binary_path"

echo "> Ensuring target directory $target_directory"
mkdir -p "$target_directory"

echo "> Moving gardener-node-agent binary to target directory"
mv -f "$tmp_dir$blob_binary_path" "$target_directory/gardener-node-agent"

# Save most recently downloaded image ref
echo "{{ .nodeAgentImage }}" > /var/lib/gardener-node-agent/node-agent-downloaded

### bootstrap
echo "> Bootstrapping gardener-node-agent"
"$target_directory/gardener-node-agent" bootstrap
