#!/usr/bin/env bash

set -euo pipefail

ROOTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly ROOTDIR

# shellcheck source=SCRIPTDIR/.util/print.sh
source "${ROOTDIR}/scripts/.util/print.sh"

# shellcheck source=SCRIPTDIR/.util/tools.sh
source "${ROOTDIR}/scripts/.util/tools.sh"

function usage() {
  cat <<-USAGE
integration.sh --github-token <token> [OPTIONS]

Runs the integration tests.

OPTIONS
  --help                         -h  prints the command usage
  --github-token <token>             GitHub token to use when making API requests
  --platform <cf|docker>             Switchblade platform to execute the tests against (default: cf)
  --cached <true|false>              Run cached/offline tests (default: false)
  --parallel <true|false>            Run tests in parallel (default: false)
  --stack <stack>                    Stack to use for tests (default: cflinuxfs4)
  --keep-failed-containers           Preserve failed test containers for debugging (default: false)

ENVIRONMENT
  GINKGO_NODES                       Number of tests run in parallel with --parallel true (default: 3)
  RERUN_FAILS                        Re-run each failed test up to this many times (default: 2, 0 disables)
  RERUN_FAILS_MAX_FAILURES           Do not re-run when more tests fail than this (default: 10)

EXAMPLES
  # Serial mode
  ./scripts/integration.sh --platform docker

  # Parallel mode (runs GINKGO_NODES tests at a time, default 3)
  ./scripts/integration.sh --platform docker --parallel true

  # Keep failed containers for debugging
  ./scripts/integration.sh --platform docker --keep-failed-containers
USAGE
}

function main() {
  local src stack platform token cached parallel keep_failed
  src="${ROOTDIR}/src/java/integration"
  stack="${CF_STACK:-cflinuxfs4}"
  platform="cf"
  cached="false"
  parallel="false"
  keep_failed="false"
  token="${GITHUB_TOKEN:-}"

  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      --platform)
        platform="${2}"
        shift 2
        ;;

      --github-token)
        token="${2}"
        shift 2
        ;;

      --cached)
        cached="${2}"
        shift 2
        ;;

      --parallel)
        parallel="${2}"
        shift 2
        ;;

      --stack)
        stack="${2}"
        shift 2
        ;;

      --keep-failed-containers)
        keep_failed="true"
        shift 1
        ;;

      --help|-h)
        shift 1
        usage
        exit 0
        ;;

      "")
        # skip if the argument is empty
        shift 1
        ;;

      *)
        echo "ERROR: unknown argument \"${1}\""
        usage
        exit 1
        ;;
    esac
  done

  echo "=== Java Buildpack Integration Tests ==="
  echo "Platform:           ${platform}"
  echo "Stack:              ${stack}"
  echo "Cached:             ${cached}"
  echo "Parallel:           ${parallel}"
  echo "Keep Failed:        ${keep_failed}"
  echo ""

  specs::run "${cached}" "${parallel}" "${stack}" "${platform}" "${token}" "${keep_failed}"
}

# Integration tests run through gotestsum so that only failed tests are re-run
# (e.g. after transient CF infrastructure errors) instead of the whole suite.
# RERUN_FAILS: max re-runs per failed test (0 disables).
# RERUN_FAILS_MAX_FAILURES: skip re-runs when more tests fail, as that indicates a real breakage.
GOTESTSUM_VERSION="v1.13.0"

function specs::run() {
  local cached parallel stack platform token keep_failed
  cached="${1}"
  parallel="${2}"
  stack="${3}"
  platform="${4}"
  token="${5}"
  keep_failed="${6}"

  local nodes cached_flag serial_flag platform_flag stack_flag token_flag keep_failed_flag
  cached_flag="--cached=${cached}"
  serial_flag="--serial=true"
  platform_flag="--platform=${platform}"
  stack_flag="--stack=${stack}"
  token_flag="--github-token=${token}"
  keep_failed_flag="--keep-failed-containers=${keep_failed}"
  nodes=1

  if [[ "${parallel}" == "true" ]]; then
    # Honour GINKGO_NODES from the CI pipeline (defaults to 3) so parallelism can be
    # tuned per environment without changing this script.
    nodes="${GINKGO_NODES:-3}"
    serial_flag=""
  fi

  cd "${ROOTDIR}"
  go mod download

  local buildpack_file
  buildpack_file="$(buildpack::package "1.2.3" "${cached}" "${stack}")"

  CF_STACK="${stack}" \
  BUILDPACK_FILE="${BUILDPACK_FILE:-"${buildpack_file}"}" \
  GOMAXPROCS="${GOMAXPROCS:-"${nodes}"}" \
    go run "gotest.tools/gotestsum@${GOTESTSUM_VERSION}" \
      --format standard-verbose \
      --rerun-fails="${RERUN_FAILS:-2}" \
      --rerun-fails-max-failures="${RERUN_FAILS_MAX_FAILURES:-10}" \
      --packages "${ROOTDIR}/src/integration" \
      -- \
      -count=1 \
      -timeout=0 \
      -mod vendor \
      -parallel "${nodes}" \
      -args \
         ${cached_flag} \
         ${platform_flag} \
         ${token_flag} \
         ${stack_flag} \
         ${serial_flag} \
         ${keep_failed_flag}
}

function buildpack::package() {
  local version cached stack
  version="${1}"
  cached="${2}"
  stack="${3}"

  local name cached_flag
  name="buildpack-${stack}-v${version}-uncached.zip"
  cached_flag=""
  if [[ "${cached}" == "true" ]]; then
    cached_flag="--cached"
    name="buildpack-${stack}-v${version}-cached.zip"
  fi

  local output
  output="$(mktemp -d)/${name}"

  CF_STACK="${stack}" bash "${ROOTDIR}/scripts/package.sh" \
    --version "${version}" \
    --output "${output}" \
    --stack "${stack}" \
    ${cached_flag} > /dev/null

  printf "%s" "${output}"
}

main "${@:-}"
