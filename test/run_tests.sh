#!/bin/bash

set -eo pipefail

function cleanup {
    find -type f -name logs.pos | xargs -I{} rm '{}'
    find -type d -name ${TEST_RESULT_DIR} | xargs -I{} rm -rf '{}'
}

function get_metrics {
    curl -s ${METRICS_URL}
}

function evaluate_test_result {
    while read line ; do
        if ! grep -q "$line" ${TEST_RESULT_DIR}/${METRICS_FN}; then
            echo "  FAIL: Missing the following metric!"
            echo "$line"
            exit 1
        fi
    done < ${METRICS_FN}
    echo "  OK: found all expected metrics"
}

SCRIPT_FN=$(readlink -f $0)
SLO_EXPORTER="$( dirname "${SCRIPT_FN}" )/../slo_exporter"

TEST_DIR_PREFIX="Test_"
TEST_RESULT_DIR="test_ouput"

CONFIG_FN="slo_exporter.yaml"
METRICS_URL="http://localhost:8080/metrics"
METRICS_FN="metrics"
EXPECTED_METRICS_FN="metrics.expected"

SLO_EXPORTER_LOG_FN="slo_exporter.log"

cleanup

for i_test in $(find $(dirname "${SCRIPT_FN}" ) -type d | grep ${TEST_DIR_PREFIX}) ; do
    echo "${i_test}"

    pushd ${i_test} > /dev/null
    mkdir ${TEST_RESULT_DIR}
    ${SLO_EXPORTER} --config-file=${CONFIG_FN} --disable-timescale-exporter > ${TEST_RESULT_DIR}/${SLO_EXPORTER_LOG_FN} 2>&1 &
    sleep 1
    get_metrics > ${TEST_RESULT_DIR}/${METRICS_FN}
    # kill slo exporter test instance
    kill %1 || true

    evaluate_test_result
    popd > /dev/null
done
