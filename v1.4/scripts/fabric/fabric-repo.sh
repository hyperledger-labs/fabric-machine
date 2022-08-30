#!/usr/bin/env bash
# This script sets up (or cleans up) Fabric repo directory with code changes from opensource
# Fabric Machine repo directory.
#
# Usage:
#   ./fabric-repo.sh -d fabric-v1.4 -c
#   ./fabric-repo.sh -d fabric-v1.4 -s
set -e

# Arguments
FABRIC_DIR=
CLEANUP=false
SETUP=false

POSITIONAL=()
while [[ $# -gt 0 ]]; do
key="$1"
case $key in
    # Fabric repo directory (to be used as destination directory).
    -d)
    FABRIC_DIR=$2
    shift # past argument
    ;;

    # Clean up the provided directory.
    -c)
    CLEANUP=true
    shift # past argument
    ;;

    # Set up the provided directory.
    -s)
    SETUP=true
    shift # past argument
    ;;

    *)    # unknown option
    POSITIONAL+=("$1") # save it in an array for later
    shift # past argument
    ;;
esac
done
set -- "${POSITIONAL[@]}" # restore positional parameters

if [[ -z "$FABRIC_DIR" ]]; then
    echo "Please use -d flag to specify Fabric repo directory (to be used as destination directory)."
    exit
fi
if [[ $CLEANUP = false && $SETUP = false ]]; then
    echo "Please provide either -c (cleanup) or -s (setup) flag."
    exit
fi

fabricmachine_dir=$(realpath $(dirname $0)/../../)
fabric_dir=$(realpath $FABRIC_DIR)
curr_dir=$(pwd)

echo ""
echo "Updating Fabric setup as follows ..."
echo "Fabric Machine repo: $fabricmachine_dir"
echo "Fabric repo: $fabric_dir"

if [ $CLEANUP = true ]; then
    echo "Operation: cleanup"
    echo ""

    cd $fabric_dir
    git checkout .
    rm -rf fabricmachine samples scripts/fabric scripts/vm
    git status

    echo ""
    echo "Done!"
    cd $curr_dir
fi

if [ $SETUP = true ]; then
    echo "Operation: setup"
    echo ""

    cd $fabric_dir
    git checkout 11ff99170
    cp -rf $fabricmachine_dir/* $fabric_dir/.
    git status

    echo ""
    echo "Done!"
    cd $curr_dir
fi
