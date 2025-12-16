module github.com/grafana/auto-triage

go 1.23

toolchain go1.24.11

require (
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/magefile/mage v1.15.0
	github.com/mrz1836/go-sanitize v1.3.3
	github.com/sashabaranov/go-openai v1.32.0
	github.com/tiktoken-go/tokenizer v0.7.0
)

require github.com/dlclark/regexp2 v1.11.5 // indirect
