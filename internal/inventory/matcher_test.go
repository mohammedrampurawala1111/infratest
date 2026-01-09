package inventory

import (
	"testing"
)

func TestMatcher_Match(t *testing.T) {
	resources := []Resource{
		{
			Type:       "aws_vpc",
			Name:       "main",
			Address:    "aws_vpc.main",
			ID:         "vpc-123",
			Attributes: map[string]interface{}{"cidr_block": "10.0.0.0/16", "enable_dns_hostnames": true},
		},
		{
			Type:       "aws_subnet",
			Name:       "public",
			Address:    "aws_subnet.public",
			ID:         "subnet-123",
			Attributes: map[string]interface{}{"map_public_ip_on_launch": true},
		},
		{
			Type:       "aws_subnet",
			Name:       "private",
			Address:    "aws_subnet.private",
			ID:         "subnet-456",
			Attributes: map[string]interface{}{"map_public_ip_on_launch": false},
		},
		{
			Type:       "aws_internet_gateway",
			Name:       "main",
			Address:    "aws_internet_gateway.main",
			ID:         "igw-123",
			Attributes: map[string]interface{}{},
		},
	}

	matcher := NewMatcher(resources)

	tests := []struct {
		name     string
		expected map[string]ResourceMatch
		wantErr  bool
	}{
		{
			name: "exact match",
			expected: map[string]ResourceMatch{
				"aws_vpc.main": {
					Type:  "aws_vpc",
					Name:  "main",
					Count: intPtr(1),
					Attributes: map[string]interface{}{
						"cidr_block": "10.0.0.0/16",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "wildcard match",
			expected: map[string]ResourceMatch{
				"aws_subnet.*": {
					Type:     "aws_subnet",
					Name:     ".*", // Wildcard pattern
					MinCount: intPtr(2),
					MaxCount: intPtr(3),
				},
			},
			wantErr: false,
		},
		{
			name: "attribute mismatch",
			expected: map[string]ResourceMatch{
				"aws_vpc.main": {
					Type:  "aws_vpc",
					Name:  "main",
					Count: intPtr(1),
					Attributes: map[string]interface{}{
						"cidr_block": "192.168.0.0/16", // Wrong CIDR
					},
				},
			},
			wantErr: true,
		},
		{
			name: "count mismatch",
			expected: map[string]ResourceMatch{
				"aws_vpc.main": {
					Type:  "aws_vpc",
					Name:  "main",
					Count: intPtr(2), // Expected 2, but only 1 exists
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, issues := matcher.Match(tt.expected)
			
			hasError := len(issues) > 0
			for _, result := range results {
				if !result.Matched {
					hasError = true
				}
			}

			if hasError != tt.wantErr {
				t.Errorf("Match() error = %v, wantErr %v. Issues: %v", hasError, tt.wantErr, issues)
			}
		})
	}
}

func TestMatcher_getNestedAttribute(t *testing.T) {
	attrs := map[string]interface{}{
		"tags": map[string]interface{}{
			"Name": "test-vpc",
			"Env":  "dev",
		},
		"cidr_block": "10.0.0.0/16",
	}

	matcher := &Matcher{}

	tests := []struct {
		name     string
		path     string
		want     interface{}
		wantErr  bool
	}{
		{
			name:    "simple attribute",
			path:    "cidr_block",
			want:    "10.0.0.0/16",
			wantErr: false,
		},
		{
			name:    "nested attribute",
			path:    "tags.Name",
			want:    "test-vpc",
			wantErr: false,
		},
		{
			name:    "nested attribute 2",
			path:    "tags.Env",
			want:    "dev",
			wantErr: false,
		},
		{
			name:    "missing attribute",
			path:    "missing",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid nested path",
			path:    "cidr_block.invalid",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := matcher.getNestedAttribute(attrs, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("getNestedAttribute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("getNestedAttribute() = %v, want %v", got, tt.want)
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}

