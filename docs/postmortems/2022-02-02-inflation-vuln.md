# Self-Destruct Inflation Vulnerability Postmortem

This document describes a critical bug in our system which we were alerted to on February 2nd, 2022.
It also details our response, lessons learned, and subsequent changes to our processes.

## Incident Summary

A vulnerability in Optimism’s fork of [Geth](https://github.com/ethereum/go-ethereum) (which we refer to as [L2Geth](https://github.com/ethereum-optimism/optimism-legacy/blob/8205f678b7b4ac4625c2afe351b9c82ffaa2e795/l2geth/README.md)) was reported
to us by [Jay Freeman](https://twitter.com/saurik) (AKA saurik) on February 2nd, 2022. If exploited,
this vulnerability would allow anyone to mint an unbounded amount of ETH on Optimism.

We confirmed the issue, implemented a fix, and upgraded our infrastructure within 3 hours. We then
alerted infrastructure providers running Optimism, as well as other network operators who are using
a fork of our software.

All parties were running a patched version of L2Geth within 32 hours of the initial report.

## Lead up

saurik had been engaging with our code, and
[opening issues](https://github.com/ethereum-optimism/optimism/issues?q=is%3Aissue+author%3Asaurik+)
for several months prior to identifying this bug.

We launched our [Immunefi-hosted bug bounty program](https://immunefi.com/bounty/optimism/) on
January 13th, 2022, slightly more than 2 weeks before his report. The maximum payout for the program
was $2,000,042. According to saurik, his decision to hunt for bugs in our code had two motivations.
One was the financial reward, the other was needing a topic to speak about at the upcoming ETHDenver
conference.

## The Vulnerability

Contract balances were improperly zeroed during self-destruction, so that the contract address would
still have a balance after it had been self-destructed. This could have enabled an attacker to run a
loop which doubled the balance of a contract each time, resulting in massive inflation and issuance
directly to the attacker.

A thorough description can be found in saurik's [writeup](https://www.saurik.com/optimism.html).

## Impact

The issue was not exploited, so there was no impact to ordinary users. However, the issue required
node operators to update as quickly as possible. Infrastructure providers were especially impacted,
since they had to roll out an emergency patch version.

## Detection

Jay Freeman (a.k.a. saurik) reported the bug to us via security@optimism.io. He first attempted to
report via our Immunefi bounty program, but decided to email us since it does not explicitly include
our L2Geth code.

## Recovery

The recovery process was executed by a small subset of the team in a private slack channel. The
timeline and activities were as follows:

### Timeline (UTC)

(Using github handles as identifiers)

- 2022-02-02 1625: smartcontracts receives an e-mail from saurik claiming to have found a critical
  issue in L2Geth. E-mail was sent to security@optimism.io.
- 2022-02-02 X: saurik messaged smartcontracts on Discord to make sure we checked the e-mail since
  he knew we had a prior problem where security advisories went to spam.
- 2022-02-02 1650: Huddle begins in #security on Slack.
- 2022-02-02 1758: tynes and smartcontracts confirm the issue on the huddle.
- 2022-02-02 1812: mslipper joins the huddle and alerts infrastructure providers that there is a
  live security issue and that an emergency update will be required.
- 2022-02-02 1906: tynes cuts the following builds:
  - Mainnet: `0.5.8_b6f79171`
  - Kovan: `0.5.9_d4c6d824`
- 2022-02-20 1930: optimisticben deploys to Kovan and mainnet.
- 2022-02-02 2021: mslipper gives instructions to infra providers on how to upgrade.
- 2022-02-02 2150: Infura upgrades both Kovan and mainnet.
- 2022-02-03 0457: Alchemy upgrades both Kovan and mainnet.
- 2022-02-03 2309: Quicknode upgrades mainnet.
- 2022-02-03 1432: Quicknode upgrades Kovan.
- 2022-02-03 1945: smartcontracts alerts Boba.
- 2022-02-03 2300: Boba patches mainnet.
- 2022-02-03 2300: smartcontracts alerts Metis. They patched mainnet at sometime overnight.
- 2022-02-04 1617: smartcontracts opens
  [PR #2146](https://github.com/ethereum-optimism/optimism/pull/2146), which we will use to sneak in
  the fix without publicly disclosing it.
- 2022-02-06 0250: mslipper merges the finalized patch into PR #2146 after testing, and cuts the
  release of L2Geth version `0.5.11`.

## How it was fixed

The
[fix](https://github.com/ethereum-optimism/optimism/pull/2146/files#diff-20d698ae9b1041792b702bf7015d0beb3cca36701495eaa45b0b8f587b9ae286R887-R889)
itself is only 3 lines long, it ensures that when the `SELFDESTRUCT` operation is called in an
account, its balance (in OVM_ETH) is also immediately set to zero.

## Lessons learned

In this section we outline the lessons learned, and how our processes have changed or will change as
a result. These lessons are the product of an internal retrospective, as well as many informal
discussions which have occurred since the incident.

### On overly-optimistic code reviews

This bug was (obviously) not caught by our code review process. Naturally we want to understand why
that is, by looking at the PR that introduced it, and the organization context of the time.

#### The PR

It was introduced in [PR #1363](https://github.com/ethereum-optimism/optimism/pull/1363), on
2021-07-20, and merged 3 days later. It includes changes to 21 files, (14 in L2Geth code, 6 in test
files). The diff added 217 lines, and removed 149 lines.

The PR was well scoped, and all of the changes were relevant according to its description:

> Refactors the usage of OVM_ETH so we can get most remaining integration tests working again. Also
> reworks `vm.UsingOVM` to be `rcfg.UsingOVM` where `rcfg` is a new package within the rollup
> folder. Was required in order to avoid an import cycle.

The PR was reviewed at least twice, with inline comments that indicate attention to detail, although
the
[comments in the buggy code](https://github.com/ethereum-optimism/optimism/pull/1363/files#diff-11f5b63c52e9c7c30e4e599f96f84db5f08121e8eb623aa1176c2801389487b9)
itself were sparse, and fairly high level.

Notably, the eventual fix to the bug was made in `instructions.go`, a **file which was completely
untouched by the PR**.

#### Code and organizational context

The PR #1363 was one small part of a major architectural update (which we refer as a 'regenesis') to
Optimism. The regenesis removed the OVM contracts, and enabled EVM equivalence.

The total size of the update can be seen in the
[regenesis 0.5.0 PR](https://github.com/ethereum-optimism/optimism/pull/1594/commits), which
included the commits from the PR above. This was a massive upgrade, as we can see from the size of
the PR (36,311 lines added, 47,430 lines removed), which consumed the attention of our entire
engineering team with a sense of urgency for several months.

An additional factor contributing to this bug was the significant complexity of the
[L2Geth](https://github.com/ethereum-optimism/optimism-legacy/blob/8205f678b7b4ac4625c2afe351b9c82ffaa2e795/l2geth) codebase, which is a fork
of [Geth](https://github.com/ethereum/go-ethereum). Geth itself is already a very complex codebase.
The changes introduced to L2Geth in order to support the OVM made it much more complex, such that
very few people properly understood how it worked.

The changes made for this regenesis mostly removed this complexity, and moved the behavior of L2Geth
closer that of Geth. Unfortunately L2Geth had already diverged significantly, and the abstractions
of the OVM leaked in enough that a change made in one part of the code could have major consequences
elsewhere.

More specifically: the OVM used `OVM_ETH`, and ERC20 token rather than native ETH, meaning that
account balances were no longer kept in the state trie. However the EVM's `SELFDESTRUCT` works by
deleting the balance in the trie. In addition, `SELFDESTRUCT` was not implemented in the OVM,
meaning it was not present to remind us that it needed updating in the EVM.

#### No standard Tests

The changes outlined above broke many of the common
[Ethereum tests](https://github.com/ethereum/tests) (though not unexpectedly). Modifying the tests
to work with L2Geth and run in CI would have been a major undertaking, but also would have caught
this bug.

#### Lack of specification

The fix for this bug might also have been identified by putting more thought into the specification
and security risks associated with the change. Doing so would have a reasonable chance of initiating
the following line of reasoning:

1. This impacts the way that balances and value transfers happen in the EVM.
1. Several opcodes refer to balance and value transfer.
1. SELFDESTRUCT involves value transfer.
1. Does SELFDESTRUCT behave the same way after the change?

#### Lack of auditing

We did not have an audit on the changes made to the regenesis. The rationale for this was that:

1. the changes were mostly deleting code and simplifying the system by removing the OVM, and
1. the availability of qualified auditors was extremely constrained.

#### Conclusion regarding the introduction of the bug

Multiple factors contributed to this bug. Firstly, the pre-existing codebase was heavily modified
from an upstream project (Geth) which very few people fully understand. Arguably the author and the
reviewer were the only people who had a proper grasp of the full scope of changes.

Perhaps most importantly, because the actual location of the bug was in a file outside of the PR, it
was not considered. This is an unavoidable reality of working in any codebase of a non-trivial size,
but it is not a problem easily solved simply by "reviewing more carefully".

In order to catch an issue like this, we as reviewers will need to adopt an adversarial mindset, and
we will need a process which enforces this mindset. Such a process would require a reviewer to
explicitly define how an attacker might try to take advantage of a particular change, and to outline
the various risks they considered.

**Actions planned:**

- Our forthcoming network upgrade
  ([Optimism: Bedrock](https://github.com/ethereum-optimism/optimistic-specs)) will use a
  [fresh fork of Geth](https://github.com/ethereum-optimism/op-geth), with a
  minimal set of changes which can be easily rebased to track the upstream Geth repository.
- We will ensure the common Ethereum tests are run against Bedrock.
- We are redesigning our code review process, to introduce measure which will:
  1. encourage authors to
     1. clearly state the motivation and specification for the change
     1. explicitly state the risks considered and the associated mitigations they incorporated
        during development
  1. encourage reviewers to:
     1. consider areas of the system which are not touched by the PR
     1. view the change from the perspective of an adversary
     1. explicitly define the risks and attacks they considered during their review
- We will build out a threat model which can be used by developers, reviewers and auditors.
- We will make it a hard requirement not to deploy high risk code without an audit.

### Maximizing the effectiveness of our bug reporting channels

Our bounty program page on Immunefi did not list L2Geth as in scope, which led saurik to report
through our security@optimism.io email. Additionally, not all members of the team are in the habit
of checking email at the start of their day. This caused some delay in the initial incident response process.

**Actions taken:**

1. We have extended the scope of the Immunefi program to include L2Geth.

**Actions planned:**

1. Ensure that instructions for reporting a vulnerability are easily discoverable on any of our web
   properties, including websites, chat forums, and github repos.
1. We will set up automated alerts for new reports which claim to be critical.
1. We will review who has access to both the email and Immunefi reporting channel, and ensure the
   group is limited to those who need to know.

### Adhering to the principle of least privilege

Early in the process, the existence of the issue was openly discussed in a public slack channel,
although the details of the vulnerability and exploit path were not described. This violates the
[principle of least privilege](https://en.wikipedia.org/wiki/Principle_of_least_privilege), as well
as our already existing incident response protocols

**Action taken:**

Our incident response documentation is now easier to locate. It explicitly prescribes the use of a
private slack channel, and the principle of least privilege in general.

### Communicating with the whitehat

Communication with saurik was initially done mostly in a direct message with a single team member.
This added communication overhead, and reduced saurik's ability to participate in the response
process.

Another lesson came when we received a review of the fix from saurik, who was able to suggest a
better approach. Consulting with saurik on the fix before implementing would have saved time.

Keeping the whitehat better informed should also help to build trust with them.

**Action taken:**

Our incident response process now requires establishing a private channel with the whitehat and the
full response team, as well as keeping them up to date as the situation progresses.

### Disclosing to infrastructure operators and forks

The distribution of patched code to infrastructure operators and forks went relatively smoothly,
still there are opportunities to better document the proper process internally.

**Actions taken:**

- We have established an internal database of users to be notified.

**Actions planned:**

- We will create internal documentation for building a patched client for infrastructure operators
  with a non-standard build target.

### Public disclosure

Moving forward, we will adopt a process similar to the Geth team’s
[silent patch policy](https://geth.ethereum.org/docs/developers/geth-developer/disclosures#why-silent-patches).

This means that we reserve the right to hide the fix, and delay the public announcement. We also
reserve the right to directly notify a subset of downstream users prior to the public announcement.

**Action taken:** This disclosure process is now documented in our
[Security Policies page](https://github.com/ethereum-optimism/.github/blob/master/SECURITY.md).

### Defensive measures during an incident

We were fortunate to be informed of this vulnerability without it having been exploited. However
this incident has also revealed that we do not have a clear criteria for deciding whether or not to
disable the sequencer, or pause smart contracts.

Ultimately this will be a decision made in the moment with the full available context. Although it
is not possible to anticipate all scenarios we outline some basic criteria to inform the decision.

**Action taken:**

We've established guiding criteria for disabling the system:

- If an attack is ongoing: we should disable or pause in order to prevent further damage.
- If we suspect that a vulnerability might be widely known: we should disable or pause proactively.
- Otherwise: we should not disable or pause the system.

### Alerting

We would not have automatically detected this bug if it had been exploited.

**Action planned:** We will expand the set of monitoring and alerting checks we run on the system,
so that we will be alerted to events such as this.
