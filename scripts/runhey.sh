#!/bin/bash

set -e

unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy

scriptdir=$(cd $(dirname $0); pwd)
. ${scriptdir}/common-vars.sh

outputdir=${1:-out/}
uri=${uri:-/redfish/v1/Managers/CMC.Integrated.1/Attributes}
runtoken=${runtoken:-1}
runbasic=${runbasic:-0}
BASIC_AUTH_HEADER=$(perl -MMIME::Base64 -e 'print "Authorization: Basic " . encode_base64("'$user':'$pass'")')

if [ -z "${outputdir}" ]; then
    echo Error: Need to set output directory
    exit 1
fi

rm -rf $outputdir/{token,basic} ||:
mkdir -p $outputdir $outputdir/token $outputdir/basic
LOGFILE=$outputdir/script-output.txt
exec 1> >(exec -a 'LOGGING TEE' tee $LOGFILE) 2>&1
TEEPID=$!

cleanup() {
    # close FDs to ensure tee finishes
    exec 1>&0 2>&1
    if [ -n "$TEEPID" ];then
        while ps --pid $TEEPID > /dev/null 2>&1
        do
            sleep 1
        done
    fi
}
trap 'cleanup' EXIT

set_auth_header

echo "Running hey"
numreqs=${numreqs:-1000}
concurrent=${concurrent:-5}
timelimit=${timelimit:-10}
single_step_rps_start=1
single_step_rps_end=30
ten_step_rps_start=40
ten_step_rps_end=200
rps=${rps:-"$(seq $single_step_rps_start $single_step_rps_end) $(seq $ten_step_rps_start 10 $ten_step_rps_end)"}

# to save CPU TOP information, you have to have SSH access to the box and already copied your ssh key. use "ssh-copy-id root@IP" to copy your key.
# on idrac, you also have to disable the avocent NSS module by mount binding /etc/nsswitch.conf and removing avct from passwd line
savetop() {
    echo "Starting TOP for hey run. RATE: $index" > $1
    ssh root@${host} 'top -b -d1 -o %CPU' >> $1 &
    SSHPID=$!
}

for i in ${rps} ; do
    index=$(printf "%03d" $i)
    concurrent_idx=$(printf "%03d" $concurrent)

    if [ "${runtoken}" = "1" ]; then
        outfile=${outputdir}/token/results-r${index}-c${concurrent_idx}
        [ "${profile}" == 1 ] && savetop ${outfile}-token-CPU.txt
        hey -q ${i} -c ${concurrent} -z ${timelimit}s -H "${AUTH_HEADER}" ${BASE}$uri  | tee ${outfile}-token.txt
        [ -n "$SSHPID" ] && kill $SSHPID ||:
    fi
    sleep 1
    if [ "${runbasic}" = "1" ]; then
        outfile=${outputdir}/basic/results-r${index}-c${concurrent_idx}
        [ "${profile}" == 1 ] && savetop ${outfile}-basic-CPU.txt
        hey -q ${i} -c ${concurrent} -z ${timelimit}s -H "${BASIC_AUTH_HEADER}"  ${BASE}${uri} | tee ${outfile}-basic.txt
        [ -n "$SSHPID" ] && kill $SSHPID ||:
    fi
    sleep 1
done

for i in {token,basic}; do
    [ -d ${outputdir}/$i ] || continue
    grep Requests ${outputdir}/${i}/results-r*.txt  > ${outputdir}/RPS-${i}.txt ||:
    grep Fastest ${outputdir}/${i}/results-r*.txt  > ${outputdir}/FASTEST-${i}.txt ||:
    grep Slowest ${outputdir}/${i}/results-r*.txt  > ${outputdir}/SLOWEST-${i}.txt ||:
    grep Average ${outputdir}/${i}/results-r*.txt  > ${outputdir}/AVERAGE-${i}.txt ||:
    grep ^%Cpu ${outputdir}/${i}/results-r*-CPU.txt  > ${outputdir}/TOTALCPU-${i}.txt ||:
done

# close FDs to ensure tee finishes
exec 1>&0 2>&1
if [ -n "$logging_tee_pid" ];then
    while ps --pid $logging_tee_pid > /dev/null 2>&1
    do
        sleep 1
    done
fi

