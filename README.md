# 🚢 cargoport 🌊

A 🐳 Docker compose environment backup & transfer tool, written in Go!

Built bottom-up for use with `cron` on Linux servers & machines.

Automated and easy snapshots of docker-compose environments.

Transfer of backups to remote, off-prem machines, utilizing a built in SSH keytool for simple backups on schedules.

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

Safely halts docker compose containers & snapshots the container's data, environment, & configs, compressing all and storing in a tarball. One of the core design pillars is simple & reliable transfer of snapshots to remote machines, with the intent being that each copy of a container is portable and self-contained. 

✅ Minimal dependencies (just Go & Rsync)

🔐 SSH keytool is built-in. Cargoport handles its own SSH keys, and sharing public keys to remote targets is made easy with the `-copy-key` flag.

🧷 Each and every backup snapshots the images & digests of the docker services, critical information for more robust container disaster-recovery, storing them alongside the `docker-compose.yml`. This is especially helpful when transferring between machines, pulling updates, using the `:latest` tag, etc., as slightly varying docker image versions/hashes can prevent the container from launching properly and leading to data loss.

📅 Cron-compatible by design, allowing both remote & local backup with one command. Ready for hands-off automation!

Cargoport is primarily built for self-hosted, smaller data services. Rolling backups is not supported, and overly large data volumes may pose trouble with the `.tar.gz` compression. 

**Use cases include:**

- Docker services behind reverse proxies can be easily moved from machine to machine, or with failover/HA setups
- Cloning environments for staging/testing
- Creating remote cold, long-term backups 
- Snapshotting or versioning services when making changes


## ⚠️ Limitations

- ❌ Does not support Docker Swarm or Kubernetes
- ❌ No native cloud storage transfer (unless via SSH access)
- ❌ Does not perform live or incremental backups (containers are stopped for consistency)
- ❌ Only works with docker-compose builds, docker run environments are not currently supported

Cargoport relies on the docker container design being self-encompassing, with data volumes and config files being mounted locally in the directory root, alongside the `docker-compose.yml`. This is a pretty common setup for homelab purposes, but please be aware of the limitations.

If your setup uses external volume mounts located elsewhere on the system, or volumes managed by the docker volume drivers, these directories **will not** be included in the backup. *This can prove useful however*, allowing you to exclude large, ephemeral, or non-critical data (such as media libraries, cache folders, etc.)

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

- For initial binary compilation, Go is needed. Prebuilt executables are not provided.
- Rsync is needed on *both* the local node running cargoport, as well as the target machine(s) you want to transfer backups to

Cargoport has been tested on both latest Debian & Arch, and while it should work well on other distros, it has not been fully tested outside of these two, so please do use at your own caution. 

#### go
Cargoport should be compiled from raw source code, and as such Go will need to be installed on your machine to build cargoport into an executable binary. 

These instructions should get ya through it, but for more detailed instructions, please visit Go's documentation for installation instructions, here: https://go.dev/doc/install
```shell
# Create go dir and build dirs for active user
# wget go tar.gz
·> cd ~
·> mkdir go && mkdir go/builds/

# This will download Go v1.25.1 for linux machines running AMD64 architecture
# Please adjust architectures as necessary
·> cd ~/go/builds/ && wget https://go.dev/dl/go1.25.1.linux-amd64.tar.gz

# Clear out any remaining or old Go install files & decompress new content into /usr/local/go
·> rm -rf /usr/local/go && tar -C /usr/local -xzf go1.25.1.linux-amd64.tar.gz

# Add /usr/local/go/bin to $PATH
# Note: Add to your shell's rcfile to persist
·> export PATH=$PATH:/usr/local/go/bin
```
```shell
# check that go is executable
·> go version
go version go1.25.1 linux/amd64
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

#### git clone repo & build into executable binary
```shell
# git clone repo
·> cd ~
·> git clone https://github.com/adrian-griffin/cargoport.git && cd cargoport
# build into executable binary
·> go build ./cmd/cargoport
```

#### add to $PATH (optional)
Using whatever means you'd like, feel free to set the binary up for execution via your PATH to be called from anywhere on the machine, cargoport requires shell elevation/sudo for docker daemon and other filestorage interactions (this is planned to be rewritten with a dedicated user)

basic binary relocation example:
```shell
·> sudo mv cargoport /usr/local/bin/
·> sudo cargoport -version # Ensure cargoport is executable from other dirs:
sudo cargoport  ~  kind words cost nothing
version: v1.x.x
```

#### run setup wizard
Run the setup utility to begin. This root directory will house logs, config, metrics data, and will be the default storage location for outgoing and incoming backup transfers.

In order to utilize the `/var/cargoport/remote` directory during transfers between machines rather than the remote-user's home directory (`/home/$USER/backup.tar.gz`), cargoport will need to be installed on *both* machines and the sending machine's config adjusted.

You will most likely want to allow the setup wizard to create your default local config.yml file

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
0 1 * * * cargoport -target-dir=/opt/docker/<dockername>
## Remote + local backup on target dockername every Monday at 3:10 AM
10 3 * * MON cargoport -docker-name=<dockername> -remote-host=10.0.0.1 -remote-user=agriffin
## Back up container twice per month, one remote copy, on local copy on a staggered schedule
0 2 10 * * cargoport -target-dir=/opt/docker/<dockername> -remote-send-defaults
0 2 25 * * cargoport -target-dir=/opt/docker/<dockername> -tag="staggered"
```

## Extra

### Cargoport /remote directory usage
By default, remote transfers will result in your backupfile being stored in the remote user's home directory, but if you have Cargoport installed on both host machine then you can optionally utilize Cargoport's `/remote` directory by specifying as such in the `config.yml` file and ensuring the intended SSH user on the remote machine has write access to their `/remote` directory:

```shell
> sudo chown someuser:someuser /var/cargoport/remote
```

You will now able to specify using `/var/cargoport/remote` on the remote host for a backup transfer

### Metrics and metrics scraping

A prometheus endpoint can be exposed to allow scraping of basic Cargoport metrics, such as last-job duration, total tarball count, backup storage usage, etc.

This can be set up in many NMSs, such as LibreNMS or Zabbix, to track tarball count, job statistics, etc over time.

It's worth noting, however, that Cargoport only updates its metrics on every job run. Part of the design philosophy is to be a single-execution, one-and-done sysadmin shell script, and I don't want to bloat it by having a daemon or background service built in.

As such, metrics exposing can be handled in one of two ways, depending on preference.

1) After the conclusion of each job, a metrics endpoint can be exposed for a set amount of time. For instance, after each job a Prometheus HTTP endpoint could be exposed for 60s to allow Zabbix to scrape for metrics tracking. The downside here is that graphs-over-time of this data will only show blips of data for 60s at a time, only when a job runs. This can lead to large gaps in the collected data-over-time.

2) Alternatively, a mini-daemon can be stood up to allow perpetual polling of the metrics endpoint. `cargoport -metrics-daemon` will do nothing besides exposing a Prometheus endpoint of metrics and supplying logs. This can be wrapped into a systemd daemon so that data collection can happen 24/7. See the `metrics-daemon-example.service` file for more information.

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
