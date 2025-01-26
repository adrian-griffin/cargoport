
A docker compose environment backup & transfer tool, written in Go!
# 🚢 cargoport


Performs backup on target docker compose container and its contents, storing them as a `*.bak.tar.gz` compression file in the location of your choosing, such as on another drive. Optionally, the newly compressed tarfile file can be transferred to a remote machine both securely and reliably when passed flags to do so via SSH TLS encyption and checksum validation for filetransfer. 

So long as SSH keys are set up between the local machine running cargoport and the target remote machine (or only local backups used), cargoport can be called by crontabs to faciliate both local and remote backups on a schedule, being written from the ground up for this purpose.

**⚠️ Please Note:** Cargoport relies on the docker container design being self-encompassing, with  data volumes being mounted, and all config files, such as `DOCKERFILE`, `.env` files, etc., being stored within the same parent directory alongside the `docker-compose.yml`. This is a pretty common setup, but if volumes are mounted directly through the docker volume driver, or if there are volume mounts defined in the docker-compose file that are located elsewhere on the system, these volumes may need to be moved or the composefile moved, otherwise they *will not be included in backups*. This can prove useful when you have external large volume mounts that you do not want backed up alongside the container and its config, critical files, etc.

Recommended Typical Directory Structure Example:

```
/opt/docker/foobar
├── docker-compose.yml
├── data1
│    └── <some-docker-data>
├── data2
│    └── <some-docker-data>
└── .env
```

Crucially, docker services are fully shut down prior to compression and transfer, and the current docker service image digests are stored alongside the `docker-compose.yml` file such that the currently active docker image digests are *always* tracked with each and every container backup to help facilitate restoration in a docker container failure/emergency, especially those regarding updates on images that result in database errors and dockercompose files defined using `:latest` images, as moving to a new machine may cause version mismatches with new image pulls.

The newly compressed file, including the aforementioned image digests, can optionally be transferred to a remote machine securely and reliably using Rsync via the `-remote-send` flag. Cargoport *forces* SSH & TLS transport to ensure security in transit and forces checksum validations during data send & receipt to ensure no data is corrupted or altered during the remote transfer process.

## Install

### Dependencies:

- For initial binary compilation, Go is needed
- Rsync is needed for remote transfer on both nodes
- Tar is needed on the system to allow compression and to restore completed backups

#### go
Cargo should be compiled from raw sourcecode, and as such Go will need to be installed on your machine to build cargoport into an executable binary. 

For more detailed instructions, please visit Go's documentation for installation instructions, here: https://go.dev/doc/install
```shell
# Create go dir and build dirs, wget go version tar.gz
·> cd ~
·> mkdir go && mkdir go/builds/
# This will donwload Go v1.22.5 for Debian machines running AMD64 architecture
·> cd ~/go/builds/ && wget https://go.dev/dl/go1.22.5.linux-amd64.tar.gz

# Clear out any remaining or old Go install files & decompress new content into from /usr/local/go
·> rm -rf /usr/local/go && tar -C /usr/local -xzf go1.22.5.linux-amd64.tar.gz

# Add /usr/local/go/bin to $PATH
# Note: Add to your shell's rcfile to persist
·> export PATH=$PATH:/usr/local/go/bin
```

#### rsync
For remote sending, rysnc is needed on both the local machine and the remote, debian-based instructions:
```shell
·> apt update && apt install rysnc
```

#### tar
For backup compression and decompression when restoring a backup, tar is needed, debian-based instructions:
```shell
·> apt update && apt install tar
```

### Set up CargoPort

#### git clone repo
```shell
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
·> mv /$GITDIR/cargoport /usr/local/bin/
·> sudo cargoport -version # Ensure cargoport is executable from other dirs:
cargoport version: v1.x.x
```




## Example use-cases

Compress a copy of target directory's data, storing it elsewhere locally
```shell
# Compresses `/home/agriffin/foobar` to `/opt/cargoport/local/`
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

### docker examples
**⚠️ Note**: ALL backups will check for a docker-compose file in the target directory, and if found, will ensure that the docker container is stopped entirely & image digests are written to disk before performing compression. Service is restarted after backup completion.

Perform a local-only backup of a docker compose container directory
```shell
# Stops Docker Container operating out of `/opt/docker/service1`, collects image digests, compresses data to store in default backup dir
·> cargoport -target-dir=/opt/docker/service1
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

### Crontab usecase
Please note that crontab backups will require SSH keys between the local and target/remote machine (unless you wanna be around to enter the remote password at runtime in the middle of the night lol)

These work great for nightly docker backups and copy critical docker container data to a remote machine for easy restoration in the event of an emergency, even being self contained enough to be rebuilt on the remote machine in the event the local one no longer exists.
```shell
·> crontab -e
# . . . 
# m h  dom mon dow   command
# . . .
## Perform local backup on docker container every night at 1:00 AM
0 1 * * * /usr/local/bin/cargoport -target-dir=/opt/docker/<dockername>
## Perform remote & local backup on target dockername every Monday at 3:10 AM (defaults to /home/agriffin/vaultwarden.bak.tar.gz on remote)
10 3 * * MON /usr/local/bin/cargoport -docker-name=vaultwarden -remote-host=10.0.0.1 -remote-user=agriffin
```
