#!/bin/bash
set -x

tag=${IMAGE##*:}
registry=${IMAGE%:*}
name=${registry##*/}
registry=${registry%/*}

tmp=$(mktemp -d)
trap "rm -rf $tmp" EXIT
cp -r ./charts/gardener/provider-local "$tmp"
tmp="$tmp/provider-local"
yq -i ".image |= \"$IMG\"" "$tmp/values.yaml"
yq -i ".name |= \"$name\"" "$tmp/Chart.yaml"
helm package "$tmp" -d "$tmp" --version "$tag"

helm push "$tmp/$name-$tag.tgz" "oci://$registry"
