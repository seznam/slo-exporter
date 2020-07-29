#!/bin/bash

GITHUB_NAMESPACE=${GITHUB_NAMESPACE:-seznam}
GITHUB_PROJECT=${GITHUB_PROJECT:-slo-exporter}

set -eo pipefail

if [ -n "${SLO_EXPORTER_VERSION}" ]; then
    echo "SLO_EXPORTER_VERSION=${SLO_EXPORTER_VERSION}"
else
    echo "SLO_EXPORTER_VERSION env variable has to be defined"
    exit 1
fi

mkdir release

# extract this particular release from changelog
awk -v version=${SLO_EXPORTER_VERSION} '$0 ~ /## \[.+\]/ {release = $2} release == "["version"]" {print $0}' CHANGELOG.md > release/CHANGELOG

# build tarballs with built binaries
for i_file in `find build -name slo_exporter -type f`; do
    tar -C `dirname $i_file` -czvf `echo $i_file | awk -F/ '{print "release/"$NF"."$(NF-1)".tgz"}'` `basename $i_file`
done

github-release release \
    --user ${GITHUB_NAMESPACE} \
    --repo ${GITHUB_PROJECT} \
    --tag ${SLO_EXPORTER_VERSION} \
    --name ${SLO_EXPORTER_VERSION} \
    --description "$(cat release/CHANGELOG)"

for i_file in `ls release/*tgz`; do
    github-release upload \
        --user ${GITHUB_NAMESPACE} \
        --repo ${GITHUB_PROJECT} \
        --tag ${SLO_EXPORTER_VERSION} \
        --name `basename ${i_file}` \
        --file ${i_file}
done


