#!/bin/bash

# list-unclassified-endpoints.sh <log-file> [classification-csv-file]
#
#   lists not-classified SLO endpoints from proxy log file <log-file>

set -eo pipefail

SCRIPT_DIR=$(dirname $(readlink -f $0))
LOG_FILE=$1
CLASSIFICATION_FILE=${2:-${SCRIPT_DIR}/../examples/userportal.csv}

if [[ "$#" -lt "1" || "${LOG_FILE}" =~ ^(-h|--help)$ || ! -s "${LOG_FILE}" ]]; then
    echo "See usage:" >&2
    awk 'NR > 1 && $0 ~ /^# /{print substr($0, 2)}' $0 >&2
    exit 1
fi

${SCRIPT_DIR}/../slo_exporter --slo-domain=userportal --regexp-classification-file=${CLASSIFICATION_FILE} ${LOG_FILE} | \
  grep -Eo 'unable to classify event [^"]+' | awk '{print $5}' | sort | uniq -c | sort -n -r
