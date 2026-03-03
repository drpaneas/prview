# prview

`prview` analyzes a GitHub pull request and prints a structured report
in your terminal. It combines heuristic analysis (scope, risks, blame-based
reviewer suggestions) with AI to produce a verdict.

## Install

```
go install github.com/drpaneas/prview@latest
```

Or build from source:

```
git clone https://github.com/drpaneas/prview.git
cd prview
go build -o prview .
```

## Setup

`prview` needs a GitHub token and an AI provider key.
Set one of the two AI providers - if both are present, Gemini is used.

```
export GITHUB_TOKEN=ghp_...
export GEMINI_API_KEY=...
```

or

```
export GITHUB_TOKEN=ghp_...
export ANTHROPIC_API_KEY=sk-ant-...
```

## Usage

```
prview owner/repo#number
prview https://github.com/owner/repo/pull/123
```

The report opens in a scrollable TUI. Navigate with `j`/`k`, page with
`f`/`b`, jump to top/bottom with `g`/`G`, quit with `q`.

### Dry run (markdown output)

Use `--dry-run` to print the report as GitHub-flavored markdown to stdout.
This is useful for previewing the output or piping it into a GitHub comment:

```
prview --dry-run owner/repo#123
prview --dry-run owner/repo#123 | gh pr comment 123 -R owner/repo --body-file -
```

## Options

```
--dry-run            print markdown to stdout instead of launching TUI
--model MODEL        AI model to use (default depends on provider)
```

## What the report contains

1. AI-slop detection (requires `aislop` on PATH)
2. PR header, author profile, and CI status
3. Scope and complexity breakdown
4. What the PR does, before vs. after
5. Potential issues and review questions
6. Risk flags (ignored errors, goroutines, hardcoded values, ...)
7. Suggested reviewers (from git blame)
8. Verdict: approve, request changes, or discuss

## GitHub Action

You can run `prview` automatically on every pull request.
When someone opens or updates a PR - including PRs from forks -
the action posts a review comment with the full analysis.

There are two pieces: a workflow file and one repository secret.

### 1. Add the secret

Go to your repository settings, then Secrets and variables, then Actions.
Create a secret called `GEMINI_API_KEY` containing your Google AI API key.
(You can get one from https://aistudio.google.com/apikey.)

The `GITHUB_TOKEN` is provided by GitHub Actions automatically.
You do not need to create it.

### 2. Add the workflow

Create `.github/workflows/pr-review.yml` in your repository:

```yaml
name: PR Review

on:
  pull_request_target:
    types: [opened, synchronize, reopened]

permissions:
  contents: read
  pull-requests: write

jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-go@v5
        with:
          go-version: stable

      - name: Install prview
        run: go install github.com/drpaneas/prview@main

      - name: Run prview
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          GEMINI_API_KEY: ${{ secrets.GEMINI_API_KEY }}
        run: |
          prview --dry-run \
            "https://github.com/${{ github.repository }}/pull/${{ github.event.pull_request.number }}" \
            > review.md 2>prview-err.log
          if [ $? -ne 0 ]; then
            echo "::error::prview failed:"
            cat prview-err.log
            exit 1
          fi

      - name: Comment on PR
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          if [ ! -s review.md ]; then
            echo "No review output, skipping."
            exit 0
          fi

          PR_NUMBER=${{ github.event.pull_request.number }}
          MARKER="<!-- prview-bot -->"
          { echo "$MARKER"; cat review.md; } > comment.md

          COMMENT_ID=$(gh api "repos/${{ github.repository }}/issues/${PR_NUMBER}/comments" \
            --paginate --jq ".[] | select(.body | startswith(\"$MARKER\")) | .id" \
            | head -1)

          if [ -n "$COMMENT_ID" ]; then
            gh api "repos/${{ github.repository }}/issues/comments/${COMMENT_ID}" \
              -X PATCH -F "body=@comment.md"
          else
            gh pr comment "$PR_NUMBER" -R "${{ github.repository }}" --body-file comment.md
          fi
```

That is the entire setup. Commit the file, push it, and the next pull
request will get a review comment.

### How it works

The workflow triggers on `pull_request_target`, which means it runs
in the context of your repository, not the fork. This has two
consequences that matter:

First, your secrets are available. A regular `pull_request` event does
not expose secrets to workflows triggered by fork PRs - a reasonable
security measure, since the fork controls the workflow code. With
`pull_request_target`, the workflow comes from your default branch, so
GitHub trusts it with your secrets.

Second, this is safe. `prview` does not check out or execute code from
the pull request. It reads the diff through the GitHub API and sends it
to the AI provider. There is no path from the fork's code to your
runner's shell.

When the contributor pushes new commits, the workflow runs again and
updates the existing comment rather than posting a new one. The comment
is identified by a hidden HTML marker, so there is never more than one
`prview` comment per pull request.

### Using Anthropic instead

If you prefer Claude over Gemini, replace `GEMINI_API_KEY` with
`ANTHROPIC_API_KEY` in both the secret and the workflow file. The tool
picks the provider based on which environment variable is set.
