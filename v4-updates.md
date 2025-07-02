# Helm 4 updates

## Automate list for manual review

1. run `./scripts/v4-changelog.sh`
2. manually review each item in list

This document outlines the manual review

WIP

## NEXT TO CHECK

#30827  2025-05-06T17:59:16Z  findnature        refactor: use slices.Contains to simplify code

## Helm 4 Features

New features in Helm 4 that were not backported to v3

```text
#13604  2025-04-05T13:45:25Z  AustinAbro321     Introduce kstatus watcher
#10309  2025-02-21T23:12:16Z  Bez625            Add hook annotation to output hook logs to client on error
#13655  2025-02-20T22:37:52Z  LuBingtan         feat: support multi-document values files
#30294  2025-02-19T20:04:06Z  Zhanweelee        Supports json arguments
#30553  2025-05-07T03:56:41Z  Zhanweelee        feat: Add mustToYaml and mustToJson template functions
#13465  2024-11-20T10:18:22Z  banjoh            Add precommit config to .gitignore
#30751  2025-04-13T22:33:40Z  benoittgt         Add detailed debug logging for resource readiness states
#30904  2025-05-23T12:06:12Z  benoittgt         [Doc] Help users avoid specifying URL scheme and path with `helm registry`

```

## Helm 4 Fixes

Fixes in Helm 4 that were not backported to v3 (we may want to exclude these)

```text
#12581  2025-06-03T19:02:19Z  MichaelMorrisEst  Consider full GroupVersionKind when matching resources
#30862  2025-05-20T14:38:49Z  OmriSteiner       fix: correctly concat absolute URIs in repo cache
#30590  2025-03-01T16:25:05Z  SantalScript      fix:add proxy support when mTLS configured
#30939  2025-06-03T19:04:57Z  TerryHowe         fix: action hooks delete policy mutex
#30958  2025-06-06T17:42:20Z  TerryHowe         fix: repo update cmd mutex
#30972  2025-06-10T19:57:55Z  TerryHowe         fix: kube client create mutex
#30981  2025-06-15T18:11:31Z  TerryHowe         fix: lint test SetEnv errors
#30979  2025-06-17T19:06:31Z  TerryHowe         fix: OAuth username password login for v4
#31004  2025-06-26T13:01:27Z  andreped          fix(docs): Typofix in README
#30842  2025-05-15T18:38:00Z  ayushontop        Fix : No repository is not an error,use the helm repo list command ,if there is no repository,it should not be an error #30606
#30766  2025-04-17T12:36:50Z  benoittgt         Fix main branch by defining wait strategy parameter on hooks
#30930  2025-05-28T19:03:47Z  benoittgt         Fix flaky TestFindChartURL due to non-deterministic map iteration
#30955  2025-06-04T15:33:40Z  carloslima        Fix tests deleting XDG_DATA_HOME
#9175   2025-04-23T18:40:52Z  dastrobu          fix: copy dependencies on aliasing to avoid sharing chart references on multiply aliased dependencies
## NOTE this is in the 3.17.3 milestone, but it should not be. It is only in main after main was v4.
## TODO remove from milestone (first check to make sure this wasn't picked into v3.17.3 before removing)
#12382  2025-04-20T19:35:51Z  edbmiller         fix(pkg/lint): unmarshals Chart.yaml strictly
#30576  2025-02-23T19:51:34Z  felipecrs         Fix flaky TestDedupeRepos


```

## Helm 4 refactor/cleanup

```text
#13425  2024-11-15T00:20:25Z  MathieuCesbron    Fix typo "re-use" to "reuse"
#13516  2025-01-24T02:30:24Z  TerryHowe         chore: fix problems with latest lint
#30844  2025-05-08T13:28:40Z  TerryHowe         fix: rename slave replica
#30829  2025-05-09T14:54:22Z  TerryHowe         Increase pkg/time test coverage
#30957  2025-06-04T17:53:06Z  acceptacross      chore: fix some function names in comment
## NOTE I'm unsure how deleting these YAML comments was a bug fix. Can someone explain?
#30824  2025-05-05T17:29:59Z  adharsh277        Fix bug in .golangci.yml configuration
#30708  2025-04-11T19:50:34Z  benoittgt         Migrate pkg to slog
#30752  2025-04-16T19:50:56Z  benoittgt         Bump golangci lint to last major version and fix static-check errors
#30775  2025-04-19T15:50:48Z  benoittgt         Bump toml
#30872  2025-05-20T11:14:48Z  benoittgt         Bump golangci-lint version to match last golangci-lint-action
#30914  2025-05-27T14:59:02Z  benoittgt         Fix dependabot upgrade of jsonschema to v6.0.2
#13602  2025-01-17T21:06:01Z  crystalstall      refactor: using slices.Contains to simplify the code
#31002  2025-06-26T13:01:09Z  curlwget          chore: fix function in comment
#30508  2025-02-19T19:47:54Z  eimhin-rover      Update version option description with more accurate info
#11112  2025-02-22T20:50:31Z  felipecrs         perf(dep-up): do not update the same repo multiple times

```

## Helm 4 changes

User-facing changes aside from the above

```text

```

## Helm project (not helm CLI or SDK) changes

```text
#30912  2025-06-17T19:18:31Z  Bhargavkonidena   Fix #30893 - issue templates
#30882  2025-05-20T18:17:35Z  caniszczyk        Add new LFX Insights Health Score Badge
#30295  2025-02-07T14:43:12Z  edithturn         Add Percona to the list of organizations using Helm

```

## Backported to v3 (exclude from list)

```text
#10309  2025-02-21T23:12:16Z  Bez625            Add hook annotation to output hook logs to client on error
#12690  2025-01-01T16:49:14Z  TerryHowe         feat: OCI install by digest
#13382  2025-02-03T17:43:38Z  TerryHowe         chore(oci): migrate to ORAS Golang library v2
#30928  2025-05-28T14:27:57Z  TerryHowe         fix: plugin installer test with no Internet
#30937  2025-05-30T19:36:14Z  TerryHowe         fix: legacy docker support broken for login
#30917  2025-06-01T22:12:15Z  TerryHowe         fix: add debug logging to oci transport
#13533  2025-01-24T20:16:59Z  althmoha          fix: (toToml) renders int as float
#12769  2024-11-19T21:29:11Z  banjoh            fix(helm): pass down username/password CLI parameters to OCI registry clients
#13481  2025-02-18T19:48:30Z  banjoh            feat: Enable CPU and memory profiling
#30696  2025-03-24T19:19:10Z  benoittgt         Inform about time spent waiting resources to be ready in slog format
#30741  2025-04-11T19:53:39Z  benoittgt         Bumps github.com/distribution/distribution/v3 from 3.0.0-rc.3 to 3.0.0
#30894  2025-05-23T19:51:14Z  benoittgt         Prevent push cmd failure in 3.18 by handling version tag resolution in ORAS memory store
#13232  2024-12-20T00:39:33Z  dnskr             ref(create): don't render empty resource fields
#30677  2025-04-18T19:02:37Z  dongjiang1989     chore: Update Golang to v1.24

```
