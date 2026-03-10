#!/bin/sh
# cSpell: words runlevel runlevels
# Script to be installed via the incus template mechanism and run via
# the lxc.hoot.start hook. It will add networking and iknite to the default
# openrc level.

# Adds specified services to the runlevel. Current working directory must be
# root of the image.
rc_add() {
	local runlevel="$1"; shift  # runlevel name
	local services="$*"  # names of services

	local svc; for svc in $services; do
		/bin/mkdir -p "/etc/runlevels/$runlevel"
		if [ -f "/etc/runlevels/$runlevel/$svc" ]; then
            echo " * service $svc already in runlevel $runlevel"
        else
            /bin/ln -s "/etc/init.d/$svc" "/etc/runlevels/$runlevel/$svc"
		    echo  " * service $svc added to runlevel $runlevel"
        fi
	done
}

date_log() {
    date +"%Y-%m-%dT%H:%M:%S"
}

/bin/echo "$(date_log) Starting prepare script..." >> /tmp/prepare.log || :
/bin/echo "$(date_log) * Adding services to runlevel..." >> /tmp/prepare.log || :
rc_add default networking iknite >> /tmp/prepare.log 2>&1 || :
/bin/echo "$(date_log) * Launching iknite prepare..." >> /tmp/prepare.log || :
/sbin/iknite prepare >> /tmp/prepare.log 2>&1 || :
/bin/echo "$(date_log) Prepare script finished." >> /tmp/prepare.log || :
