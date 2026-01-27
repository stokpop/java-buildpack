#!/usr/bin/env bash

set -e
set -u
set -o pipefail

ROOTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly ROOTDIR

# shellcheck source=SCRIPTDIR/.util/tools.sh
source "${ROOTDIR}/scripts/.util/tools.sh"

# shellcheck source=SCRIPTDIR/.util/print.sh
source "${ROOTDIR}/scripts/.util/print.sh"

function main() {
  local stack version cached output fast_zip
  stack="cflinuxfs4"
  cached="false"
  output="${ROOTDIR}/build/buildpack.zip"
  fast_zip="false"

  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      --stack)
        stack="${2}"
        shift 2
        ;;

      --version)
        version="${2}"
        shift 2
        ;;

      --cached)
        cached="true"
        shift 1
        ;;

      --output)
        output="${2}"
        shift 2
        ;;

      --fast-zip)
        fast_zip="true"
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
        util::print::error "unknown argument \"${1}\""
    esac
  done

  if [[ -z "${version:-}" ]]; then
    version=$(cat "${ROOTDIR}/VERSION" 2>/dev/null || echo "0.0.0")
    echo "No version specified, using VERSION file: ${version}"
  fi

  package::buildpack "${version}" "${cached}" "${stack}" "${output}" "${fast_zip}"
}


function usage() {
  cat <<-USAGE
package.sh --version <version> [OPTIONS]
Packages the buildpack into a .zip file.
OPTIONS
  --help               -h            prints the command usage
  --version <version>  -v <version>  specifies the version number to use when packaging the buildpack
  --cached                           cache the buildpack dependencies (default: false)
  --stack  <stack>                   specifies the stack (default: cflinuxfs4)
  --output <file>                    output file path (default: build/buildpack.zip)
  --fast-zip                         recompress with store-only (no compression) to speed local tests
USAGE
}

function package::buildpack() {
  local version cached stack output fast_zip
  version="${1}"
  cached="${2}"
  stack="${3}"
  output="${4}"
  fast_zip="${5}"

  mkdir -p "$(dirname "${output}")"

  util::tools::buildpack-packager::install --directory "${ROOTDIR}/.bin"

  echo "Building buildpack (version: ${version}, stack: ${stack}, cached: ${cached}, output: ${output}, fast_zip: ${fast_zip})"

  local stack_flag
  stack_flag="--any-stack"
  if [[ "${stack}" != "any" ]]; then
    stack_flag="--stack=${stack}"
  fi

  local file
  file="$(
    "${ROOTDIR}/.bin/buildpack-packager" build \
      "--version=${version}" \
      "--cached=${cached}" \
      "${stack_flag}" \
    | xargs -n1 | grep -e '\.zip$'
  )"

  mv "${file}" "${output}"

  if [[ "${fast_zip}" == "true" ]]; then
    # Recompress to store-only to reduce CPU during test packaging usage
    local tmpdir tmpzip
    tmpdir="$(mktemp -d)"
    tmpzip="$(mktemp -u).zip"

    # Extract current zip
    unzip -q "${output}" -d "${tmpdir}"

    # Create a new zip with no compression
    (
      cd "${tmpdir}"
      zip -q -r -0 "${tmpzip}" .
    )

    # Replace output with store-only zip
    mv "${tmpzip}" "${output}"
    rm -rf "${tmpdir}"
  fi
}

main "${@:-}"
