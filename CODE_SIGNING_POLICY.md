# Code signing policy

> **Status: Not yet active.** This policy is in place for when code signing is enabled. SatisFacts is currently distributed as an unsigned executable. See the [SignPath.io](https://signpath.io) open-source program for details.

Free code signing will be provided by [SignPath.io](https://signpath.io), certificate by [SignPath Foundation](https://signpath.org).

## What is signed

- Windows executable packages (`.exe`) published on [GitHub Releases](https://github.com/PhysoxNB/SatisFacts/releases)

## Build and signing process

- Artifacts are built from this repository using GitHub Actions CI.
- Only CI-built artifacts are submitted to SignPath for signing.
- The private key is held by SignPath (HSM-backed). This project does not store or have access to the private key.

## Team roles

- **Committers and reviewers:** [PhysoxNB](https://github.com/PhysoxNB)
- **Approvers:** [PhysoxNB](https://github.com/PhysoxNB)

As the sole maintainer of SatisFacts, PhysoxNB is responsible for
source code changes, code review, and approving releases for signing.

## Privacy

SatisFacts will not transfer any information to other networked systems
unless specifically requested by the user or the person installing or
operating it.
