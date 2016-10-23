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

# Build the appliance filesystem ontop of the base

# exit on failure and configure debug, include util functions
set -e && [ -n "$DEBUG" ] && set -x
DIR=$(dirname $(readlink -f "$0"))
. $DIR/base/utils.sh


function usage() {
echo "Usage: $0 -p staged-package(tgz) -b binary-dir" 1>&2
exit 1
}

while getopts "p:b:" flag
do
    case $flag in

        p)
            # Required. Package name
            PACKAGE="$OPTARG"
            ;;

        b)
            # Required. Target for iso and source for components
            BIN="$OPTARG"
            ;;

        *)
            usage
            ;;
    esac
done

shift $((OPTIND-1))

# check there were no extra args and the required ones are set
if [ ! -z "$*" -o -z "$PACKAGE" -o -z "${BIN}" ]; then
    usage
fi

PKGDIR=$(mktemp -d)

# unpackage base package
unpack $PACKAGE $PKGDIR

#################################################################
# Above: arg parsing and setup
# Below: the image authoring
#################################################################

## systemd configuration
# create systemd vic target
rootfs_cmd $PKGDIR cp ${DIR}/appliance/vic.target etc/systemd/system/
rootfs_cmd $PKGDIR cp ${DIR}/appliance/vic-init.service etc/systemd/system/
rootfs_cmd $PKGDIR cp ${DIR}/appliance/nat.service etc/systemd/system/
rootfs_cmd $PKGDIR cp ${DIR}/appliance/nat-setup etc/systemd/scripts

rootfs_cmd $PKGDIR mkdir -p etc/systemd/system/vic.target.wants
rootfs_cmd $PKGDIR ln -s /etc/systemd/system/vic-init.service etc/systemd/system/vic.target.wants/
rootfs_cmd $PKGDIR ln -s /etc/systemd/system/nat.service etc/systemd/system/vic.target.wants/
rootfs_cmd $PKGDIR ln -s /etc/systemd/system/multi-user.target etc/systemd/system/vic.target.wants/

# disable networkd given we manage the link state directly
rootfs_cmd $PKGDIR rm -f etc/systemd/system/multi-user.target.wants/systemd-networkd.service
rootfs_cmd $PKGDIR rm -f etc/systemd/system/sockets.target.wants/systemd-networkd.socket

# change the default systemd target to launch VIC
rootfs_cmd $PKGDIR ln -sf /etc/systemd/system/vic.target etc/systemd/system/default.target

# do not use the systemd dhcp client
rootfs_cmd $PKGDIR rm -f etc/systemd/network/*
rootfs_cmd $PKGDIR cp ${DIR}/base/no-dhcp.network etc/systemd/network/

# do not use the default iptables rules - nat-setup supplants this
rootfs_cmd $PKGDIR rm -f etc/systemd/network/*

# populate the vic-admin assets
rootfs_cmd $PKGDIR cp -R ${DIR}/vicadmin/* home/vicadmin
rootfs_cmd $PKGDIR chown -R 1000:1000 home/vicadmin
# so vicadmin can read the system journal via journalctl
rootfs_cmd $PKGDIR install -m 755 -d etc/tmpfiles.d
echo "m  /var/log/journal/%m/system.journal 2755 root systemd-journal - -" > $(rootfs_dir $PKGDIR)/etc/tmpfiles.d/systemd.conf

## main VIC components
# tether based init
rootfs_cmd $PKGDIR cp ${BIN}/vic-init sbin/vic-init

rootfs_cmd $PKGDIR cp ${BIN}/{docker-engine-server,port-layer-server,vicadmin} sbin/

echo "net.ipv4.ip_forward = 1" > $(rootfs_dir $PKGDIR)/usr/lib/sysctl.d/50-vic.conf

## Generate the ISO
# Select systemd for our init process
generate_iso $PKGDIR $BIN/appliance.iso /lib/systemd/systemd
