# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

## [0.87.60] - 2025-1-25
- Adjust custom local backup output dir logic

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
- Adjusted backup storage paths (uses /opt/cargoport/ as root dir), docker digest file adjustments
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
