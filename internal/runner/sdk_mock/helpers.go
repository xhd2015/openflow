package sdk

import "fmt"

type Feedback interface {
	String() string
	ToAgent(agentName string) string
}

func Print(msg string) {}

func S(v any) string {
	return fmt.Sprint(v)
}
