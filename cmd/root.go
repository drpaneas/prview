package cmd

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/drpaneas/prview/internal/ai"
	"github.com/drpaneas/prview/internal/aislop"
	"github.com/drpaneas/prview/internal/analyzer"
	ghclient "github.com/drpaneas/prview/internal/github"
	"github.com/drpaneas/prview/internal/markdown"
	"github.com/drpaneas/prview/internal/model"
	"github.com/drpaneas/prview/internal/ui"
	"github.com/spf13/cobra"
)

var (
	modelName string
	dryRun    bool
)

var rootCmd = &cobra.Command{
	Use:     "prview <owner/repo#number | PR-URL>",
	Short:   "Analyze GitHub PRs at a glance",
	Long:    "prview fetches a GitHub pull request and uses AI to provide a structured analysis to help maintainers review PRs quickly and thoroughly.",
	Version: getVersion(),
	Args:    cobra.ExactArgs(1),
	RunE:    runRoot,
}

func init() {
	rootCmd.Flags().StringVar(&modelName, "model", "", "AI model to use (default depends on provider)")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "print markdown to stdout instead of launching TUI")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runRoot(cmd *cobra.Command, args []string) error {
	ghToken := os.Getenv("GITHUB_TOKEN")
	if ghToken == "" {
		return fmt.Errorf("GITHUB_TOKEN environment variable is required.\nSet it with: export GITHUB_TOKEN=your_token")
	}

	geminiKey := os.Getenv("GEMINI_API_KEY")
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	if geminiKey == "" && anthropicKey == "" {
		return fmt.Errorf("set GEMINI_API_KEY or ANTHROPIC_API_KEY environment variable")
	}

	input, err := ParsePRInput(args[0])
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	// Fetch PR data
	stop := startSpinner(fmt.Sprintf("Fetching %s/%s#%d", input.Owner, input.Repo, input.Number))
	client := ghclient.NewClient(ghToken)
	data, err := client.FetchPR(ctx, input)
	stop()
	if err != nil {
		return fmt.Errorf("failed to fetch PR data: %w", err)
	}
	data.Input = input

	// Pre-compute heuristic risks so we can pass them to the AI
	risks := analyzer.PreComputeRisks(data)

	// Run AI analysis and aislop detection in parallel
	var aiClient ai.Caller
	if geminiKey != "" {
		aiClient = ai.NewGeminiClient(geminiKey, modelName)
	} else {
		aiClient = ai.NewAnthropicClient(anthropicKey, modelName)
	}
	var aiAnalysis *model.AIAnalysis
	var aiErr error
	var slopResult *model.AISlopResult
	var slopErr error

	prURL := fmt.Sprintf("https://github.com/%s/%s/pull/%d", input.Owner, input.Repo, input.Number)

	var wg sync.WaitGroup
	wg.Add(2)

	stop = startSpinner("Analyzing PR (AI + slop detection)")

	go func() {
		defer wg.Done()
		aiAnalysis, aiErr = ai.AnalyzePR(ctx, aiClient, data, risks)
	}()

	go func() {
		defer wg.Done()
		provider := "claude"
		if geminiKey != "" {
			provider = "gemini"
		}
		slopResult, slopErr = aislop.Detect(ctx, prURL, provider)
	}()

	wg.Wait()
	stop()

	if aiErr != nil {
		return fmt.Errorf("AI analysis failed: %w", aiErr)
	}
	if slopErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: AI slop detection failed: %v\n", slopErr)
		slopResult = nil
	}

	report := analyzer.Analyze(data, aiAnalysis)
	report.Input = input
	report.AISlop = slopResult

	if dryRun {
		fmt.Print(markdown.Render(report))
		return nil
	}

	return ui.Run(report)
}

var (
	shortPattern = regexp.MustCompile(`^([^/]+)/([^#]+)#(\d+)$`)
	urlPattern   = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)/pull/(\d+)`)
)

func ParsePRInput(raw string) (model.PRInput, error) {
	if m := shortPattern.FindStringSubmatch(raw); m != nil {
		num, _ := strconv.Atoi(m[3])
		return model.PRInput{Owner: m[1], Repo: m[2], Number: num}, nil
	}
	if m := urlPattern.FindStringSubmatch(raw); m != nil {
		num, _ := strconv.Atoi(m[3])
		return model.PRInput{Owner: m[1], Repo: m[2], Number: num}, nil
	}
	return model.PRInput{}, fmt.Errorf("invalid PR reference %q\nUsage: prview owner/repo#123 or prview https://github.com/owner/repo/pull/123", raw)
}

func startSpinner(msg string) func() {
	if dryRun {
		fmt.Fprintf(os.Stderr, "%s...\n", msg)
		return func() {}
	}
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	var mu sync.Mutex
	done := false
	go func() {
		i := 0
		for {
			mu.Lock()
			if done {
				mu.Unlock()
				return
			}
			mu.Unlock()
			fmt.Fprintf(os.Stderr, "\r%s %s...", frames[i%len(frames)], msg)
			i++
			time.Sleep(80 * time.Millisecond)
		}
	}()
	return func() {
		mu.Lock()
		done = true
		mu.Unlock()
		fmt.Fprint(os.Stderr, "\r\033[K")
	}
}
