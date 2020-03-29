#!/bin/bash

set -eo pipefail

# exit with $1 status code and printing $2 to stderr
function myexit {
    echo "$2" > /dev/stderr
    exit $1
}

function cleanup {
    find -type f -name logs.pos | xargs -I{} rm '{}'
    find -type d -name ${TEST_RESULT_DIR} | xargs -I{} rm -rf '{}'
}

function get_metrics {
    curl -s ${METRICS_URL}
}

function evaluate_test_result {
    while read line ; do
        if ! grep -q "$line" ${TEST_RESULT_DIR}/${METRICS_FILENAME}; then
            echo "  FAIL: Missing the following metric!"
            echo "$line"
            exit 1
        fi
    done < ${METRICS_FILENAME}
    echo "  OK: found all expected metrics"
}

SCRIPT_DIR=$( dirname "$(readlink -f $0)" )
SLO_EXPORTER="${SCRIPT_DIR}/../slo_exporter"

TEST_DIR_PREFIX="Test_"
TEST_RESULT_DIR="test_output"

CONFIG_FILENAME="slo_exporter.yaml"
METRICS_URL="http://localhost:8080/metrics"
METRICS_FILENAME="metrics"

SLO_EXPORTER_LOG_FILENAME="slo_exporter.log"

cleanup

for i_test in $(find "${SCRIPT_DIR}" -type d | grep ${TEST_DIR_PREFIX}) ; do
    echo "${i_test}"

    pushd ${i_test} > /dev/null
    mkdir ${TEST_RESULT_DIR}
    ${SLO_EXPORTER} --config-file=${CONFIG_FILENAME} > ${TEST_RESULT_DIR}/${SLO_EXPORTER_LOG_FILENAME} 2>&1 &
    sleep 1
    # test whether SLO_EXPORTER is running in the background (did not exited during the initialization)
    [ -z "$(jobs %% | grep Running)" ] && \
        myexit 1 "${SLO_EXPORTER} is not running. Exiting..."
    get_metrics > ${TEST_RESULT_DIR}/${METRICS_FILENAME}
    # kill slo exporter test instance
    kill %%

    evaluate_test_result
    popd > /dev/null
done
