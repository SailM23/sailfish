#!/bin/sh

set -e
set -x

# new default 8080 port for this for speed
port=${port:-8080}

scriptdir=$(cd $(dirname $0); pwd)
. ${scriptdir}/common-vars.sh

if [ -z "$1" ]; then
    echo "need to specify event file"
    exit 1
fi

tmpfile=$(mktemp ./TEMP-XXXXXX)
trap "rm -f $tmpfile" EXIT

while read -u 5 line ; do
   echo "$line" > $tmpfile
    $CURLCMD --fail -f $BASE/api/Event%3AInject -d  @$tmpfile

    if [ -n "$singlestep" ]; then  read -p "Paused" pause; fi

# rate limit requests, if needed:
#    i=$((i+1))
#    if [ $((i%20)) -eq 0 ] ; then
#        sleep 1
#    fi

done 5<$1
