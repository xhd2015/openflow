package sdk

import (
	"fmt"
	"os"
	"strings"
)

type Feedback interface {
	String() string
	ToAgent(agentName string) string
}

func Print(msg string) {
	fmt.Println(msg)
}

func S(v any) string {
	return fmt.Sprint(v)
}

func logMsg(label string, msg string) {
	fmt.Fprintf(os.Stderr, "[openflow] %s: %s\n", label, msg)
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func truncateStr(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func quoteArg(arg string) string {
	if strings.ContainsAny(arg, " '\"") {
		return "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
	}
	return arg
}

func truncateBytes(b []byte, max int) string {
	return truncateStr(string(b), max)
}
