# 🚢 cargoport

Performs backup on target docker compose container and its contents, storing them as a `*.bak.tar.gz` compression file to save on disk space. Optionally, the newly compressed tarfile file can be copier to a remote machine securely and reliably when passed flags to do so. 

So long as SSH keys are set up between the local machine running cargoport and the target remote machine, (or only local backups used) cargoport can be called by crontabs to faciliate both local and remote backups on a schedule.

Cargoport is primarily designed to perform backups on docker containers defined via `docker compose` *with relative volume mounts in the working directory of the `docker-compose.yml` file*, cargoport can also perform the same style of local and remote backups on regular everyday file directories as well. Unfortunately cargoport cannot support backups for docker volume mounts other than `.` without some extra effort. Cargoport will dynamically determine if the target directory containers a `docker-compose.yml` file and will perform various docker tasks, such as  `docker compose down` and `docker compose up -d` before and after performing backups to ensure data consistency and safety.

**⚠️ Please Note:** Cargoport relies on the docker container design being self-encompassing, with  data volumes being mounted, and all config files, such as `DOCKERFILE`, `.env` files, etc., all being stored within the same parent directory alongside the `docker-compose.yml`. This is a pretty common setup, but if external volumes are mounted through the docker volume driver itself, or if there are volume mounts defined in the docker-compose file that are located elsewhere on the system, volumes may need to be moved and remapped. Directory Structure Example:

```
/opt/docker/foobar
├── docker-compose.yml
├── data1
│    └── <some-docker-data>
├── data2
│    └── <some-docker-data>
└── .env
```

Crucially, prior to shutting down a docker compose service and performing a compressive backup, cargoport will also collect the current docker image digests from the docker services and store them alongside the `docker-compose.yml` file such that the current;y used docker image digests are *always* tracked with each backup at shutdown to help facilitate restoration in a docker container failure/emergency, especially those regarding updates on images that result in DB errors and dockercompose files defined using `:latest` images as moving to a new machine may cause version mismatches with new pulls.

The newly compressed file, including the aforementioned image digests, can optionally be transferred to a remote machine securely and reliably using Rsync via the `-remote-send` flag. Noteably, cargoport *forces* SSH transport to ensure security in transit and forces checksum validations during receipt to ensure no data is corrupted or altered during the remote transfer process. 

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
using whatever means you'd like, feel free to set the binary up for execution via your PATH to be called from anywhere on the machine, cargoport requires shell elevation/sudo for docker daemon and other filestorage interactions (this is planned to be rewritten eventually)

basic binary relocation example:
```shell
·> mv /$GITDIR/cargoport /usr/local/bin/
·> sudo cargoport -version # Ensure cargoport is executable from other dirs:
cargoport version: v1.x.x
```

## Example use-cases

Compress a copy of a target directory's data, storing it elsewhere locally
```shell
# Compresses `/home/agriffin/foobar` to `/opt/cargoport/local/`
·> cargoport -target-dir=/home/agriffin/foobar
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
# . . .
## Perform local backup on docker container every night at 1:00
0 1 * * * /usr/local/bin/cargoport -target=foobar -docker
```
