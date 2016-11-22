// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2016 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package builtin

import (
	"bytes"

	"github.com/snapcore/snapd/interfaces"
	"github.com/snapcore/snapd/release"
)

const ofonoPermanentSlotAppArmor = `
# Description: Allow operating as the ofono service. Reserved because this
# gives privileged access to the system.

# to create ppp network interfaces
capability net_admin,

# To check present devices
/run/udev/data/+usb:* r,
/run/udev/data/+usb-serial:* r,
/run/udev/data/+pci:* r,
/run/udev/data/+platform:* r,
/run/udev/data/+pnp:* r,
/run/udev/data/c* r,
/run/udev/data/n* r,
/sys/bus/usb/devices/ r,
# FIXME snapd should be querying udev and adding the /sys and /run/udev accesses
# that are assigned to the snap, but we are not there yet.
/sys/bus/usb/devices/** r,

# To get current seat, used to know user preferences like default SIM in
# multi-SIM devices.
/run/systemd/seats/{,*} r,

# Access to modem ports
# FIXME snapd should be more dynamic to avoid conflicts between snaps trying to
# access same ports.
/dev/tty[^0-9]* rw,
/dev/cdc-* rw,
/dev/modem* rw,
/dev/dsp rw,
/dev/chnlat11 rw,
/dev/socket/rild* rw,
# ofono puts ppp on top of the tun device
/dev/net/tun rw,

network netlink raw,
network netlink dgram,
network bridge,
network inet,
network inet6,
network packet,
network bluetooth,

include <abstractions/nameservice>

# DBus accesses
include <abstractions/dbus-strict>

dbus (send)
    bus=system
    path=/org/freedesktop/DBus
    interface=org.freedesktop.DBus
    member={Request,Release}Name
    peer=(name=org.freedesktop.DBus, label=unconfined),

# Allow binding the service to the requested connection name
dbus (bind)
    bus=system
    name="org.ofono",

# Allow traffic to/from our path and interface with any method for unconfined
# clients to talk to our ofono services.
dbus (receive, send)
    bus=system
    path=/{,**}
    interface=org.ofono.*
    peer=(label=unconfined),
`

const ofonoConnectedSlotAppArmor = `
# Allow service to interact with connected clients

# Allow traffic to/from our interfaces. The path depends on the modem plugin,
# and is arbitrary.
dbus (receive, send)
    bus=system
    path=/{,**}
    interface=org.ofono.*
    peer=(label=###PLUG_SECURITY_TAGS###),
`

const ofonoConnectedPlugAppArmor = `
# Description: Allow using Ofono service. Reserved because this gives
# privileged access to the Ofono service.

#include <abstractions/dbus-strict>

# Allow all access to ofono services
dbus (receive, send)
    bus=system
    path=/{,**}
    interface=org.ofono.*
    peer=(label=###SLOT_SECURITY_TAGS###),
`

const ofonoConnectedPlugAppArmorClassic = `
# Allow access to the unconfined ofono services on classic.
dbus (receive, send)
    bus=system
    path=/{,**}
    interface=org.ofono.*
    peer=(label=unconfined),
`

const ofonoPermanentSlotSecComp = `
# Description: Allow operating as the ofono service. Reserved because this
# gives privileged access to the system.

# Communicate with DBus, netlink, rild
accept
accept4
bind
getsockopt
listen
recv
recvfrom
recvmmsg
recvmsg
send
sendmmsg
sendmsg
sendto
shutdown
`

const ofonoConnectedPlugSecComp = `
# Description: Allow using ofono service. Reserved because this gives
# privileged access to the ofono service.

# Can communicate with DBus system service
recv
recvmsg
recvfrom
send
sendto
sendmsg
`

const ofonoPermanentSlotDBus = `
<!-- Comes from src/ofono.conf in sources -->

<policy user="root">
  <allow own="org.ofono"/>
  <allow send_destination="org.ofono"/>
  <allow send_interface="org.ofono.SimToolkitAgent"/>
  <allow send_interface="org.ofono.PushNotificationAgent"/>
  <allow send_interface="org.ofono.SmartMessagingAgent"/>
  <allow send_interface="org.ofono.PositioningRequestAgent"/>
  <allow send_interface="org.ofono.HandsfreeAudioAgent"/>
</policy>

<policy context="default">
  <deny send_destination="org.ofono"/>
  <!-- Additional restriction in next line (not in ofono.conf) -->
  <deny own="org.ofono"/>
</policy>
`

