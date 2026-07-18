package uptimekuma

import "testing"

func TestParseMetrics(t *testing.T) {
	input := `# HELP monitor_status Monitor status
monitor_status{monitor_name="Jellyfin",monitor_type="http",monitor_url="http://jellyfin:8096"} 1
monitor_response_time{monitor_name="Jellyfin",monitor_type="http",monitor_url="http://jellyfin:8096"} 0.123
monitor_status{monitor_name="DNS \"internal\"",monitor_type="dns"} 0`

	metrics, err := ParseMetrics(input)
	if err != nil {
		t.Fatalf("ParseMetrics() error = %v", err)
	}
	if len(metrics) != 3 {
		t.Fatalf("got %d metrics, want 3", len(metrics))
	}
	if metrics[0].Labels["monitor_name"] != "Jellyfin" || metrics[0].Value != 1 {
		t.Fatalf("unexpected first metric: %#v", metrics[0])
	}
	if metrics[2].Labels["monitor_name"] != `DNS "internal"` {
		t.Fatalf("escaped label was not decoded: %#v", metrics[2].Labels)
	}
}
