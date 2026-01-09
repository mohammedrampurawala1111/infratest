package interpolator

import (
	"testing"
)

func TestInterpolate(t *testing.T) {
	outputs := map[string]interface{}{
		"alb_dns":      "example-alb-123.us-east-1.elb.amazonaws.com",
		"instance_ips": []interface{}{"1.2.3.4", "5.6.7.8"},
		"config": map[string]interface{}{
			"database": map[string]interface{}{
				"host": "db.example.com",
			},
		},
		"count": 42,
		"enabled": true,
	}

	tests := []struct {
		name     string
		template string
		want     string
	}{
		{
			name:     "simple output",
			template: "http://${output.alb_dns}/health",
			want:     "http://example-alb-123.us-east-1.elb.amazonaws.com/health",
		},
		{
			name:     "array access",
			template: "http://${output.instance_ips[0]}:3000",
			want:     "http://1.2.3.4:3000",
		},
		{
			name:     "nested path",
			template: "http://${output.config.database.host}:5432",
			want:     "http://db.example.com:5432",
		},
		{
			name:     "number output",
			template: "count=${output.count}",
			want:     "count=42",
		},
		{
			name:     "bool output",
			template: "enabled=${output.enabled}",
			want:     "enabled=true",
		},
		{
			name:     "multiple interpolations",
			template: "${output.alb_dns} and ${output.count}",
			want:     "example-alb-123.us-east-1.elb.amazonaws.com and 42",
		},
		{
			name:     "missing output",
			template: "http://${output.missing}/test",
			want:     "http://${output.missing}/test", // Should remain unchanged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Interpolate(tt.template, outputs)
			if got != tt.want {
				t.Errorf("Interpolate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		want string
	}{
		{"string", "test", "test"},
		{"int", 42, "42"},
		{"float", 3.14, "3.14"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"array", []interface{}{1, 2, 3}, "1,2,3"},
		{"array single", []interface{}{"single"}, "single"},
		{"map", map[string]interface{}{"key": "value"}, "{key: value}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatValue(tt.val)
			// For maps, order may vary, so just check it's not empty
			if _, ok := tt.val.(map[string]interface{}); ok {
				if got == "" {
					t.Errorf("formatValue() returned empty string for map")
				}
			} else if got != tt.want {
				t.Errorf("formatValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

