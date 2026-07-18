package uptimekuma

import (
	"fmt"
	"strconv"
	"strings"
)

type Metric struct {
	Name   string
	Labels map[string]string
	Value  float64
}

func ParseMetrics(text string) ([]Metric, error) {
	lines := strings.Split(text, "\n")
	metrics := make([]Metric, 0, len(lines))
	for lineNumber, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		metric, err := parseMetricLine(line)
		if err != nil {
			return nil, fmt.Errorf("parse metrics line %d: %w", lineNumber+1, err)
		}
		metrics = append(metrics, metric)
	}
	return metrics, nil
}

func parseMetricLine(line string) (Metric, error) {
	braceDepth := 0
	splitAt := -1
	for index := 0; index < len(line); index++ {
		switch line[index] {
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		case ' ', '\t':
			if braceDepth == 0 {
				splitAt = index
			}
		}
		if splitAt >= 0 {
			break
		}
	}
	if splitAt < 0 {
		return Metric{}, fmt.Errorf("missing metric value")
	}

	metricPart := strings.TrimSpace(line[:splitAt])
	valueFields := strings.Fields(strings.TrimSpace(line[splitAt:]))
	if len(valueFields) == 0 {
		return Metric{}, fmt.Errorf("missing metric value")
	}
	value, err := strconv.ParseFloat(valueFields[0], 64)
	if err != nil {
		return Metric{}, fmt.Errorf("invalid metric value")
	}

	metric := Metric{Labels: map[string]string{}, Value: value}
	if open := strings.IndexByte(metricPart, '{'); open >= 0 {
		if !strings.HasSuffix(metricPart, "}") {
			return Metric{}, fmt.Errorf("invalid label block")
		}
		metric.Name = metricPart[:open]
		labels, err := parseLabels(metricPart[open+1 : len(metricPart)-1])
		if err != nil {
			return Metric{}, err
		}
		metric.Labels = labels
	} else {
		metric.Name = metricPart
	}
	if metric.Name == "" {
		return Metric{}, fmt.Errorf("missing metric name")
	}
	return metric, nil
}

func parseLabels(text string) (map[string]string, error) {
	labels := make(map[string]string)
	for index := 0; index < len(text); {
		for index < len(text) && (text[index] == ' ' || text[index] == '\t' || text[index] == ',') {
			index++
		}
		if index == len(text) {
			break
		}
		start := index
		for index < len(text) && text[index] != '=' && text[index] != ',' {
			index++
		}
		if index >= len(text) || text[index] != '=' {
			return nil, fmt.Errorf("invalid label")
		}
		key := strings.TrimSpace(text[start:index])
		index++
		if index >= len(text) || text[index] != '"' {
			return nil, fmt.Errorf("invalid label value")
		}
		index++
		var value strings.Builder
		terminated := false
		for index < len(text) {
			character := text[index]
			index++
			if character == '"' {
				terminated = true
				break
			}
			if character == '\\' && index < len(text) {
				escaped := text[index]
				index++
				switch escaped {
				case 'n':
					value.WriteByte('\n')
				case 'r':
					value.WriteByte('\r')
				case 't':
					value.WriteByte('\t')
				default:
					value.WriteByte(escaped)
				}
				continue
			}
			value.WriteByte(character)
		}
		if !terminated || key == "" {
			return nil, fmt.Errorf("invalid label")
		}
		labels[key] = value.String()
	}
	return labels, nil
}
