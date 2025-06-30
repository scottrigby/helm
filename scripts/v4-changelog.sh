#!/bin/bash

set -e
trap 'echo "Error on line $LINENO: Command failed with exit code $?"' ERR

## Keep for using JSON for output rather than gh command
# shas=$(git log dev-v3..main --format=format:%H)
# echo "${shas}"
# json='[]'
# echo "${shas}" | while IFS= read -r sha ; do
#     pr=$(gh pr list --search "${sha}" --state merged --json number,title,closedAt,url)
#     json=$(jq --argjson pr "${pr}" '. += $pr | unique' <<< "${json}")
#     echo "${json}"
#     echo
# done
# echo "FINAL:"
# echo "${json}"

prs=''
endCursor=''
pageCount=1
while [[ "${hasNextPage:-}" != "false" ]]; do
    # echo "pageCount=${pageCount}"
    # echo "hasNextPage=${hasNextPage:-starting}"
    # echo "endCursor=${endCursor:-starting}"

    # give some indication that things are happening and not just hanging
    echo "Processing page ${pageCount}..."

    out=$(gh api graphql \
    -F org=helm -F repo=helm \
    -F endCursor=${endCursor:-} \
    -f query='
    query ($org: String!, $repo: String!, $endCursor: String) {
      repository(owner: $org, name: $repo) {
        ref(qualifiedName: "dev-v3") {
          compare(headRef: "main") {
            commits(first: 100, after: $endCursor) {
              pageInfo {
                hasNextPage
                endCursor
              }
              totalCount
              nodes {
                abbreviatedOid
                ## UNCOMMMENT FOR CO-AUTHORS
                ## SEE commit authors doc
                ## REF https://docs.github.com/en/graphql/reference/objects#commit
                # oid
                # authors(first: 100) {
                #   nodes {
                #     user {
                #       login
                #     }
                #   }
                # }
                associatedPullRequests(first: 100) {
                  nodes {
                    number
                    title
                    mergedAt
                    author {
                      login
                      }
                    }
                  }
                }
              }
            }
          }
        }
      }
    }')
    ## DEBUG to find a commit with co-authors
    # echo ${out} | jq '.data.repository.ref.compare.commits.nodes[] | select(.oid=="8d964588cd3b54b470510ee9663eedba25c6186b")'
    # exit

    hasNextPage=$(jq '.data.repository.ref.compare.commits.pageInfo.hasNextPage' <<< "${out}")
    endCursor=$(jq -r '.data.repository.ref.compare.commits.pageInfo.endCursor' <<< "${out}")

    associatedPullRequests=$(jq '.data.repository.ref.compare.commits.nodes[].associatedPullRequests.nodes[].number' <<< "${out}" | uniq)

    if [[ -n "${prs}" ]]; then
        prs+=$'\n'
    fi
    prs+="${associatedPullRequests:-} "

    ((pageCount++))
done

prs=$(echo -n "${prs}" | uniq)

echo
echo "Final page count: ${pageCount}"
echo "Final PR count: $(echo "${prs}" | wc -l | bc)"
prs_string=$(sort -u <<< "${prs}")
echo "Final PR list: $(tr '\n' ' ' <<< ${prs_string})"
echo

# gh search prs --repo=helm/helm "${prs}"
# Error: The search is longer than 256 characters.
# TODO get json data and print our own table in the end instead
echo "Outputting PR info in max chunks of 30"
echo
echo -n "$prs" | xargs -n 30 gh search prs --repo=helm/helm
