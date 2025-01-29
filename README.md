
A docker compose environment backup & transfer tool, written in Go!


## Table of Contents
- [Install](#install)
  - [Dependencies](#dependencies)
  - [Set up CargoPort](#set-up-cargoport)
- [Usage Examples](#usage-examples)
  - [Docker examples](#docker-examples)
  - [Crontab Usage](#crontab-usage)
- [Restoring a Cargoport Backup](#restoring-a-cargoport-backup)

---

# 🚢 cargoport

Performs backup on target docker compose container's data, environment, & configs, storing them within a tar compression backupfile. Cargoport can copy the new backup to a remote machine for off-prem storage both securely & reliably utilizing SSH+TLS and data checksum validations. Cargoport is built from the ground up to be utilized with crontabs to allow scheduled backups, and as such SSH keys are forced (by default) for transfers to ensure reliable backups off-hours & on a schedule.  

To help manage the SSH key aspect, Cargoport comes equipped with an ssh keytool that manages its own SSH keys for backups, and can easily share its public keys to remote target machines using the `-copy-key` flag. 

**⚠️ Please Note:** Cargoport relies on the docker container design being self-encompassing, with  data volumes being mounted, and all config files, such as `DOCKERFILE`, `.env` files, etc., being stored within the same parent directory alongside the `docker-compose.yml`. This is a pretty common setup, but if volumes are mounted directly through the docker volume driver, or if there are volume mounts defined in the docker-compose file that are located elsewhere on the system than the directory housing the docker-compose file, these volumes may need to be moved or the composefile moved, otherwise they *will not be included in backups*. This can prove useful, however, when you have external large volume mounts that you do not want backed up alongside the container critical data and its config.

Recommended Typical Directory Structure Example:

```
/srv/docker/foobar
├── docker-compose.yml
├── data1
│    └── <some-docker-data>
├── data2
│    └── <some-docker-data>
└── .env
```

Crucially, docker services are fully shut down prior to compression and transfer, and the current docker service image digests are stored alongside the `docker-compose.yml` file such that the currently active docker image digests are *always* tracked with each and every container backup to help facilitate restoration in a docker container failure/emergency, especially those regarding updates on images that result in database errors and dockercompose files defined using `:latest` images, as moving to a new machine may cause version mismatches with new image pulls.

The newly compressed file, including the aforementioned image digests, can optionally be transferred to a remote machine securely and reliably. Cargoport *forces* SSH & TLS transport to ensure security in transit and forces checksum validations during data send & receipt to ensure no data is corrupted or altered during the remote transfer process.

---

## Install

### Dependencies:

- For initial binary compilation, Go is needed
- Rsync is needed for remote transfer on both nodes
- Tar is needed on the system to allow compression and to restore completed backups

#### go
Cargoport should be compiled from raw sourcecode, and as such Go will need to be installed on your machine to build into an executable binary. 

For more detailed instructions, please visit Go's documentation for installation instructions, here: https://go.dev/doc/install
```shell
# Create go dir and build dirs, wget go version tar.gz
·> cd ~
·> mkdir go && mkdir go/builds/
# This will download Go v1.22.5 for Debian machines running AMD64 architecture
# Please adjust as necessary
·> cd ~/go/builds/ && wget https://go.dev/dl/go1.22.5.linux-amd64.tar.gz

# Clear out any remaining or old Go install files & decompress new content into /usr/local/go
·> rm -rf /usr/local/go && tar -C /usr/local -xzf go1.22.5.linux-amd64.tar.gz

# Add /usr/local/go/bin to $PATH
# Note: Add to your shell's rcfile to persist
·> export PATH=$PATH:/usr/local/go/bin
```
```shell
# check that go is executable
·> go version
go version go1.22.5 linux/amd64
```

#### rsync
For remote sending, rsync is needed on both the local machine and the remote, debian-based instructions:
```shell
·> apt update && apt install rsync
```

#### tar
For backup compression and decompression when restoring a backup, tar is needed, debian-based instructions:
```shell
·> apt update && apt install tar
```

### Set up CargoPort

#### git clone repo
```shell
·> cd ~
·> git clone https://github.com/adrian-griffin/cargoport.git && cd cargoport
```

#### build into binary
```shell
·> go build cargoport.go
```

#### add to $PATH (optional)
using whatever means you'd like, feel free to set the binary up for execution via your PATH to be called from anywhere on the machine, cargoport requires shell elevation/sudo for docker daemon and other filestorage interactions (this is planned to be rewritten eventually)

basic binary relocation example:
```shell
·> sudo mv cargoport /usr/local/bin/
·> sudo cargoport -version # Ensure cargoport is executable from other dirs:
cargoport  ~  kind words cost nothing
version: v0.88.20
```

#### run setup wizard
Run the setup utility to begin. This root directory will house logs, config, and be the default storage location for outgoing and incoming backup transfers

In order to utilize the `/var/cargoport/remote` directory during transfers between machines, `cargoport -setup` should be run on both machines; otherwise a manual valid path must be passed using the `-remote-dir` flag instead

Typically you will want to allow the setup wizard to create your default local config.yml file

```shell
·> cargoport -setup  
Welcome to cargoport initial setup . . .
 
Enter the root directory for Cargoport (default: /var/cargoport/): 
Using root directory: /var/cargoport

. . . 

No config.yml found in /var/cargoport. Would you like to create one? (y/n): y
Default config.yml created at /var/cargoport/config.yml.
```

---

## Usage Examples

Copy SSH Key to remote machine
```shell
# You will be prompted to log in via password on the remote to transfer the key
·> cargoport -copy-key -remote-host=10.115.0.1 -remote-user=agriffin  
```

Compress a copy of target directory's data, storing it elsewhere locally
```shell
# Compresses `/home/agriffin/foobar/` to `/$CARGOPORT/local/foorbar.bak.tar.gz`
·> cargoport -target-dir=/home/agriffin/foobar
```

Compress a copy of target directory's data, storing it locally on another drive & transferring a copy of the backup to a remote machine
```shell
·> cargoport \
  -target-dir=/home/agriffin/foobar \
  -remote-user=agriffin \
  -remote-host=192.168.0.1 \
  -output-dir=/mnt/external-drive/cargoport
```

Create backup and send to remote host using defaults defined in config.yml file, skip saving to local disk
```shell
·> cargoport -target-dir=/path/to/dir -remote-send-defaults -skip-local
```

### docker examples
**⚠️ Note**: ALL backups will check for a docker-compose file in the target directory, and if found, will ensure that the docker container is stopped entirely & image digests are written to disk before performing compression. Service is restarted after backup completion.

Perform a local-only backup of a docker compose container directory
```shell
# Stops Docker Container operating out of `/srv/docker/service1`, collects image digests, compresses data to store in default backup dir
·> cargoport -target-dir=/srv/docker/service1
```

Perform backup of a docker container based on docker container's name

The db service here being an example, because ALL services defined in the composefile associated with this target container will be restarted & backed up
i.e: `container-name` will be backed up, including its associated parts, such as `container-name-db`, `container-name-web`, `container-name-etc`
```shell
# Performs backup on docker container operating out of the directory associated with target $docker-name
·> cargoport -docker-name=<container-name-db> 
```


Backup based on container name, remote send to another machine
```shell
# Remote & local backup for a docker compose setup running a container named 'vaultwarden'
·> cargoport
-docker-name=vaultwarden \ 
-remote-host=10.115.0.1 \
-remote-user=agriffin
```

### Crontab usage
Please note that crontab backups will require SSH keys between the local and target/remote machine (unless you wanna be around to enter the remote password at runtime in the middle of the night lol)

These work great for nightly docker backups and copy critical docker container data to a remote machine for easy restoration in the event of an emergency, even being self contained enough to be rebuilt on the remote machine in the event the local one no longer exists.
```shell
·> crontab -e
# . . . 
# m h  dom mon dow   command
# . . .
## Perform local backup on docker container every night at 1:00 AM
0 1 * * * /usr/local/bin/cargoport -target-dir=/srv/docker/<dockername>
## Perform remote & local backup on target dockername every Monday at 3:10 AM (defaults to /home/agriffin/vaultwarden.bak.tar.gz on remote)
10 3 * * MON /usr/local/bin/cargoport -docker-name=vaultwarden -remote-host=10.0.0.1 -remote-user=agriffin
```

### Extra

To view the saved filesizes of your storage paths and the new backup file, use `du -sh`
```shell
> sudo du -sh /opt/docker/Joplin 
1.1G    /opt/docker/Joplin

> sudo du -sh /var/cargoport/local/Joplin.bak.tar.gz 
410M    /var/cargoport/local/Joplin.bak.tar.gz
```
---

# Restoring a Cargoport Backup 

First decompress file contents:
```shell
·> cd /var/cargoport/local/ && ls
Container1.bak.tar.gz  Container2.bak.tar.gz Vaultwarden.bak.tar.gz
# decompress target tarball
·> sudo tar -xzvf Vaultwarden.bak.tar.gz && ls
. . . 
#                                             output
Container1.bak.tar.gz  Container2.bak.tar.gz Vaultwarden Vaultwarden.bak.tar.gz
```

#### Docker specific restoration:

If the newly output directory contained `docker-compose.yml` file during former backup process, then image digests will be stored within:
```shell
·> cat Vaultwarden/compose-img-digests.txt 
Image: <image-id> Digest: vaultwarden/server@sha256:<image-digest>
```

Oftentimes, activating the docker compose container from this point will do the trick, but if you run into any sort of image version issues (such as ones caused by using the `:latest` tag, compose file adjustments, etc), you can statically set the index digest in your composefile and perform a `docker compose up` once again:
```shell
·> vim docker-compose.yml
#! ./Vaultwarden/docker-compose.yml
services:
  vaultwarden:
    image: vaultwarden/server@sha256:<image-digest>

    ## MACVLAN networking
    #  . . . 
    # EOF
```

This will ensure that the exact image formerly used pre-backup is pulled to the machine when the docker compose container is spun back up.
