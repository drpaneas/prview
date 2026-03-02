# prview

`prview` analyzes a GitHub pull request and prints a structured report
in your terminal. It combines heuristic analysis (scope, risks, blame-based
reviewer suggestions) with AI (Anthropic Claude) to produce a verdict.

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

Two environment variables are required:

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

## Options

```
--persona-dir DIR    directory with devlica personas (required when reviewers are found)
--model MODEL        Anthropic model to use (default: claude-sonnet-4-20250514)
```

When the PR touches files with git blame data, `prview` suggests reviewers and
generates a persona-based review using pre-crawled devlica profiles. If
`--persona-dir` is not set, the persona review is skipped with a warning.
Generate personas with `devlica <username>` before running `prview`.

## What the report contains

1. AI-slop detection (requires `aislop` on PATH)
2. PR header and CI status
3. Scope and complexity breakdown
4. What the PR does, before vs. after
5. Potential issues and review questions
6. Test coverage assessment
7. Risk flags (ignored errors, goroutines, hardcoded values, ...)
8. Suggested reviewers (from git blame)
9. Persona-based review (requires `--persona-dir`)
10. Verdict: approve, request changes, or discuss
