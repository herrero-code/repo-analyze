package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	RepoPath      string
	StaleDays     int
	TodoDays      int
	ShowHelp      bool
	CheckBranches bool
	CheckPRs      bool
	CheckTodos    bool
}

type BranchInfo struct {
	Name       string
	LastCommit time.Time
	Author     string
	DaysStale  int
	IsRemote   bool
}

type TodoItem struct {
	File    string
	Line    int
	Content string
	Type    string // TODO, FIXME, etc.
	Age     time.Time
	DaysOld int
}

func main() {
	config := parseFlags()

	if config.ShowHelp {
		printUsage()
		return
	}

	// Validate repository path
	if err := validateGitRepo(config.RepoPath); err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Printf("🔍 Analyzing repository: %s\n\n", config.RepoPath)

	// Change to repository directory
	originalDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	if err := os.Chdir(config.RepoPath); err != nil {
		log.Fatalf("Failed to change to repo directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Run analyses based on configuration
	if config.CheckBranches {
		analyzeStaleBranches(config.StaleDays)
	}

	if config.CheckPRs {
		analyzeUnmergedPRs()
	}

	if config.CheckTodos {
		analyzeTodoComments(config.TodoDays)
	}
}

func parseFlags() Config {
	config := Config{}

	flag.StringVar(&config.RepoPath, "path", ".", "Path to git repository")
	flag.IntVar(&config.StaleDays, "stale-days", 30, "Days to consider a branch stale")
	flag.IntVar(&config.TodoDays, "todo-days", 90, "Days to consider TODO/FIXME comments old")
	flag.BoolVar(&config.ShowHelp, "help", false, "Show help message")
	flag.BoolVar(&config.CheckBranches, "branches", true, "Check for stale branches")
	flag.BoolVar(&config.CheckPRs, "prs", true, "Check for unmerged PRs")
	flag.BoolVar(&config.CheckTodos, "todos", true, "Check for old TODO/FIXME comments")

	flag.Parse()

	// If no specific checks are enabled, enable all
	if !config.CheckBranches && !config.CheckPRs && !config.CheckTodos {
		config.CheckBranches = true
		config.CheckPRs = true
		config.CheckTodos = true
	}

	return config
}

func printUsage() {
	fmt.Println("Git Repository Analyzer")
	fmt.Println("=======================")
	fmt.Println()
	fmt.Println("Usage: git-analyzer [options]")
	fmt.Println()
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  git-analyzer -path=/path/to/repo -stale-days=14")
	fmt.Println("  git-analyzer -branches=false -todos=true -todo-days=60")
}

func validateGitRepo(path string) error {
	gitDir := filepath.Join(path, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository: %s", path)
	}
	return nil
}

func analyzeStaleBranches(staleDays int) {
	fmt.Println("📊 Analyzing Stale Branches")
	fmt.Println("===========================")

	branches, err := getStaleBranches(staleDays)
	if err != nil {
		log.Printf("Error analyzing branches: %v", err)
		return
	}

	if len(branches) == 0 {
		fmt.Printf("✅ No stale branches found (older than %d days)\n\n", staleDays)
		return
	}

	// Sort by days stale (most stale first)
	sort.Slice(branches, func(i, j int) bool {
		return branches[i].DaysStale > branches[j].DaysStale
	})

	fmt.Printf("Found %d stale branches:\n\n", len(branches))

	for _, branch := range branches {
		branchType := "local"
		if branch.IsRemote {
			branchType = "remote"
		}

		fmt.Printf("🔸 %s (%s)\n", branch.Name, branchType)
		fmt.Printf("   Last commit: %s (%d days ago)\n",
			branch.LastCommit.Format("2006-01-02"), branch.DaysStale)
		fmt.Printf("   Author: %s\n\n", branch.Author)
	}
}

func getStaleBranches(staleDays int) ([]BranchInfo, error) {
	var branches []BranchInfo
	cutoffDate := time.Now().AddDate(0, 0, -staleDays)

	// Get all branches (local and remote)
	cmd := exec.Command("git", "branch", "-a", "--format=%(refname:short)%09%(committerdate:iso8601)%09%(authorname)")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}

		branchName := strings.TrimSpace(parts[0])
		commitDateStr := strings.TrimSpace(parts[1])
		authorName := strings.TrimSpace(parts[2])

		// Skip HEAD references
		if strings.Contains(branchName, "HEAD") {
			continue
		}

		commitDate, err := time.Parse("2006-01-02 15:04:05 -0700", commitDateStr)
		if err != nil {
			continue
		}

		if commitDate.Before(cutoffDate) {
			daysStale := int(time.Since(commitDate).Hours() / 24)
			isRemote := strings.HasPrefix(branchName, "origin/")

			branches = append(branches, BranchInfo{
				Name:       branchName,
				LastCommit: commitDate,
				Author:     authorName,
				DaysStale:  daysStale,
				IsRemote:   isRemote,
			})
		}
	}

	return branches, nil
}

func analyzeUnmergedPRs() {
	fmt.Println("🔀 Analyzing Unmerged Pull Requests")
	fmt.Println("===================================")

	// This is a simplified implementation that checks for branches that might be PRs
	// In a real implementation, you'd integrate with GitHub/GitLab APIs

	unmergedBranches, err := getUnmergedBranches()
	if err != nil {
		log.Printf("Error analyzing unmerged branches: %v", err)
		return
	}

	if len(unmergedBranches) == 0 {
		fmt.Println("✅ No potential unmerged PR branches found\n")
		return
	}

	fmt.Printf("Found %d potential unmerged PR branches:\n\n", len(unmergedBranches))

	for _, branch := range unmergedBranches {
		fmt.Printf("🔸 %s\n", branch.Name)
		fmt.Printf("   Last commit: %s (%d days ago)\n",
			branch.LastCommit.Format("2006-01-02"), branch.DaysStale)
		fmt.Printf("   Author: %s\n\n", branch.Author)
	}

	fmt.Println("💡 Note: For complete PR analysis, integrate with your Git hosting platform's API")
	fmt.Println()
}

