package ui

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/muhammadmuzzammil1998/jsonc"
)

// debugTemplateLoader enables detailed logging for template loading (set to true for debugging)
const debugTemplateLoader = false

// debugLog logs a message only if debugTemplateLoader is enabled
func debugLog(format string, args ...interface{}) {
	if debugTemplateLoader {
		log.Printf("TemplateLoader: "+format, args...)
	}
}

type TemplateData struct {
	ParserConfig    string
	Sections        map[string]json.RawMessage
	SectionOrder    []string
	SelectableRules []TemplateSelectableRule
	DefaultFinal    string
}

type TemplateSelectableRule struct {
	Label           string
	Description     string
	Raw             map[string]interface{}
	DefaultOutbound string
	HasOutbound     bool // true if rule has "outbound" field that can be selected
}

func loadTemplateData(execDir string) (*TemplateData, error) {
	templatePath := filepath.Join(execDir, "bin", "config_template.json")
	debugLog("Starting to load template from: %s", templatePath)
	raw, err := os.ReadFile(templatePath)
	if err != nil {
		debugLog("Failed to read template file: %v", err)
		return nil, err
	}
	debugLog("Successfully read template file, size: %d bytes", len(raw))

	rawStr := string(raw)
	parserConfig, cleaned := extractCommentBlock(rawStr, "ParcerConfig")
	debugLog("After extractCommentBlock, parserConfig length: %d, cleaned length: %d", len(parserConfig), len(cleaned))

	selectableBlocks, cleaned := extractAllSelectableBlocks(cleaned)
	debugLog("After extractAllSelectableBlocks, found %d blocks, cleaned length: %d", len(selectableBlocks), len(cleaned))
	if len(selectableBlocks) > 0 {
		for i, block := range selectableBlocks {
			debugLog("Block %d (first 100 chars): %s", i+1, truncateString(block, 100))
		}
	}

	// Validate JSON before parsing
	jsonBytes := jsonc.ToJSON([]byte(cleaned))
	debugLog("After jsonc.ToJSON, jsonBytes length: %d", len(jsonBytes))

	if !json.Valid(jsonBytes) {
		debugLog("JSON validation failed. First 500 chars: %s", truncateString(string(jsonBytes), 500))
		return nil, fmt.Errorf("invalid JSON after removing @SelectableRule blocks. This may indicate a syntax error in config_template.json")
	}

	debugLog("JSON is valid, proceeding to unmarshal")

	sections := make(map[string]json.RawMessage)
	if err := json.Unmarshal(jsonBytes, &sections); err != nil {
		debugLog("JSON unmarshal failed: %v", err)
		return nil, fmt.Errorf("failed to parse config_template.json: %w", err)
	}

	debugLog("Successfully unmarshaled %d sections", len(sections))

	sectionOrder := orderTemplateSections(sections)
	debugLog("Section order: %v", sectionOrder)

	defaultFinal := extractDefaultFinal(sections)
	if defaultFinal != "" {
		debugLog("Detected default final outbound: %s", defaultFinal)
	}

	selectableRules, err := parseSelectableRules(selectableBlocks)
	if err != nil {
		debugLog("parseSelectableRules failed: %v", err)
		return nil, err
	}

	debugLog("Successfully parsed %d selectable rules", len(selectableRules))

	result := &TemplateData{
		ParserConfig:    strings.TrimSpace(parserConfig),
		Sections:        sections,
		SectionOrder:    sectionOrder,
		SelectableRules: selectableRules,
		DefaultFinal:    defaultFinal,
	}

	debugLog("Successfully loaded template data with %d sections and %d selectable rules", len(sections), len(selectableRules))

	return result, nil
}

// truncateString truncates a string to maxLen characters, adding "..." if truncated
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func extractCommentBlock(src, marker string) (string, string) {
	pattern := regexp.MustCompile(`(?s)/\*\*\s*@` + marker + `\s*(.*?)\*/`)
	matches := pattern.FindStringSubmatch(src)
	if len(matches) < 2 {
		return "", src
	}
	cleaned := pattern.ReplaceAllString(src, "")
	return strings.TrimSpace(matches[1]), cleaned
}

