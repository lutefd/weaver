package git

import "strings"

func ParseBranches(output string) []string {
	if output == "" {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	branches := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimPrefix(line, "*"))
		if line == "" {
			continue
		}
		branches = append(branches, line)
	}

	return branches
}

func ParseCurrentBranch(output string) string {
	return strings.TrimSpace(output)
}
