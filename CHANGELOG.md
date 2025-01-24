# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

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
