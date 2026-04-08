package git

import (
	"fmt"
	"strconv"
	"strings"
)

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

func ParseAheadBehind(output string) (int, int, error) {
	parts := strings.Fields(strings.TrimSpace(output))
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected ahead/behind output %q", output)
	}

	ahead, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("parse ahead count %q: %w", parts[0], err)
	}
	behind, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parse behind count %q: %w", parts[1], err)
	}

	return ahead, behind, nil
}