func getUnmergedBranches() ([]BranchInfo, error) {
	var branches []BranchInfo

	// Get remote branches that haven't been merged to main/master
	mainBranches := []string{"main", "master", "develop"}
	var mainBranch string

	for _, branch := range mainBranches {
		cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
		if cmd.Run() == nil {
			mainBranch = branch
			break
		}
	}

	if mainBranch == "" {
		return branches, fmt.Errorf("no main branch found (main, master, or develop)")
	}

	// Get branches not merged into main
	cmd := exec.Command("git", "branch", "-r", "--no-merged", mainBranch,
		"--format=%(refname:short)%09%(committerdate:iso8601)%09%(authorname)")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}

		branchName := strings.TrimSpace(parts[0])
		commitDateStr := strings.TrimSpace(parts[1])
		authorName := strings.TrimSpace(parts[2])

		// Skip HEAD references
		if strings.Contains(branchName, "HEAD") {
			continue
		}

		commitDate, err := time.Parse("2006-01-02 15:04:05 -0700", commitDateStr)
		if err != nil {
			continue
		}

		daysOld := int(time.Since(commitDate).Hours() / 24)

		branches = append(branches, BranchInfo{
			Name:       branchName,
			LastCommit: commitDate,
			Author:     authorName,
			DaysStale:  daysOld,
			IsRemote:   true,
		})
	}

	return branches, nil
}

func analyzeTodoComments(todoDays int) {
	fmt.Println("📝 Analyzing TODO/FIXME Comments")
	fmt.Println("================================")

	todos, err := findTodoComments(todoDays)
	if err != nil {
		log.Printf("Error analyzing TODO comments: %v", err)
		return
	}

	if len(todos) == 0 {
		fmt.Printf("✅ No old TODO/FIXME comments found (older than %d days)\n\n", todoDays)
		return
	}

	// Group by type and sort by age
	sort.Slice(todos, func(i, j int) bool {
		return todos[i].DaysOld > todos[j].DaysOld
	})

	todoCount := 0
	fixmeCount := 0
	for _, todo := range todos {
		if strings.ToUpper(todo.Type) == "TODO" {
			todoCount++
		} else if strings.ToUpper(todo.Type) == "FIXME" {
			fixmeCount++
		}
	}

	fmt.Printf("Found %d old comments (%d TODOs, %d FIXMEs):\n\n",
		len(todos), todoCount, fixmeCount)

	for _, todo := range todos {
		fmt.Printf("🔸 %s (%d days old)\n", todo.Type, todo.DaysOld)
		fmt.Printf("   File: %s:%d\n", todo.File, todo.Line)
		fmt.Printf("   Content: %s\n\n", strings.TrimSpace(todo.Content))
	}
}

func findTodoComments(todoDays int) ([]TodoItem, error) {
	var todos []TodoItem
	cutoffDate := time.Now().AddDate(0, 0, -todoDays)

	// Regular expression to match TODO/FIXME comments
	todoRegex := regexp.MustCompile(`(?i)(TODO|FIXME|XXX|HACK)\s*[:\-]?\s*(.*)`)

	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip non-source files and git directories
		if info.IsDir() || shouldSkipFile(path) {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			if matches := todoRegex.FindStringSubmatch(line); matches != nil {
				todoType := strings.ToUpper(matches[1])
				content := matches[2]
				if content == "" {
					content = line
				}

				// Get the age of this file/line using git blame
				age, err := getLineAge(path, lineNum)
				if err != nil {
					// If we can't get the age, assume it's old
					age = cutoffDate.AddDate(0, 0, -1)
				}

				if age.Before(cutoffDate) {
					daysOld := int(time.Since(age).Hours() / 24)

					todos = append(todos, TodoItem{
						File:    path,
						Line:    lineNum,
						Content: content,
						Type:    todoType,
						Age:     age,
						DaysOld: daysOld,
					})
				}
			}
		}

		return scanner.Err()
	})

	return todos, err
}

func shouldSkipFile(path string) bool {
	// Skip common non-source directories and files
	skipPatterns := []string{
		".git/", "node_modules/", "vendor/", ".vscode/", ".idea/",
		"build/", "dist/", "target/", "bin/", "obj/",
	}

	for _, pattern := range skipPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	// Skip binary files and common non-source extensions
	ext := strings.ToLower(filepath.Ext(path))
	skipExtensions := []string{
		".exe", ".dll", ".so", ".dylib", ".a", ".lib",
		".jpg", ".jpeg", ".png", ".gif", ".ico", ".svg",
		".pdf", ".doc", ".docx", ".xls", ".xlsx",
		".zip", ".tar", ".gz", ".7z", ".rar",
		".mp3", ".mp4", ".avi", ".mov",
	}

	for _, skipExt := range skipExtensions {
		if ext == skipExt {
			return true
		}
	}

	return false
}

func getLineAge(file string, lineNum int) (time.Time, error) {
	cmd := exec.Command("git", "blame", "-L", fmt.Sprintf("%d,%d", lineNum, lineNum), "--porcelain", file)
	output, err := cmd.Output()
	if err != nil {
		return time.Time{}, err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "committer-time ") {
			timestampStr := strings.TrimPrefix(line, "committer-time ")
			timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
			if err != nil {
				return time.Time{}, err
			}
			return time.Unix(timestamp, 0), nil
		}
	}

	return time.Time{}, fmt.Errorf("could not parse git blame output")
}
