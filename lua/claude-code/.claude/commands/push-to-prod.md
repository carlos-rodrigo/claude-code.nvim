---
description: Push changes to production with smart git operations
allowed-tools:
  - bash
  - read
  - write
  - grep
---

# Push to Production

This command will help you push your changes to production by:
1. Checking git status
2. Committing changes with a descriptive message
3. Pushing to the current branch
4. Creating a pull request if needed

## Usage:
- `/push-to-prod` - Standard push with commit
- `/push-to-prod release` - Also create a release tag
- `/push-to-prod "custom message"` - Use custom commit message

## Instructions:

First, check the current git status and branch:
!git status
!git branch --show-current

Based on the status, perform the following:

1. If there are uncommitted changes:
   - Stage all changes: `git add .`
   - Create a commit with a descriptive message
   - If $ARGUMENTS contains "release", determine the next version number

2. Push changes:
   - Push to origin
   - If not on main/master, offer to create a pull request

3. If $ARGUMENTS contains "release":
   - Create an annotated tag
   - Push the tag
   - Update CHANGELOG.md if it exists

Arguments provided: $ARGUMENTS

Please proceed with the git operations, ensuring all commit messages follow conventional commit format and include proper attribution.
