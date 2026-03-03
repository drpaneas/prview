package model

import "time"

type Severity int

const (
	SeverityLow Severity = iota
	SeverityMedium
	SeverityHigh
)

func (s Severity) String() string {
	switch s {
	case SeverityLow:
		return "LOW"
	case SeverityMedium:
		return "MEDIUM"
	case SeverityHigh:
		return "HIGH"
	default:
		return "UNKNOWN"
	}
}

type PRInput struct {
	Owner  string
	Repo   string
	Number int
}

type PRMetadata struct {
	Title       string
	Author      string
	State       string
	CreatedAt   time.Time
	HeadBranch  string
	BaseBranch  string
	IsDraft     bool
	ReviewCount int
	CIStatus    string
	BaseSHA     string
	HeadSHA     string
}

type FileDiff struct {
	Path        string
	OldPath     string
	Status      string // added, modified, deleted, renamed
	Additions   int
	Deletions   int
	Patch       string
	IsBinary    bool
	IsGenerated bool
}

type FileContent struct {
	Path    string
	Content string
	SHA     string
}

type BlameLine struct {
	Author string
	Email  string
	Login  string // GitHub username (may be empty if git author has no linked GitHub account)
	Line   int
	Date   time.Time
}

type BlameResult struct {
	Path  string
	Lines []BlameLine
}

type AuthorProfile struct {
	Login            string
	MergedPRs        int
	FirstContribDate time.Time
	LastContribDate  time.Time
	TopAreas         []string
	IsFirstTime      bool
}

type PRData struct {
	Input     PRInput
	Meta      PRMetadata
	Diffs     []FileDiff
	BaseFiles []FileContent
	HeadFiles []FileContent
	Blames    []BlameResult
	Author    AuthorProfile
}

// Analysis results

type ScopeResult struct {
	FilesChanged  int
	TotalAdded    int
	TotalDeleted  int
	PackagesCount int
	DirBreakdown  []DirStat
	Complexity    int
}

type DirStat struct {
	Dir        string
	LinesAdded int
	Percentage float64
}

type ChangeType string

const (
	ChangeTypeFeature  ChangeType = "Feature"
	ChangeTypeBugfix   ChangeType = "Bugfix"
	ChangeTypeRefactor ChangeType = "Refactor"
	ChangeTypeDocs     ChangeType = "Docs"
	ChangeTypeDeps     ChangeType = "Deps"
	ChangeTypeTest     ChangeType = "Test"
	ChangeTypeConfig   ChangeType = "Config"
	ChangeTypeMixed    ChangeType = "Mixed"
)

type ClassifyResult struct {
	Type       ChangeType
	Confidence string
}

type FuncChange struct {
	Name       string
	File       string
	Exported   bool
	ChangeKind string // added, modified, deleted
	Signature  string
}

type ASTResult struct {
	Functions  []FuncChange
	NewExports []string
}

type RiskFlag struct {
	Severity    Severity
	File        string
	Line        int
	Description string
	Code        string
}

type Reviewer struct {
	Login      string
	Confidence string
	Ownership  float64
	Files      []string
	Reason     string
	LastActive string
}

type ArchChange struct {
	Description string
	Details     []string
}

type ArchResult struct {
	Before          []ArchChange
	After           []ArchChange
	DesignDecisions []string
	NewDeps         []string
	NewPackages     []string
}

type ReviewFocus struct {
	File     string
	Priority string
	Why      string
	LookFor  []string
}

type Verdict string

const (
	VerdictApprove        Verdict = "Approve"
	VerdictRequestChanges Verdict = "Request changes"
	VerdictDiscuss        Verdict = "Needs discussion"
)

type AIIssue struct {
	Severity    Severity
	File        string
	Line        int
	WhatChanged string
	WhyRisky    string
	TradeOff    string
	Suggestion  string
}

type ReviewQuestion struct {
	Question  string
	WhereLook string
	HowVerify string
}

type CriticalPath struct {
	Path           string
	WhyCritical    string
	RegressionRisk string
}

type MissingScenario struct {
	Scenario  string
	WhyNeeded string
}

type TestVerdict struct {
	Sufficient       bool
	Summary          string
	CriticalUntested []CriticalPath
	KeyTestFiles     []string
	MissingScenarios []MissingScenario
}

type RiskComment struct {
	File          string
	Line          int
	Pattern       string
	AIAssessment  string
	IsRealProblem bool
}

type AIAnalysis struct {
	Summary         string
	Before          string
	After           string
	Issues          []AIIssue
	ReviewQuestions []ReviewQuestion
	TestVerdict     TestVerdict
	Verdict         string
	VerdictReason   string
	RiskCommentary  []RiskComment
}

type PatternHit struct {
	Field       string
	Pattern     string
	MatchedText string
}

type AISlopResult struct {
	Verdict     string // "ai-assisted", "inconclusive", "human"
	Confidence  int
	Evidence    []string
	PatternHits []PatternHit
	LLMVerdict  string
}

type PRReport struct {
	Input        PRInput
	Meta         PRMetadata
	Scope        ScopeResult
	Classify     ClassifyResult
	AST          ASTResult
	Risks        []RiskFlag
	Reviewers    []Reviewer
	Architecture ArchResult
	ReviewFocus  []ReviewFocus
	Verdict      Verdict
	VerdictNote  string
	Author       AuthorProfile
	AI           *AIAnalysis
	AISlop       *AISlopResult
}
