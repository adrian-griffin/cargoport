# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [0.88.37] - 2025-2-28
- Added bool flag for skipping docker container restart after backup

## [0.88.36] - 2025-1-31
- Separated check ssh & check icmp
- Added net handler package
- Rsync call adjustments
- Remote flag validation improvements
- Minor validation & bugfixes for SSH handling

## [0.88.33] - 2025-1-29
- Docker image bugfix with not writing img digests to backup if container was down at backup execution
- Skip Local bugfixes
- Fixed bugs with Docker container not being restarted when fatalError during backup or transfer jobs
- Custom output dir bugfixes
- Integrated gzip & tar functionality writers
- Auto-copy ssh keys on remote transfer

## [0.88.30] - 2025-1-28
- Refactored all code into packages
- Major logic adjustments all around

## [0.88.23] - 2025-1-28
- SSH Keytool added
- SSH Key copy tool added

## [0.88.20] - 2025-1-26
- Incorporated setuptool
- Added configfile
- Large logic overhauls for most docker-related functions
- Large overauls for all remote send logic and validations
- More robust environment setup, init environment flag 
- Generates default config (hardcoded in source) if none is found at set root directory
- Revamped all logging
- Message of the day implementation
- Help flag total rework
- Incorporated backup job duration time to completion output

## [0.87.61] - 2025-1-25
- Adjust custom local backup output dir logic
- New default storage directories for remote and local, new log output file along with other cargoport files
- Smarter logic for remote send output dir/destination 

## [0.87.57] - 2025-1-25
- Overhaulinged Docker logic, adding additional flags for docker logic regarding restarting container or not
- Fixed issue with not performing backup when target directory's docker container was not in running state
- Adjusted all remote send logic and flags, remote send mode enabled when remote-host is passed now, rather than via separate flag
- Remote host ipv6 & ipv6 validations incorporated
- Directory input & handling sanitations and cleanup

## [0.87.52] - 2025-1-24
- Fixed issue with backup completion success confirmation log notices not showing intended dir
- Adjusted logic for flag processing, container restarts, and local cargoport dir creation 
- Overhauls of target paths, docker compose file detection, compression calls

## [0.87.50] - 2025-1-23
- Reworked majority of target & path logic
- Adjusted backup storage paths (uses /var/cargoport/ as root dir), docker digest file adjustments
- Overhauled docker container location & detection logic, docker mode can now be dynamically enabled by cargoport using docker-compose.yml searching features
- Now supports locating of docker-compose.yml by docker container name, such as `-docker-name=<container-name>` to locate container & perform docker tasks

## [v0.87.1] - 2025-1-22
- Branding adjustments & general tidyup
- Basic logging implemented
- Implemented versioning & flags

## [v0.86.3] - 2024-11-10
- Fixed further syntax and minor inconsistencies
- Cargoport readme doc updated to latest
- Added option to forego local backups
- Added support for custom backup and target directories

## [v0.85.2] - 2024-11-10
- Fixed typo in `checkDockerRunState` that prevented compilation
- Declared `err` at beginning of main func

### Added

- Init versioning of `cargoport`.
