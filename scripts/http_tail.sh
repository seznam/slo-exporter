#!/bin/bash

URL=${URL:-"https://skweb1.ko.seznam.cz:8888/szn-sklik-userproxy/access_log"}

if [ -z "$SZN_LOGY_USER" -o -z "$SZN_LOGY_PASSWORD" ]; then
  echo "Envs SZN_LOGY_USER and SZN_LOGY_PASSWORD are mandatory!"
  exit 1
fi

touch access_log
while(true); do
    wget -c  --password "$SZN_LOGY_PASSWORD" --user "$SZN_LOGY_USER" --tries=3 -o /dev/null --no-check-certificate  "$URL" || echo "WGET FAILED! Remove the '-o /dev/null' in the /scripts/http_tail.sh and rerun."
    sleep 2;
done
