# Signing and releases

Official releases are signed on a GitHub-hosted Windows runner using Azure Artifact Signing and GitHub OIDC. No certificate private-key export or Azure client secret is needed.

## Azure federation

Use an Entra application/service principal dedicated to this repository. Its federated credential currently has:

```text
Issuer:   https://token.actions.githubusercontent.com
Subject:  repo:thegeekkid@8616594/church-device-control@1449827212:ref:refs/heads/main
Audience: api://AzureADTokenExchange
```

This repository uses GitHub's immutable subject format. The numeric owner and repository IDs above are identifiers, not secrets. A fork has a different repository identity and cannot satisfy this credential, even if it copies the workflow and client ID. A pull request's OIDC subject also differs from the trusted main-branch subject. Making this same repository public does not itself change the branch-based subject.

Keep the Azure identity's role limited to **Artifact Signing Certificate Profile Signer** (formerly **Trusted Signing Certificate Profile Signer**) on the intended certificate profile. Broad subscription or account Contributor access is not required. Identity validation and the signing account/profile must already exist. Use a public-trust profile for general Windows distribution; a private-trust profile requires client trust configuration.

Renaming or transferring the repository can change the subject even when numeric IDs remain. Adding a GitHub environment normally changes the subject from a branch to an environment context. Inspect the actual OIDC configuration and update Azure deliberately before either change; do not broaden the credential to arbitrary repositories or branches.

## Repository variables

Under **Settings > Secrets and variables > Actions > Variables**, configure:

| Name | Value |
| --- | --- |
| `AZURE_CLIENT_ID` | Entra application/client ID |
| `AZURE_TENANT_ID` | Directory/tenant ID |
| `AZURE_SUBSCRIPTION_ID` | Azure subscription ID |
| `AZURE_SIGNING_ENDPOINT` | Regional endpoint, such as `https://eus.codesigning.azure.net/` |
| `AZURE_SIGNING_ACCOUNT` | Artifact Signing account name |
| `AZURE_CERTIFICATE_PROFILE` | Certificate profile name |

These identify Azure resources. There is no client password or certificate private key in the repository. Do not add Kasa credentials or LAN sync keys to Actions variables or source files.

## Public-repository trust boundary

Federation prevents another repository from impersonating this one. It does not protect against someone allowed to run malicious code on trusted `main`. Code executing in the release job has access to that job's token permissions, including the ability to request an OIDC token.

- **CI** handles `pull_request` and `push` events with read-only contents access, no OIDC permission, and ephemeral GitHub-hosted runners.
- **Build and release** is manually dispatched and restricted to this repository's ID and `main`. It builds its own checkout instead of downloading pull-request artifacts. Checkout does not persist Git credentials.
- Only the trusted release job has `contents: write` and `id-token: write`. Action versions are pinned to commit SHAs.
- Review code, build scripts, workflow changes, and dependencies before merging. Use a main-branch ruleset and CODEOWNERS review for release-related files; keep write access limited to trusted maintainers.
- Do not change signing to run on `pull_request_target`, untrusted `workflow_run` artifacts, or contributor branches. Fork CI must stay separate from signing.

An optional protected release environment can add approval controls, but requires coordinated OIDC subject changes. It is not configured by the current workflow.

## Make a release

1. Update version metadata in `winres/winres.json` and `installer/setup.iss`, regenerate resources, and update the workflow's default release tag.
2. Write `docs/release-vX.Y.Z.txt` and add an entry to [CHANGELOG.md](../CHANGELOG.md). Keep version history out of the main README.
3. Update third-party notices when dependencies change. Review tests and CI on `main`.
4. Open **Actions > Build and release > Run workflow**, select `main`, enter the new version, and check **sign**. Check **publish_release** to publish after validation. Official publication requires signing.
5. Check the resulting signatures and publisher. The workflow tests the EXE, installs/uninstalls in both scopes, verifies shortcuts, signs the EXE and installer, and computes checksums after signing. A signing failure stops publication; existing releases are not overwritten.

Release assets:

```text
Church-TV-Control.exe
Church-Device-Control-Setup.exe
SHA256.txt
LICENSE.txt
THIRD-PARTY-NOTICES.txt
```

The installer includes the licenses. Keep the notices with a portable copy when redistributing it.

## References

- [Azure Artifact Signing action](https://github.com/Azure/artifact-signing-action)
- [Azure signing integrations](https://learn.microsoft.com/en-us/azure/artifact-signing/how-to-signing-integrations)
- [GitHub Actions security guidance](https://docs.github.com/en/actions/security-for-github-actions/security-guides/security-hardening-for-github-actions)
- [GitHub OIDC claims](https://docs.github.com/en/actions/reference/security/oidc)
