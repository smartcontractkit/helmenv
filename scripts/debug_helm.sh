#!/usr/bin/env bash

set -e

# get this scripts directory
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &>/dev/null && pwd )
CHARTS_DIR="${SCRIPT_DIR}"/../environment/charts/

# Start in the charts dir
cd "${CHARTS_DIR}"

# Clean up the path because some versions of sh don't like the /../ in the loop
CHARTS_DIR="$(pwd)"

debug_directory(){
    local -r directory="$1"

    helm template "${directory}" --debug
}

for directory in "${CHARTS_DIR}"/*/     # list directories
do
    directory=${directory%*/}      # remove the trailing "/"
    debug_directory "${directory}"
done

