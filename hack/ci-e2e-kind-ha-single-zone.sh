#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o nounset
set -o pipefail
set -o errexit
set -x

source $(dirname "${0}")/ci-common.sh

clamp_mss_to_pmtu

ensure_glgc_resolves_to_localhost

# test setup
make kind-ha-single-zone-up

# export all container logs and events after test execution
trap "
  ( export_artifacts "gardener-local-ha-single-zone" )
  ( make kind-ha-single-zone-down )
" EXIT

make gardener-ha-single-zone-up
make test-e2e-local-ha-single-zone
make gardener-ha-single-zone-down