const ofonoPermanentSlotUdev = `
## Concatenation of all ofono udev rules (plugins/*.rules in ofono sources)
## Note that ofono uses this for very few modems and that in most cases it finds
## modems by checking directly in code udev events, so changes here will be rare

## plugins/ofono.rules
# do not edit this file, it will be overwritten on update

ACTION!="add|change", GOTO="ofono_end"

# ISI/Phonet drivers
SUBSYSTEM!="net", GOTO="ofono_isi_end"
ATTRS{type}!="820", GOTO="ofono_isi_end"
KERNELS=="gadget", GOTO="ofono_isi_end"

# Nokia N900 modem
SUBSYSTEMS=="hsi", ENV{OFONO_DRIVER}="n900", ENV{OFONO_ISI_ADDRESS}="108"
KERNEL=="phonet*", ENV{OFONO_DRIVER}="n900", ENV{OFONO_ISI_ADDRESS}="108"

# STE u8500
KERNEL=="shrm0", ENV{OFONO_DRIVER}="u8500"

LABEL="ofono_isi_end"

SUBSYSTEM!="usb", GOTO="ofono_end"
ENV{DEVTYPE}!="usb_device", GOTO="ofono_end"

# Ignore fake serial number
ATTRS{serial}=="1234567890ABCDEF", ENV{ID_SERIAL_SHORT}=""

# Nokia CDMA Device
ATTRS{idVendor}=="0421", ATTRS{idProduct}=="023e", ENV{OFONO_DRIVER}="nokiacdma"
ATTRS{idVendor}=="0421", ATTRS{idProduct}=="00b6", ENV{OFONO_DRIVER}="nokiacdma"

# Lenovo H5321gw 0bdb:1926
ATTRS{idVendor}=="0bdb", ATTRS{idProduct}=="1926", ENV{OFONO_DRIVER}="mbm"

LABEL="ofono_end"

## plugins/ofono-speedup.rules
# do not edit this file, it will be overwritten on update

ACTION!="add|change", GOTO="ofono_speedup_end"

SUBSYSTEM!="tty", GOTO="ofono_speedup_end"
KERNEL!="ttyUSB[0-9]*", GOTO="ofono_speedup_end"

# SpeedUp 7300
ATTRS{idVendor}=="1c9e", ATTRS{idProduct}=="9e00", ENV{ID_USB_INTERFACE_NUM}=="00", ENV{OFONO_LABEL}="modem"
ATTRS{idVendor}=="1c9e", ATTRS{idProduct}=="9e00", ENV{ID_USB_INTERFACE_NUM}=="03", ENV{OFONO_LABEL}="aux"

# SpeedUp
ATTRS{idVendor}=="2020", ATTRS{idProduct}=="1005", ENV{ID_USB_INTERFACE_NUM}=="03", ENV{OFONO_LABEL}="modem"
ATTRS{idVendor}=="2020", ATTRS{idProduct}=="1005", ENV{ID_USB_INTERFACE_NUM}=="01", ENV{OFONO_LABEL}="aux"

ATTRS{idVendor}=="2020", ATTRS{idProduct}=="1008", ENV{ID_USB_INTERFACE_NUM}=="03", ENV{OFONO_LABEL}="modem"
ATTRS{idVendor}=="2020", ATTRS{idProduct}=="1008", ENV{ID_USB_INTERFACE_NUM}=="01", ENV{OFONO_LABEL}="aux"

# SpeedUp 9800
ATTRS{idVendor}=="1c9e", ATTRS{idProduct}=="9800", ENV{ID_USB_INTERFACE_NUM}=="01", ENV{OFONO_LABEL}="modem"
ATTRS{idVendor}=="1c9e", ATTRS{idProduct}=="9800", ENV{ID_USB_INTERFACE_NUM}=="02", ENV{OFONO_LABEL}="aux"

# SpeedUp U3501
ATTRS{idVendor}=="1c9e", ATTRS{idProduct}=="9605", ENV{ID_USB_INTERFACE_NUM}=="03", ENV{OFONO_LABEL}="modem"
ATTRS{idVendor}=="1c9e", ATTRS{idProduct}=="9605", ENV{ID_USB_INTERFACE_NUM}=="01", ENV{OFONO_LABEL}="aux"

LABEL="ofono_speedup_end"
`

type OfonoInterface struct{}

func (iface *OfonoInterface) Name() string {
	return "ofono"
}

func (iface *OfonoInterface) PermanentPlugSnippet(plug *interfaces.Plug, securitySystem interfaces.SecuritySystem) ([]byte, error) {
	return nil, nil
}

func (iface *OfonoInterface) ConnectedPlugSnippet(plug *interfaces.Plug, slot *interfaces.Slot, securitySystem interfaces.SecuritySystem) ([]byte, error) {
	switch securitySystem {
	case interfaces.SecurityAppArmor:
		old := []byte("###SLOT_SECURITY_TAGS###")
		new := slotAppLabelExpr(slot)
		snippet := bytes.Replace([]byte(ofonoConnectedPlugAppArmor), old, new, -1)
		if release.OnClassic {
			// Let confined apps access unconfined ofono on classic
			snippet = append(snippet, ofonoConnectedPlugAppArmorClassic...)
		}
		return snippet, nil
	case interfaces.SecuritySecComp:
		return []byte(ofonoConnectedPlugSecComp), nil
	}
	return nil, nil
}

func (iface *OfonoInterface) PermanentSlotSnippet(slot *interfaces.Slot, securitySystem interfaces.SecuritySystem) ([]byte, error) {
	switch securitySystem {
	case interfaces.SecurityAppArmor:
		return []byte(ofonoPermanentSlotAppArmor), nil
	case interfaces.SecuritySecComp:
		return []byte(ofonoPermanentSlotSecComp), nil
	case interfaces.SecurityUDev:
		return []byte(ofonoPermanentSlotUdev), nil
	case interfaces.SecurityDBus:
		return []byte(ofonoPermanentSlotDBus), nil
	}
	return nil, nil
}

func (iface *OfonoInterface) ConnectedSlotSnippet(plug *interfaces.Plug, slot *interfaces.Slot, securitySystem interfaces.SecuritySystem) ([]byte, error) {
	switch securitySystem {
	case interfaces.SecurityAppArmor:
		old := []byte("###PLUG_SECURITY_TAGS###")
		new := plugAppLabelExpr(plug)
		snippet := bytes.Replace([]byte(ofonoConnectedSlotAppArmor), old, new, -1)
		return snippet, nil
	}
	return nil, nil
}

func (iface *OfonoInterface) SanitizePlug(plug *interfaces.Plug) error {
	return nil
}

func (iface *OfonoInterface) SanitizeSlot(slot *interfaces.Slot) error {
	return nil
}

func (iface *OfonoInterface) LegacyAutoConnect() bool {
	return false
}

func (iface *OfonoInterface) AutoConnect(*interfaces.Plug, *interfaces.Slot) bool {
	// allow what declarations allowed
	return true
}