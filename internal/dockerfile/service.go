package dockerfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

var ErrValidation = errors.New("dockerfile validation failed")

type Service struct{}

func NewService() Service {
	return Service{}
}

func (s Service) Generate(input GenerateRequest) (string, error) {
	baseImage := strings.TrimSpace(input.BaseImage)
	if baseImage == "" {
		return "", fmt.Errorf("%w: baseImage is required", ErrValidation)
	}

	lines := []string{"FROM " + baseImage}

	for _, key := range sortedKeys(input.Args) {
		if key = strings.TrimSpace(key); key != "" {
			lines = append(lines, "ARG "+key+"="+shellQuote(input.Args[key]))
		}
	}
	for _, key := range sortedKeys(input.Labels) {
		if key = strings.TrimSpace(key); key != "" {
			lines = append(lines, "LABEL "+key+"="+shellQuote(input.Labels[key]))
		}
	}
	for _, key := range sortedKeys(input.Environment) {
		if key = strings.TrimSpace(key); key != "" {
			lines = append(lines, "ENV "+key+"="+shellQuote(input.Environment[key]))
		}
	}

	if packages := cleanList(input.Packages); len(packages) > 0 {
		lines = append(lines, "RUN apt-get update && apt-get install -y --no-install-recommends "+strings.Join(packages, " ")+" && rm -rf /var/lib/apt/lists/*")
	}
	if workdir := strings.TrimSpace(input.Workdir); workdir != "" {
		lines = append(lines, "WORKDIR "+workdir)
	}
	for _, rule := range input.Copy {
		source := strings.TrimSpace(rule.Source)
		target := strings.TrimSpace(rule.Target)
		if source != "" && target != "" {
			lines = append(lines, "COPY "+source+" "+target)
		}
	}
	for _, port := range input.Expose {
		if port > 0 {
			lines = append(lines, "EXPOSE "+strconv.Itoa(port))
		}
	}
	if len(input.Entrypoint) > 0 {
		line, err := jsonInstruction("ENTRYPOINT", input.Entrypoint)
		if err != nil {
			return "", err
		}
		lines = append(lines, line)
	}
	if len(input.CMD) > 0 {
		line, err := jsonInstruction("CMD", input.CMD)
		if err != nil {
			return "", err
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n") + "\n", nil
}

func (s Service) Validate(value string) ValidationResult {
	result := ValidationResult{Valid: true, Warnings: []string{}, Errors: []string{}}
	value = strings.TrimSpace(value)
	if value == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "Dockerfile is required.")
		return result
	}

	hasFrom := false
	for index, line := range strings.Split(value, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		upper := strings.ToUpper(trimmed)
		if strings.HasPrefix(upper, "FROM ") {
			hasFrom = true
		}
		if strings.HasPrefix(upper, "ADD ") {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Line %d uses ADD; COPY is preferred unless archive extraction or remote URL handling is required.", index+1))
		}
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "--password") || strings.Contains(lower, "token=") {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Line %d may contain a secret value.", index+1))
		}
	}
	if !hasFrom {
		result.Valid = false
		result.Errors = append(result.Errors, "Dockerfile must contain a FROM instruction.")
	}
	return result
}

func jsonInstruction(name string, values []string) (string, error) {
	cleaned := cleanList(values)
	if len(cleaned) == 0 {
		return "", nil
	}
	data, err := json.Marshal(cleaned)
	if err != nil {
		return "", fmt.Errorf("encode %s instruction: %w", name, err)
	}
	return name + " " + string(data), nil
}

func cleanList(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	return cleaned
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func shellQuote(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return `""`
	}
	return strconv.Quote(value)
}
