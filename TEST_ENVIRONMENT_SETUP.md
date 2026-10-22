Test Environment set-up
=======================

Setting up the environment
--------------------------

### Prepare the environment

Prepare the environment, you need networkd to have a network in the container:

    sudo systemctl enable --now systemd-networkd

You need to enable name resolution for local machines by adding `mymachines` to
`/etc/nsswitch.conf`. See `nss-mymachines(8)`.

### Create the machine

Create a machine for tests (import image and create machine):

    sudo importctl pull-tar -mN https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64-root.tar.xz conductor-tests

    sudo systemd-firstboot --root=/var/lib/machines/conductor-tests --hostname=conductor-tests --prompt-root-password --force

### Start the machine

Disable firewall becauee it can prevent the guest container to correctly connect
to the host (it can prevent DHCP from working correctly). If you use `firewalld`
(Fedora / RedHat), you can configure it like so (check the interface name):

    firewall-cmd --permanent --zone=trusted --change-interface=ve-conductoCgmY

To start manually the machine:

    sudo machinectl start conductor-tests

You can choose to start on boot the machine every time using `machinectl enable
conductor-tests`. The machine will then be always available (but it will also
always take some limites resources).

You can check on the test machine and wait for the boot to complete (you should
see `Ubuntu 24.04.5 LTS conductor-tests pts/0` as last message):

    machinectl status conductor-tests

You can connect to the machine (after it has booted successfully):

    sudo machinectl login conductor-tests

### machine first time setup

Once connected on the machine, you need to perform some basic configuration:

- get your public SSH key:

        cat ~/.ssh/id_ed25519.pub

- copy the key to the authorized hosts on the machine and enable the SSH daemon
  (on the terminal where you did the `machinectl login`):

        echo ssh-ed25519 XXXXXXXXXX username@hostname > ~/.ssh/authorized_keys

        ssh-keygen -A
        systemctl enable --now ssh

- install podman and caddy:

        apt update
        apt install podman caddy

Using the environment
---------------------

Before using the environment, you need to start the machine if you did not
choose to enable it on boot. Use `machinectl start conductor-tests`. Then you
can access the machine with ssh:

    ssh root@conductor-tests
