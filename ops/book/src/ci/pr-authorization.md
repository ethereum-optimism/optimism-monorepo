# PR Authorization

Jobs running on self-hosted infrastructure will not run on forked PRs. This is a hard limitation of
CircleCI. Additionally, any jobs that use secrets or contexts will not run if the PR's owner is not part of the
`ethereum-optimism` GitHub org. This is a security policy: without these requirements, anyone could open a forked PR
and run untrusted code on our executors, or exfiltrate secrets.

To get around this, forked PRs must be authorized by pushing them to the main repository. We run a bot called
[bailiff][bailiff] to do this.

To use Bailiff, you must be a member of the `engineering` team on GitHub. Then, to authorize a PR, post a comment
that looks like this:

```shell
/ci authorize <full commit hash>

- or -

/ci authorize <full URL to commit>
```

The commit hash is compared with the head of the forked PR. If they don't match, Bailiff will refuse to push the PR.
This prevents malicious users from ninja-pushing to PRs after they've been authorized, and ensures that we know
exactly what is running on our infrastructure. **New commits will have to be authorized again.**

Note: Bailiff does not push to the source branch on the fork. It pushes to its own branch based on a hash of the source
branch. It'll look like `external-fork/<sha256 hash>`. For this reason, **re-running CI jobs on the source branch
will not work.** Instead, re-run the commit on the `external-fork` branch or push a new commit and re-authorize.

Bailiff will post a PR status if it succeeded. That status looks like this:

<img src="assets/bailiff.png">

## Requirements for Authorization

All of the following must be true for a PR to be authorized:

- The authorizing user must be a member of the `engineering` group on GitHub.
- The `/ci` command must be valid. **The commit hash must be the full hash, not a shortened version.** It is
  too easy to spoof commit short hashes.
- The commit hash must match the head of the PR.
- The PR must be on a fork.

## Troubleshooting

If you're having trouble with Bailiff, see below.

1. Check for the PR status. If that exists, bailiff ran and the problem is elsewhere.
2. Check for the existence of an `external-fork` branch. If that exists, bailiff ran and CI results are pending.
3. Wait a few minutes. Sometimes bailiff needs time to process a large queue of authorizations.
4. Ensure your `/ci` command is correct.
5. Check the bailiff logs [here][bailiff-logs], and report the issue in the [#platforms-general][plat-gen] channel on
   Discord.

[bailiff]: https://github.com/ethereum-optimism/bailiff
[bailiff-logs]: https://optimistic.grafana.net/goto/vCCT3AKNg?orgId=1
[plat-gen]: https://discord.com/channels/1244729134312198194/1260624141497798706