#!/bin/bash
# Copyright 2016 VMware, Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Build the bootstrap filesystem ontop of the base

# exit on failure
set -e

if [ -n "$DEBUG" ]; then
    set -x
fi

DIR=$(dirname $(readlink -f "$0"))
. $DIR/base/utils.sh

function usage() {
echo "Usage: $0 -p staged-package(tgz) -b binary-dir -d <activates debug when set>" 1>&2
exit 1
}

while getopts "p:b:d:" flag
do
    case $flag in

        p)
            # Required. Package name
            package="$OPTARG"
            ;;

        b)
            # Required. Target for iso and source for components
            BIN="$OPTARG"
            ;;
        d)
            # Optional. directs script to make a debug iso instead of a production iso.
            debug="$OPTARG"
            ;;
        *)

            usage
            ;;
    esac
done

shift $((OPTIND-1))

# check there were no extra args and the required ones are set
if [ ! -z "$*" -o -z "$package" -o -z "${BIN}" ]; then
    usage
fi

#################################################################
# Above: arg parsing and setup
# Below: the image authoring
#################################################################

PKGDIR=$(mktemp -d)

unpack $package $PKGDIR

#selecting the init script as our entry point.
if [ -v debug ]; then
    export ISONAME="bootstrap-debug.iso"
    rootfs_cmd ${PKGDIR} cp ${DIR}/bootstrap/bootstrap.debug bin/bootstrap
    rootfs_cmd ${PKGDIR} cp ${BIN}/rpctool sbin/
else
    export ISONAME="bootstrap.iso"
    rootfs_cmd ${PKGDIR} cp ${DIR}/bootstrap/bootstrap bin/bootstrap
fi

# copy in our components
rootfs_cmd ${PKGDIR} cp ${BIN}/tether-linux bin/tether

# kick off our components at boot time
rootfs_cmd ${PKGDIR} mkdir -p etc/systemd/system/vic.target.wants
rootfs_cmd ${PKGDIR} cp ${DIR}/bootstrap/tether.service etc/systemd/system/
rootfs_cmd ${PKGDIR} cp ${DIR}/appliance/vic.target etc/systemd/system/
rootfs_cmd ${PKGDIR} ln -s /etc/systemd/system/tether.service etc/systemd/system/vic.target.wants/
rootfs_cmd ${PKGDIR} ln -sf /etc/systemd/system/vic.target etc/systemd/system/default.target

# disable networkd given we manage the link state directly
rootfs_cmd ${PKGDIR} rm -f etc/systemd/system/multi-user.target.wants/systemd-networkd.service
rootfs_cmd ${PKGDIR} rm -f etc/systemd/system/sockets.target.wants/systemd-networkd.socket

# do not use the systemd dhcp client
rootfs_cmd ${PKGDIR} rm -f etc/systemd/network/*
rootfs_cmd ${PKGDIR} cp ${DIR}/base/no-dhcp.network etc/systemd/network/

generate_iso $PKGDIR $BIN/$ISONAME /lib/systemd/systemd
