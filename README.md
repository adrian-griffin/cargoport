# 🚢 cargoport 🌊

A 🐳 Docker compose environment backup & transfer tool, written in Go!

Built from the ground up for seamless use with `cron` on Linux servers & machines.

Allows easy transfer of backups to remote, off-prem machines, utilizing a built in SSH keytool for simple & hands-off backups on schedules.

## Table of Contents
- [Install](#install)
  - [Dependencies](#dependencies)
  - [Set up CargoPort](#set-up-cargoport)
- [Usage Examples](#usage-examples)
  - [Docker examples](#docker-examples)
  - [Crontab Usage](#crontab-usage)
- [Restoring a Cargoport Backup](#restoring-a-cargoport-backup)

---

## ✨ Features

Handles docker service halting & backups of compose container's data, environment, & configs, storing all data in a `.tar.gz` archive by default. One of Cargoport's core design pillars is to enable easy and reliable transfer of backup data to remote machines, with the intent being that copies of containers are portable and self-contained. 

✅ Minimal dependencies (just Go & Rsync)

🔐 SSH keytool is built-in. Cargoport handles its own SSH keys, and sharing public keys to remote targets is made easy with the `-copy-key` flag.

🧷 Each and every backup snapshots the images & digests of the docker services, storing them alongside the `docker-compose.yml` file for easier and more reliable restoration (especially helpful when transferring between machines, pulling updates, using the `:latest` tag, etc.). 

📅 Cron-compatible by design, allowing both remote & local backup with one command. Ready for hands-off automation!


**Use cases include:**

- Docker services behind reverse proxies can be easily moved from machine to machine, or with failover/HA setups
- Cloning environments for staging/testing
- Creating remote cold backups 
- Snapshotting or versioning services


## ⚠️ Limitations

- ❌ Does not support Docker Swarm or Kubernetes
- ❌ No native cloud storage transfer (unless via SSH access)
- ❌ Does not perform live or incremental backups (containers are stopped for consistency)
- ❌ Only works with docker compose builds, docker run environments are not currently supported

Cargoport relies on the docker container design being self-encompassing, with data volumes and config files being mounted locally, stored within the same parent directory alongside the `docker-compose.yml`. This is a pretty common setup, but please be aware of the limitations.

If your setup uses external volume mounts located elsewhere on the system, or volumes managed by the docker volume drivers, these directories **will not** be included in the backup. This can prove useful however, allowing you to exclude large, ephemeral, or non-critical data (like media libraries, cache folders, etc.)

### Recommended Directory Layout 📁
```shell
/srv/docker/foobar
├── docker-compose.yml
├── data1
│    └── <some-docker-data>
├── data2
│    └── <some-docker-data>
└── .env
```

## ⚙️ Install

### Dependencies:

- For initial binary compilation, Go is needed
- Rsync is needed on both the local node running cargoport, as well as the target machine you want to share backups with

Cargoport has been tested on both latest Debian & Arch, and while it should work well on other distros, it has not been fully tested outside of these two, so please do use at your own caution. 

#### go
Cargoport should be compiled from raw source code, and as such Go will need to be installed on your machine to build cargoport into an executable binary. 

These instructions should get ya through it, but for more detailed instructions, please visit Go's documentation for installation instructions, here: https://go.dev/doc/install
```shell
# Create go dir and build dirs for active user
# wget go tar.gz
·> cd ~
·> mkdir go && mkdir go/builds/

# This will download Go v1.22.5 for linux machines running AMD64 architecture
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
For remote sending, rsync is needed on both the local machine and the remote:
```shell
# debian-based distro
·> sudo apt update && sudo apt install rsync

# arch-based distro
·> sudo pacman -Syu && sudo pacman -Sy rsync
```

### Set up CargoPort

Note that it is recommended to install Cargoport on each machine that you plan to manage/transfer backups between!

#### git clone repo & build into executable binary
```shell
# git clone repo
·> cd ~
·> git clone https://github.com/adrian-griffin/cargoport.git && cd cargoport
# build into executable binary
·> go build ./cmd/cargoport
```

#### add to $PATH (optional)
Using whatever means you'd like, feel free to set the binary up for execution via your PATH to be called from anywhere on the machine, cargoport requires shell elevation/sudo for docker daemon and other filestorage interactions (this is planned to be rewritten)

basic binary relocation example:
```shell
·> sudo mv cargoport /usr/local/bin/
·> sudo cargoport -version # Ensure cargoport is executable from other dirs:
sudo cargoport  ~  kind words cost nothing
version: v1.x.x
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

# Usage Examples

Copy SSH Key to remote machine
```shell
# You will be prompted to log in via password on the remote to transfer the key
·> cargoport -copy-key -remote-host=10.115.0.1 -remote-user=agriffin  
```

Compress a copy of target directory's data, storing it in the default local backup location
```shell
# Compresses `/home/agriffin/foobar/` to `/$CARGOPORT/local/foorbar.bak.tar.gz`
·> cargoport -target-dir=/home/agriffin/foobar -tag="identifying-text"
```

Perform backup on target directory, storing in a custom path locally, as well as remote transferring the backup to a remote machine
```shell
·> cargoport -target-dir=/home/agriffin/foobar -remote-user=agriffin -remote-host=192.168.0.1 -output-dir=/mnt/external-drive/cargoport
```

Create backup and send to remote host using defaults defined in config.yml file, skip saving to local disk
```shell
·> cargoport -target-dir=/path/to/dir -remote-send-defaults -skip-local
```

## docker examples

**✅ Note**: All backups will check for a docker-compose file in the target directory, and if found, will ensure that the docker container is stopped entirely & image digests are written to disk before performing compression. Service is restarted after backup completion by default.

Docker containers can be stopped by passing the path to the directory they are hosted from within, or by specifying the name of a docker service that is running

Perform a local-only backup of a target directory housing a docker compose environment
```shell
# Stops Docker Container operating out of `/srv/docker/service1`, collects image digests, compresses data to store in default backup dir
·> cargoport -target-dir=/srv/docker/service1
```

Perform backup of a docker container based on docker service name

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

## Crontab usage
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

## Extra

### Cargoport /remote directory usage
By default, remote transfers will result in your backupfile being stored in the remote user's home directory, but if you have Cargoport installed on both host machine then you can optionally utilize Cargoport's `/remote` directory by specifying as such in the `config.yml` file and ensuring the intended SSH user on the remote machine has write access to their `/remote` directory:

```shell
> sudo chown someuser:someuser /var/cargoport/remote
```

You will now able to specify using `/var/cargoport/remote` on the remote host for a backup transfer


### Viewing output filesizes or Cargoport total directory volume

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
#                                              output
Container1.bak.tar.gz  Container2.bak.tar.gz Vaultwarden Vaultwarden.bak.tar.gz
```

#### Docker specific restoration:

If the newly output directory contained `docker-compose.yml` file during former backup process, then image digests will be stored within:
```shell
·> cat Vaultwarden/compose-img-digests.txt 
Image ID: <image-id>  |  Image Digest: vaultwarden/server@sha256:<image-digest>
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
