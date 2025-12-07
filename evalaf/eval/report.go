package eval

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Print prints the report to stdout in a human-readable format.
func (r *Report) Print() {
	r.PrintTo(os.Stdout)
}

// PrintTo prints the report to the specified writer.
func (r *Report) PrintTo(w io.Writer) {
	fmt.Fprintf(w, "Evaluation Report\n")
	fmt.Fprintf(w, "=================\n\n")

	fmt.Fprintf(w, "Timestamp: %s\n", r.Timestamp.Format(time.RFC3339))
	fmt.Fprintf(w, "Duration: %s\n\n", r.Summary.TotalDuration)

	fmt.Fprintf(w, "Summary\n")
	fmt.Fprintf(w, "-------\n")
	fmt.Fprintf(w, "Total Examples: %d\n", r.Summary.TotalExamples)
	fmt.Fprintf(w, "Passed: %d (%.1f%%)\n", r.Summary.PassedExamples, r.Summary.PassRate*100)
	fmt.Fprintf(w, "Failed: %d (%.1f%%)\n", r.Summary.FailedExamples, (1-r.Summary.PassRate)*100)
	fmt.Fprintf(w, "Average Score: %.3f\n\n", r.Summary.AverageScore)

	if len(r.Summary.EvaluatorStats) > 0 {
		fmt.Fprintf(w, "Evaluator Statistics\n")
		fmt.Fprintf(w, "--------------------\n")
		for name, stats := range r.Summary.EvaluatorStats {
			fmt.Fprintf(w, "\n%s:\n", name)
			fmt.Fprintf(w, "  Total Evaluations: %d\n", stats.TotalEvaluations)
			fmt.Fprintf(w, "  Passed: %d (%.1f%%)\n", stats.Passed, stats.PassRate*100)
			fmt.Fprintf(w, "  Failed: %d\n", stats.Failed)
			if stats.Errors > 0 {
				fmt.Fprintf(w, "  Errors: %d\n", stats.Errors)
			}
			fmt.Fprintf(w, "  Average Score: %.3f\n", stats.AverageScore)
			fmt.Fprintf(w, "  Average Duration: %s\n", stats.AverageDuration)
		}
		fmt.Fprintf(w, "\n")
	}

	// Print failed examples
	failedCount := 0
	for i, result := range r.Results {
		hasFailed := false
		for _, evalResult := range result.Results {
			if evalResult.Error != nil || !evalResult.Pass {
				hasFailed = true
				break
			}
		}

		if hasFailed {
			if failedCount == 0 {
				fmt.Fprintf(w, "Failed Examples\n")
				fmt.Fprintf(w, "---------------\n\n")
			}
			failedCount++

			fmt.Fprintf(w, "Example %d:\n", i+1)
			fmt.Fprintf(w, "  Input: %v\n", result.Example.Input)
			if result.Example.Reference != nil {
				fmt.Fprintf(w, "  Reference: %v\n", result.Example.Reference)
			}
			fmt.Fprintf(w, "  Output: %v\n", result.Output)

			for evalName, evalResult := range result.Results {
				if evalResult.Error != nil {
					fmt.Fprintf(w, "  %s: ERROR - %v\n", evalName, evalResult.Error)
				} else if !evalResult.Pass {
					fmt.Fprintf(w, "  %s: FAIL (score: %.3f) - %s\n", evalName, evalResult.Score, evalResult.Reason)
				}
			}
			fmt.Fprintf(w, "\n")
		}
	}

	if failedCount == 0 {
		fmt.Fprintf(w, "All examples passed!\n")
	}
}

// ToJSON converts the report to JSON.
func (r *Report) ToJSON(pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(r, "", "  ")
	}
	return json.Marshal(r)
}

// ToYAML converts the report to YAML.
func (r *Report) ToYAML() ([]byte, error) {
	return yaml.Marshal(r)
}

// formatValueForMarkdown formats any value for markdown display.
// For maps and complex types, it renders as JSON. For simple types, uses string conversion.
func formatValueForMarkdown(v any) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case string:
		return val
	case map[string]any, map[string]string, []any, []string:
		data, err := json.MarshalIndent(val, "", "  ")
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(data)
	default:
		// Try JSON marshaling for other complex types
		data, err := json.MarshalIndent(val, "", "  ")
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		// If it just marshals to a simple value, return that directly
		result := string(data)
		if strings.HasPrefix(result, "\"") && strings.HasSuffix(result, "\"") {
			// Unwrap simple strings
			return result[1 : len(result)-1]
		}
		return result
	}
}