func extractAllSelectableBlocks(src string) ([]string, string) {
	debugLog("extractAllSelectableBlocks: input length: %d", len(src))
	// Only support @SelectableRule
	// Match the block including optional leading/trailing commas, whitespace, and empty lines
	pattern := regexp.MustCompile(`(?is)(\s*,?\s*)/\*\*\s*@selectablerule\s*(.*?)\*/(\s*,?\s*)`)
	matches := pattern.FindAllStringSubmatch(src, -1)
	debugLog("extractAllSelectableBlocks: found %d matches", len(matches))
	if len(matches) == 0 {
		debugLog("extractAllSelectableBlocks: no matches, returning original source")
		return nil, src
	}

	// Extract blocks first before removing
	var blocks []string
	for _, m := range matches {
		if len(m) >= 3 {
			blocks = append(blocks, strings.TrimSpace(m[2]))
		}
	}
	debugLog("extractAllSelectableBlocks: extracted %d blocks", len(blocks))

	// Remove the blocks, including surrounding commas and whitespace
	// Use a more aggressive pattern that also removes empty lines after blocks
	cleaned := pattern.ReplaceAllString(src, "")
	debugLog("extractAllSelectableBlocks: after removing blocks, length: %d", len(cleaned))

	// Remove empty lines that might be left (lines with only whitespace)
	cleaned = regexp.MustCompile(`(?m)^\s*$\n?`).ReplaceAllString(cleaned, "")
	debugLog("extractAllSelectableBlocks: after removing empty lines, length: %d", len(cleaned))

	// Clean up any double commas that might result
	cleaned = regexp.MustCompile(`,\s*,`).ReplaceAllString(cleaned, ",")
	// Clean up comma before closing bracket
	cleaned = regexp.MustCompile(`,\s*\]`).ReplaceAllString(cleaned, "]")
	// Clean up comma after opening bracket
	cleaned = regexp.MustCompile(`\[\s*,`).ReplaceAllString(cleaned, "[")
	debugLog("extractAllSelectableBlocks: after cleaning commas, length: %d", len(cleaned))
	debugLog("extractAllSelectableBlocks: first 200 chars of cleaned: %s", truncateString(cleaned, 200))

	return blocks, cleaned
}

func parseSelectableRules(blocks []string) ([]TemplateSelectableRule, error) {
	debugLog("parseSelectableRules: incoming blocks (%d total)", len(blocks))
	for i, block := range blocks {
		debugLog("parseSelectableRules: incoming block %d raw (first 200 chars): %s", i+1, truncateString(block, 200))
	}

	if len(blocks) == 0 {
		debugLog("parseSelectableRules: no blocks provided, returning empty result")
		return nil, nil
	}

	var rules []TemplateSelectableRule
	for i, rawBlock := range blocks {
		debugLog("parseSelectableRules: processing block %d/%d", i+1, len(blocks))
		if strings.TrimSpace(rawBlock) == "" {
			debugLog("parseSelectableRules: block %d is empty after trimming, skipping", i+1)
			continue
		}

		label, description, cleanedBlock := extractRuleMetadata(rawBlock, i+1)
		debugLog("parseSelectableRules: block %d label='%s', description='%s'", i+1, label, description)
		debugLog("parseSelectableRules: block %d cleaned body (first 200 chars): %s", i+1, truncateString(cleanedBlock, 200))

		if cleanedBlock == "" {
			return nil, fmt.Errorf("selectable rule block %d has no JSON content", i+1)
		}

		jsonStr, err := normalizeRuleJSON(cleanedBlock, i+1)
		if err != nil {
			return nil, fmt.Errorf("selectable rule block %d: %w", i+1, err)
		}
		debugLog("parseSelectableRules: block %d normalized JSON (first 200 chars): %s", i+1, truncateString(jsonStr, 200))

		jsonBytes := jsonc.ToJSON([]byte(jsonStr))
		if !json.Valid(jsonBytes) {
			debugLog("parseSelectableRules: block %d JSON invalid after jsonc conversion (first 200 chars): %s", i+1, truncateString(string(jsonBytes), 200))
			return nil, fmt.Errorf("selectable rule block %d contains invalid JSON", i+1)
		}

		var items []map[string]interface{}
		if err := json.Unmarshal(jsonBytes, &items); err != nil {
			debugLog("parseSelectableRules: block %d JSON unmarshal failed: %v", i+1, err)
			return nil, fmt.Errorf("failed to parse selectable rule block %d: %w", i+1, err)
		}
		debugLog("parseSelectableRules: block %d parsed into %d item(s)", i+1, len(items))

		for _, item := range items {
			rule := TemplateSelectableRule{
				Raw:         make(map[string]interface{}),
				Label:       label,
				Description: description,
			}

			for key, value := range item {
				rule.Raw[key] = value
			}

			if rule.Label == "" {
				if labelVal, ok := item["label"]; ok {
					if labelStr, ok := labelVal.(string); ok {
						rule.Label = labelStr
					}
				}
			}

			if outboundVal, hasOutbound := item["outbound"]; hasOutbound {
				rule.HasOutbound = true
				if outboundStr, ok := outboundVal.(string); ok {
					rule.DefaultOutbound = outboundStr
				}
			}

			if rule.Label == "" {
				rule.Label = fmt.Sprintf("Rule %d", len(rules)+1)
			}

			rules = append(rules, rule)
		}
	}

	debugLog("parseSelectableRules: completed with %d rule(s)", len(rules))
	return rules, nil
}

