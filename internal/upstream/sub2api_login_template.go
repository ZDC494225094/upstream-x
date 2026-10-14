package upstream

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const defaultSub2APILoginBodyTemplate = `{"email":"{{email}}","password":"{{password}}"}`

var sub2APILoginTemplatePattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_]+)\s*\}\}`)

func ValidateSub2APILoginBodyTemplate(template string) error {
	trimmed := strings.TrimSpace(template)
	if trimmed == "" {
		return nil
	}

	payload, err := decodeSub2APILoginBodyTemplate(trimmed)
	if err != nil {
		return err
	}
	if err := validateSub2APILoginBodyTemplateValue(payload); err != nil {
		return err
	}
	return nil
}

func RenderSub2APILoginBody(template string, email string, password string) ([]byte, error) {
	trimmed := strings.TrimSpace(template)
	if trimmed == "" {
		trimmed = defaultSub2APILoginBodyTemplate
	}

	payload, err := decodeSub2APILoginBodyTemplate(trimmed)
	if err != nil {
		return nil, err
	}

	vars := map[string]string{
		"email":    email,
		"password": password,
	}

	rendered, err := renderSub2APILoginBodyTemplateValue(payload, vars)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(rendered)
	if err != nil {
		return nil, fmt.Errorf("marshal sub2api login payload: %w", err)
	}
	return body, nil
}

func decodeSub2APILoginBodyTemplate(template string) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(template), &payload); err != nil {
		return nil, fmt.Errorf("invalid sub2api login body template: %w", err)
	}
	if payload == nil {
		return nil, errors.New("sub2api login body template must be a JSON object")
	}
	return payload, nil
}

func validateSub2APILoginBodyTemplateValue(value any) error {
	switch current := value.(type) {
	case map[string]any:
		for _, nested := range current {
			if err := validateSub2APILoginBodyTemplateValue(nested); err != nil {
				return err
			}
		}
	case []any:
		for _, nested := range current {
			if err := validateSub2APILoginBodyTemplateValue(nested); err != nil {
				return err
			}
		}
	case string:
		matches := sub2APILoginTemplatePattern.FindAllStringSubmatch(current, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			if !isSupportedSub2APILoginVariable(match[1]) {
				return fmt.Errorf("unsupported sub2api login template variable %q", "{{"+strings.TrimSpace(match[1])+"}}")
			}
		}
	}

	return nil
}

func renderSub2APILoginBodyTemplateValue(value any, vars map[string]string) (any, error) {
	switch current := value.(type) {
	case map[string]any:
		rendered := make(map[string]any, len(current))
		for key, nested := range current {
			next, err := renderSub2APILoginBodyTemplateValue(nested, vars)
			if err != nil {
				return nil, err
			}
			rendered[key] = next
		}
		return rendered, nil
	case []any:
		rendered := make([]any, 0, len(current))
		for _, nested := range current {
			next, err := renderSub2APILoginBodyTemplateValue(nested, vars)
			if err != nil {
				return nil, err
			}
			rendered = append(rendered, next)
		}
		return rendered, nil
	case string:
		return replaceSub2APILoginTemplateVariables(current, vars)
	default:
		return current, nil
	}
}

func replaceSub2APILoginTemplateVariables(value string, vars map[string]string) (string, error) {
	var renderErr error
	rendered := sub2APILoginTemplatePattern.ReplaceAllStringFunc(value, func(token string) string {
		if renderErr != nil {
			return token
		}

		match := sub2APILoginTemplatePattern.FindStringSubmatch(token)
		if len(match) < 2 {
			return token
		}

		key := strings.ToLower(strings.TrimSpace(match[1]))
		replacement, ok := vars[key]
		if !ok {
			renderErr = fmt.Errorf("unsupported sub2api login template variable %q", token)
			return token
		}
		return replacement
	})

	if renderErr != nil {
		return "", renderErr
	}
	return rendered, nil
}

func isSupportedSub2APILoginVariable(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "email", "password":
		return true
	default:
		return false
	}
}
