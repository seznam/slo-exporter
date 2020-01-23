#!/bin/bash

URL=${URL:-"https://skweb1.ko.seznam.cz:8888/szn-sklik-userproxy/access_log"}

if [ -z "$USER" -o -z "$PASSWD" ]; then
  echo "Envs USER and PASSWD are mandatory!"
  exit 1
fi

while(true); do
    sleep 2;
    wget -c  --password "$PASSWD" --user "$USER"  -o /dev/null --no-check-certificate  "$URL"
done
