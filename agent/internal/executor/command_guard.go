package executor

import (
	"fmt"
	"regexp"
	"strings"
)

type GuardViolation struct {
	Rule    string
	Details string
}

func (v *GuardViolation) Error() string {
	return fmt.Sprintf("security violation [%s]: %s", v.Rule, v.Details)
}

type CommandGuard struct {
	blockedPatterns []*regexp.Regexp
	blockedBinaries []string
}

func NewCommandGuard() *CommandGuard {
	patterns := []string{
		`\brm\s+-[a-zA-Z]*(?:r|R)[a-zA-Z]*\s+(?:/|\./|\.\./|\*|/\*|--no-preserve-root)(?:\s|$|;)`,
		`\brm\s+--recursive\s+(?:/|\./|\.\./|\*|/\*)(?:\s|$|;)`,
		`\bmkfs(?:\.[a-z0-9]+)?\b`,
		`\bfdisk\b`,
		`\bdd\s+.*of=/dev/(?:sd[a-z]|vd[a-z]|nvme[0-9]n[0-9]|null|zero)`,
		`\b(?:shutdown|reboot|poweroff|halt)\b`,
		`\binit\s+[06]\b`,
		`\buserdel\b`,
		`\bgroupdel\b`,
		`\biptables\s+-F\b`,
		`\bnft\s+flush\s+ruleset\b`,
		`curl\s+.*\|\s*(?:ba)?sh`,
		`wget\s+.*\|\s*(?:ba)?sh`,
		`:\(\)\s*\{\s*:\|:&\s*\}\s*;\s*:`, // Fork bomb
	}

	compiled := make([]*regexp.Regexp, len(patterns))
	for i, p := range patterns {
		compiled[i] = regexp.MustCompile("(?i)" + p)
	}

	return &CommandGuard{
		blockedPatterns: compiled,
		blockedBinaries: []string{
			"mkfs", "fdisk", "parted", "userdel", "groupdel", "shutdown", "reboot", "poweroff",
		},
	}
}

func (g *CommandGuard) Validate(command string) error {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return &GuardViolation{Rule: "EMPTY_COMMAND", Details: "command string is empty"}
	}

	for _, pattern := range g.blockedPatterns {
		if pattern.MatchString(trimmed) {
			return &GuardViolation{
				Rule:    "BLOCKED_PATTERN",
				Details: fmt.Sprintf("command matches blocked pattern: %s", pattern.String()),
			}
		}
	}

	tokens := strings.Fields(trimmed)
	if len(tokens) > 0 {
		bin := strings.ToLower(tokens[0])
		// Strip sudo prefix if present
		if bin == "sudo" && len(tokens) > 1 {
			bin = strings.ToLower(tokens[1])
		}
		for _, blocked := range g.blockedBinaries {
			if bin == blocked || strings.HasSuffix(bin, "/"+blocked) {
				return &GuardViolation{
					Rule:    "FORBIDDEN_BINARY",
					Details: fmt.Sprintf("execution of binary '%s' is strictly forbidden", blocked),
				}
			}
		}
	}

	return nil
}