func extractRuleMetadata(block string, blockIndex int) (string, string, string) {
	const (
		labelDirective = "@label"
		descDirective  = "@description"
	)

	var builder strings.Builder
	var label string
	var description string

	lines := strings.Split(block, "\n")
	for lineIdx, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, labelDirective):
			value := strings.TrimSpace(trimmed[len(labelDirective):])
			if value != "" {
				label = value
				debugLog("parseSelectableRules: block %d line %d label parsed: %s", blockIndex, lineIdx+1, value)
			}
			continue
		case strings.HasPrefix(trimmed, descDirective):
			value := strings.TrimSpace(trimmed[len(descDirective):])
			if value != "" {
				description = value
				debugLog("parseSelectableRules: block %d line %d description parsed: %s", blockIndex, lineIdx+1, value)
			}
			continue
		default:
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}

	cleaned := strings.TrimSpace(builder.String())
	debugLog("parseSelectableRules: block %d body length after removing directives: %d", blockIndex, len(cleaned))
	return label, description, cleaned
}

func normalizeRuleJSON(body string, blockIndex int) (string, error) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return "", fmt.Errorf("no JSON content after trimming block %d", blockIndex)
	}

	trimmed = strings.TrimRight(trimmed, " \t\r\n,")
	trimmed = strings.TrimSpace(trimmed)
	debugLog("parseSelectableRules: block %d body after trimming trailing commas (first 200 chars): %s", blockIndex, truncateString(trimmed, 200))

	if trimmed == "" {
		return "", fmt.Errorf("no JSON content remains in block %d after trimming", blockIndex)
	}

	if strings.HasPrefix(trimmed, "[") {
		return trimmed, nil
	}

	normalized := fmt.Sprintf("[%s]", trimmed)
	return normalized, nil
}

func orderTemplateSections(sections map[string]json.RawMessage) []string {
	defaultOrder := []string{"log", "dns", "inbounds", "outbounds", "route", "experimental", "rule_set", "rules"}
	ordered := make([]string, 0, len(sections))
	seen := make(map[string]bool)
	for _, key := range defaultOrder {
		if _, ok := sections[key]; ok {
			ordered = append(ordered, key)
			seen[key] = true
		}
	}
	for key := range sections {
		if !seen[key] {
			ordered = append(ordered, key)
		}
	}
	return ordered
}

func extractDefaultFinal(sections map[string]json.RawMessage) string {
	raw, ok := sections["route"]
	if !ok || len(raw) == 0 {
		return ""
	}
	var route map[string]interface{}
	if err := json.Unmarshal(raw, &route); err != nil {
		debugLog("extractDefaultFinal: failed to unmarshal route section: %v", err)
		return ""
	}
	if finalVal, ok := route["final"]; ok {
		if finalStr, ok := finalVal.(string); ok {
			return finalStr
		}
	}
	return ""
}
