This is a backup utility for an Incus server (https://linuxcontainers.org/incus/).

It is intended to run on the Incus host and product backups that can be easily restored if needed in the future.

See here for information on how to backup Incus: https://linuxcontainers.org/incus/docs/main/backup/

This is intended to be a wrapper around existing Incus utilities to automate the process for ease-of-use.

# BACKUP LOCATION

A backup target must be configured.  
To start with, we will use a regular directory, but we will likely want to consider something like Restic in the future.

# USAGE (backup and restore all)

## Create a new backup of everything
`incus-backup backup --all --dir=/mnt/nas/sysbackup/incus`

## Restore the latest backup of everything
`incus-restore restore --all --dir=/mnt/nas/sysbackup/incus`

## Partial backups

What gets backed up?

- instances
- volumes
- images (optionally)

- networks - from database backup
- profiles - from database backup

## Volumes/instances/images can be backed up individually
`incus-backup backup --instances --dir=/mnt/nas/sysbackup/incus`

## The database itself to get networks/profiles
`incus-backup backup --database --dir=/mnt/nas/sysbackup/incus`

## Listing backups
### List all backups of all types
`incus-backup list --dir=/mnt/nas/sysbackup/incus`

## Listing backups of a type
`incus-backup list --instances --dir=/mnt/nas/sysbackup/incus`

## Restoring a specific backup to a specific instance
`incus-backup restore --instance --name=docker01 --version=20250914`

# Requirements

When a backup is being restored, any destructive operation should require explicit confirmation or override by the user.  If confirmed, this should always proceed.

For example:
`incus-backup restore --volume --name=dockge-data --version=20250914`

If the dockge-data volume already exists, the application should prompt the user for confirmation.  If confirmed, it should then completely replace the existing contents of that volume with the new contents.

If that means stopped some instances to allow replacement of the volume, that should also happen.  The confirmation should tell the user that so they are aware.

A `--force` flag should be available to do it anyway without confirmation.

# Decision to be made

- What tech stack do we use?
- How do we setup an Incus environment so we can write automated tests safely?
- Need to refine the CLI syntax for consistency and best-practices

# Non-functional requirements

- All functionality should have tests.
- We want actual integration tests that really call incus
- We need these tests to have NO IMPACT on the host machine they are running on
- They need to be fast (ideally can run in seconds)

