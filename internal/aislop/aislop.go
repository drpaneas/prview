package aislop

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/drpaneas/prview/internal/model"
)

const (
	repoAPI    = "https://api.github.com/repos/drpaneas/aislop/releases/latest"
	releaseURL = "https://github.com/drpaneas/aislop/releases"
)

type rawResult struct {
	Verdict     string `json:"verdict"`
	PatternHits []struct {
		Field       string `json:"field"`
		Pattern     string `json:"pattern"`
		MatchedText string `json:"matched_text"`
	} `json:"pattern-hits"`
	LLMVerdict *struct {
		Verdict    string   `json:"verdict"`
		Confidence int      `json:"confidence"`
		Evidence   []string `json:"evidence"`
	} `json:"llm_verdict"`
}

func findBinary() string {
	if p, err := exec.LookPath("aislop"); err == nil {
		return p
	}
	installed := installedPath()
	if _, err := os.Stat(installed); err == nil {
		return installed
	}
	return ""
}

func installedPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "bin", "aislop")
}

func Detect(ctx context.Context, prURL, llmProvider string) (*model.AISlopResult, error) {
	bin := findBinary()
	if bin == "" {
		var err error
		bin, err = install(ctx)
		if err != nil {
			return nil, fmt.Errorf("aislop not found and auto-install failed: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Installed aislop to %s\n", bin)
	}

	if llmProvider == "" {
		llmProvider = "claude"
	}
	args := []string{prURL, "-f", "json", "--llm", "--llm-provider", llmProvider}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = os.Environ()

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			output = exitErr.Stderr
		}
		return nil, fmt.Errorf("aislop failed: %w (output: %s)", err, string(output))
	}

	var raw rawResult
	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("parsing aislop output: %w", err)
	}

	result := &model.AISlopResult{
		Verdict: raw.Verdict,
	}

	for _, hit := range raw.PatternHits {
		result.PatternHits = append(result.PatternHits, model.PatternHit{
			Field:       hit.Field,
			Pattern:     hit.Pattern,
			MatchedText: hit.MatchedText,
		})
	}

	if raw.LLMVerdict != nil {
		result.LLMVerdict = raw.LLMVerdict.Verdict
		result.Confidence = raw.LLMVerdict.Confidence
		result.Evidence = raw.LLMVerdict.Evidence
	}

	return result, nil
}

func assetSuffix() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return "aarch64-apple-darwin.tar.gz", nil
	case "linux":
		return "x86_64-unknown-linux-gnu.tar.gz", nil
	default:
		return "", fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

type releaseResponse struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func install(ctx context.Context) (string, error) {
	suffix, err := assetSuffix()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", repoAPI, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API returned status %d (try setting GITHUB_TOKEN)", resp.StatusCode)
	}

	var release releaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("parsing release info: %w", err)
	}

	var downloadURL string
	for _, a := range release.Assets {
		if len(a.Name) >= len(suffix) && a.Name[len(a.Name)-len(suffix):] == suffix {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return "", fmt.Errorf("no asset found for %s at %s", suffix, releaseURL)
	}

	dest := installedPath()
	if dest == "" {
		return "", fmt.Errorf("cannot determine install path")
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("creating directory: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Downloading aislop %s...\n", release.TagName)

	dlReq, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating download request: %w", err)
	}

	dlResp, err := http.DefaultClient.Do(dlReq)
	if err != nil {
		return "", fmt.Errorf("downloading: %w", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != 200 {
		return "", fmt.Errorf("download returned status %d", dlResp.StatusCode)
	}

	if err := extractBinary(dlResp.Body, dest); err != nil {
		return "", fmt.Errorf("extracting binary: %w", err)
	}

	return dest, nil
}

func extractBinary(r io.Reader, dest string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("aislop binary not found in archive")
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		if filepath.Base(hdr.Name) == "aislop" && hdr.Typeflag == tar.TypeReg {
			f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err != nil {
				return fmt.Errorf("creating file: %w", err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("writing binary: %w", err)
			}
			return f.Close()
		}
	}
}
