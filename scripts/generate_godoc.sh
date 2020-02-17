#!/bin/bash
CURRENT_PKG=$(go list -m)
INTERFACE=localhost:6060

DST_DIR=${1:-"public/godoc"}

# run a godoc server
go get golang.org/x/tools/cmd/godoc
godoc -http=$INTERFACE & DOC_PID=$!

sleep 10
# Wait for the server to start
until curl -sSf "http://$INTERFACE/pkg/$CURRENT_PKG/" > /dev/null
do
    sleep 1
done
sleep 1

# recursive fetch entire web including CSS & JS
# turn off robots check, otherwise might get blocked with details in `robots.txt` file
# only get the directories we are looking for
wget -r -p \
    -e robots=off \
    --include-directories="/lib/godoc,/pkg/$CURRENT_PKG,/src/$CURRENT_PKG" \
    --exclude-directories="/pkg/$CURRENT_PKG/vendor,/src/$CURRENT_PKG/vendor" \
    "http://$INTERFACE/pkg/$CURRENT_PKG/"

# Stop the godoc server
kill -9 $DOC_PID

# all file will be generated into `localhost:6060` folder, hence we move them out from docker to local machine
mkdir -p "$(dirname "$DST_DIR")"
rm -rf "$DST_DIR"
mv "$INTERFACE" "$DST_DIR"
# replace relative links
find "$DST_DIR" -name "*.html" -exec sed -Ei 's/\/(lib|src|pkg)\//\/slo-exporter\/godoc\/\1\//g' {} +
