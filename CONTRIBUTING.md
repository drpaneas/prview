Contributing to prview

Prerequisites
- Go 1.25 or newer
- GITHUB_TOKEN with access to read pull requests
- ANTHROPIC_API_KEY for AI analysis
- Optional: aislop installed and available on PATH

Development flow
1. Create a branch for your change.
2. Run formatting and checks before every commit:
   - gofmt -w .
   - go test ./...
   - go vet ./...
3. Keep changes scoped and include tests for behavior changes.
4. Open a pull request with:
   - Clear problem statement
   - Why the solution is correct
   - Verification steps and results

Code quality expectations
- Avoid unrelated refactors in the same pull request.
- Preserve existing behavior unless explicitly changing it.
- Prefer explicit errors over silent fallbacks.
- Add tests when touching parsing, API mapping, or output rendering logic.
