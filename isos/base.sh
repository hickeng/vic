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

# Build the base of a bootable ISO

# exit on failure and configure debug, include util functions
set -e && [ -n "$DEBUG" ] && set -x
DIR=$(dirname $(readlink -f "$0"))
. $DIR/base/utils.sh


function usage() {
echo "Usage: $0 -p package-name(tgz) [-c yum-cache]" 1>&2
exit 1
}

while getopts "c:p:r:k:" flag
do
    case $flag in

        p)
            # Required. Package name
            PACKAGE="$OPTARG"
            ;;

        r)
            # Optional. Name of repo set in base/repos to use
            REPO="$OPTARG"
            ;;

        k)
            # Optional. Allows provision of custom kernel rpm
            # assumes it contains suitable /boot/vmlinuz-* and /lib/modules/... files
            CUSTOM_KERNEL_RPM="${OPTARG}"
            ;;

        c)
            # Optional. Offline cache of yum packages
            cache="$OPTARG"
            ;;

        *)
            usage
            ;;
    esac
done

shift $((OPTIND-1))

# check there were no extra args and the required ones are set
if [ ! -z "$*" -o -z "$PACKAGE" ]; then
    usage
fi

# prep the build system
ensure_apt_packages cpio rpm tar ca-certificates xz-utils

PKGDIR=$(mktemp -d)

# initialize the bundle
initialize_bundle $PKGDIR

# base filesystem setup
mkdir -p $(rootfs_dir $PKGDIR)/{etc/yum,etc/yum.repos.d}
ln -s /lib $(rootfs_dir $PKGDIR)/lib64

# work arounds for incorrect filesystem-1.0-13.ph2 package
mkdir -p $(rootfs_dir $PKGDIR)/{run,var}
ln -s /run $(rootfs_dir $PKGDIR)/var/run

if [[ $DRONE_BUILD_NUMBER && $DRONE_BUILD_NUMBER > 0 ]]; then
    # THIS SHOULD BE MOVED TO .drone.yml AS IT OVERRIDES THE -r OPTION
    REPOS="ci"
fi

cp -a $DIR/base/repos/${REPO:-photon-1.0}/* $(rootfs_dir $PKGDIR)/etc/yum.repos.d/
cp $DIR/base/yum.conf $(rootfs_dir $PKGDIR)/etc/yum/

if [ -z "${CUSTOM_KERNEL_RPM}" ]; then
    PHOTON_KERNEL="linux-esx"
fi

# install the core packages
yum_cached -c $cache -u -p $PKGDIR install filesystem coreutils kmod ${PHOTON_KERNEL} --nogpgcheck -y

# check for raw kernel override
if [ -n "${CUSTOM_KERNEL_RPM}" ]; then
    ( 
        echo "Using custom kernel package ${CUSTOM_KERNEL_RPM}"
        cd $(rootfs_dir $PKGDIR)
        rpm2cpio ${CUSTOM_KERNEL_RPM} | cpio -idm --extract-over-symlinks
    )
fi

# Issue 3858: find all kernel modules and unpack them and run depmod against that directory
find $(rootfs_dir $PKGDIR)/lib/modules -name "*.ko.xz" -exec xz -d {} \;
KERNEL_VERSION=$(basename $(rootfs_dir $PKGDIR)/lib/modules/*)
chroot $(rootfs_dir $PKGDIR) depmod $KERNEL_VERSION 

# strip the cache from the resulting image
yum_cached -c $cache -p $PKGDIR clean all

# move kernel into bootfs /boot directory so that syslinux could load it
mv $(rootfs_dir $PKGDIR)/boot/vmlinuz-* $(bootfs_dir $PKGDIR)/boot/vmlinuz64
# try copying over the other boot files - rhel kernel seems to need a side car configuration file
find $(rootfs_dir $PKGDIR)/boot -type f | xargs cp -t $(bootfs_dir $PKGDIR)/boot/

# package up the result
pack $PKGDIR $PACKAGE