// ToMarkdown converts the report to Markdown format.
func (r *Report) ToMarkdown() string {
	var sb strings.Builder

	sb.WriteString("# Evaluation Report\n\n")
	sb.WriteString(fmt.Sprintf("**Timestamp**: %s\n\n", r.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Duration**: %s\n\n", r.Summary.TotalDuration))

	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Total Examples**: %d\n", r.Summary.TotalExamples))
	sb.WriteString(fmt.Sprintf("- **Passed**: %d (%.1f%%)\n", r.Summary.PassedExamples, r.Summary.PassRate*100))
	sb.WriteString(fmt.Sprintf("- **Failed**: %d (%.1f%%)\n", r.Summary.FailedExamples, (1-r.Summary.PassRate)*100))
	sb.WriteString(fmt.Sprintf("- **Average Score**: %.3f\n\n", r.Summary.AverageScore))

	if len(r.Summary.EvaluatorStats) > 0 {
		sb.WriteString("## Evaluator Statistics\n\n")
		sb.WriteString("| Evaluator | Total | Passed | Failed | Errors | Avg Score | Avg Duration |\n")
		sb.WriteString("|-----------|-------|--------|--------|--------|-----------|-------------|\n")

		for name, stats := range r.Summary.EvaluatorStats {
			sb.WriteString(fmt.Sprintf("| %s | %d | %d (%.1f%%) | %d | %d | %.3f | %s |\n",
				name,
				stats.TotalEvaluations,
				stats.Passed,
				stats.PassRate*100,
				stats.Failed,
				stats.Errors,
				stats.AverageScore,
				stats.AverageDuration,
			))
		}
		sb.WriteString("\n")
	}

	// Show failed examples
	failedCount := 0
	for i, result := range r.Results {
		hasFailed := false
		for _, evalResult := range result.Results {
			if evalResult.Error != nil || !evalResult.Pass {
				hasFailed = true
				break
			}
		}

		if hasFailed {
			if failedCount == 0 {
				sb.WriteString("## Failed Examples\n\n")
			}
			failedCount++

			sb.WriteString(fmt.Sprintf("### Example %d\n\n", i+1))
			sb.WriteString(fmt.Sprintf("**Input**: %s\n\n", result.Example.Input))
			if result.Example.Reference != nil {
				sb.WriteString(fmt.Sprintf("**Reference**: %s\n\n", result.Example.Reference))
			}

			// Format output nicely
			outputStr := formatValueForMarkdown(result.Output)
			if strings.Contains(outputStr, "\n") {
				sb.WriteString("**Output**:\n\n```json\n")
				sb.WriteString(outputStr)
				sb.WriteString("\n```\n\n")
			} else {
				sb.WriteString(fmt.Sprintf("**Output**: %s\n\n", outputStr))
			}

			// Show retrieved context documents
			if len(result.Example.Context) > 0 {
				sb.WriteString("**Retrieved Documents**:\n\n")
				for j, doc := range result.Example.Context {
					docStr := formatValueForMarkdown(doc)

					// Truncate long documents
					if len(docStr) > 500 {
						docStr = docStr[:500] + "..."
					}

					sb.WriteString(fmt.Sprintf("**Doc %d**:\n", j+1))

					// Use code block for multi-line content
					if strings.Contains(docStr, "\n") {
						sb.WriteString("```json\n")
						sb.WriteString(docStr)
						sb.WriteString("\n```\n\n")
					} else {
						docStr = strings.ReplaceAll(docStr, "`", "\\`")
						sb.WriteString(fmt.Sprintf("`%s`\n\n", docStr))
					}
				}
			}

			sb.WriteString("**Results**:\n\n")
			for evalName, evalResult := range result.Results {
				if evalResult.Error != nil {
					sb.WriteString(fmt.Sprintf("- **%s**: ❌ ERROR - %v\n", evalName, evalResult.Error))
				} else if !evalResult.Pass {
					sb.WriteString(fmt.Sprintf("- **%s**: ❌ FAIL (score: %.3f) - %s\n", evalName, evalResult.Score, evalResult.Reason))
				} else {
					sb.WriteString(fmt.Sprintf("- **%s**: ✅ PASS (score: %.3f)\n", evalName, evalResult.Score))
				}
			}
			sb.WriteString("\n")
		}
	}

	if failedCount == 0 {
		sb.WriteString("## Results\n\n")
		sb.WriteString("✅ All examples passed!\n")
	}

	return sb.String()
}

// SaveToFile saves the report to a file in the specified format.
func (r *Report) SaveToFile(path string, format string, pretty bool) error {
	var data []byte
	var err error

	switch format {
	case "json":
		data, err = r.ToJSON(pretty)
	case "yaml", "yml":
		data, err = r.ToYAML()
	case "markdown", "md":
		data = []byte(r.ToMarkdown())
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return fmt.Errorf("failed to convert report to %s: %w", format, err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write report to file: %w", err)
	}

	return nil
}

// Save saves the report using the output configuration.
func (r *Report) Save() error {
	if r.Config.Output.Path == "" {
		return nil // No output path configured
	}

	format := r.Config.Output.Format
	if format == "" {
		format = "json"
	}

	return r.SaveToFile(r.Config.Output.Path, format, r.Config.Output.Pretty)
}
