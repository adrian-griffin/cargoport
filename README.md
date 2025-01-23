# 🚢 cargoport

Performs backup on target docker compose container and its contents, storing them as a `*.bak.tar.gz` compression file to save on disk space. Optionally, the newly compressed `.tar.gz` file can be copied securely to a remote machine when passed flags to do so. So long as SSH keys are set up between the local machine running cargoport and the target remote machine, (or only local backups used) cargoport can be called by crontabs to faciliate both local and remote backups on a nightly schedule.

cargoport is primarily designed to perform backups on docker containers defined via `docker compose` *with relative volume mounts in the working directory of the `docker-compose.yml` file*, cargoport can also perform the same style of local and remote backups on regular everyday file directories as well. Unfortunately cargoport cannot support backups for docker volume mounts other than `.` without some extra effort. Passing the `-docker` flag at command runtime will allow cargoport to perform `docker compose down` and `docker compose up -d` commands from your docker container's working directories before and after performing backups to ensure data consistency and safety.

Crucially, prior to shutting down a docker compose service and performing a compressive backup, cargoport will also collect the current docker image digests from the docker services and store them alongside the `docker-compose.yml` file such that the current image digests are *always* tracked with each backup at shutdown to help facilitate restoration in a docker container failure/emergency, especially those regarding updates on images that result in DB errors.

The newly compressed file, including the aforementioned image digests, can optionally be transferred to a remote machine securely and reliably via Rsync using the `-remote-send` flag. cargoport forces SSH transport to ensure security in transit and forces checksum validations during receipt to ensure no data is corrupted or altered during the remote transfer process. 

## Install

### Dependencies:

- For initial binary compilation, Go is needed
- Rsync is needed for remote transfer on either node

#### go
Raw source, Go will need to be installed on your machine to build cargoport into a binary on your machine. Please visit Go's documentation for installation instructions, here: https://go.dev/doc/install

#### rsync
For remote sending, rysnc is needed on both the local machine and the remote, debian-based instructions:
```shell
·> apt update && apt install rysnc
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
using whatever means you'd like, feel free to get the binary into your path to be called from anywhere on the machine

basic binary relocation example:
```shell
·> mv /$GITDIR/cargoport /usr/local/bin/
# Ensure cargoport is executable from other dirs:
·> cargoport -version
cargoport version: v1.x.x
```

## Example use-cases

Compress a copy of a target directories data, storing it in the defined backup directory
```shell
## Compresses /$ROOTPATH/$TARGETNAME/ to defined backup directory
·> ./cargoport -target=foo
```

Stop target docker container, perform a backup of its current image digests and stored data, and restart it
```shell
## Stops $TARGETNAME docker container, collects image digests, compresses data to store in backup dir, and restarts container in background
·> ./cargoport -target=foobar -docker
```

Backup docker container, storing copy of the backup and img digests both Locally and on a Remote backup server

Note that in order for cargoport to be used with Crontabs, an SSH key must be utilized! Otherwise, remote password can be passed at runtime.
```shell
## Backs up docker container's data and copies it to remote machine
·> ./cargoport \
  -target=foobar \
  -docker \
  -remote-send=true \
  -remote-user=admin \
  -remote-host=192.168.0.1  
```

Perform compression & secure transfer for regular (non Docker), specific target directory, sending to specified remote path on remote machine. Skips storage of directory backup on local machine.
```shell
### Skip local backup, and compress & copy non-docker directory to remote destination path
·> sudo ./cargoport 
-target=foo \ 
-skip-local \ 
-docker=false \
-remote-send \ 
-remote-host 192.168.0.1 \
-src-root=/etc/foo \ 
-remote-file=/path/to/dst/foo.bak.tar.gz \ 
-remote-user=agriffin 

```

### Crontab usecase
Again, please note that crontab backups will require SSH keys between the local and target/remote machine (unless you wanna be around to enter the remote password at runtime in the middle of the night lol)

These work great for nightly docker backups and copy critical docker container data to a remote machine for easy restoration in the event of an emergency, even being self contained enough to be rebuilt on the remote machine in the event the local one no longer exists.
```shell
·> crontab -e
# . . . 
# m h  dom mon dow   command

## Perform local backup on docker container every night at 1:00
0 1 * * * /usr/local/bin/cargoport -target=foobar -docker
```
