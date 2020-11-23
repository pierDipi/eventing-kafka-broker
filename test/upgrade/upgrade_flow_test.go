package upgrade

import (
	"reflect"
	"testing"
)

func TestExtractNamespaces(t *testing.T) {
	tests := []struct {
		name string
		m    string
		want []string
	}{
		{
			name: "single manifest no match",
			m: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm
data: |
  hello
`,
			want: []string{},
		},
		{
			name: "single manifest",
			m: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm
  namespace: my-namespace
`,
			want: []string{
				"my-namespace",
			},
		},
		{
			name: "multi manifest",
			m: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm
  namespace: my-namespace
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm
  namespace: my-namespace-2
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm
data: |
  hello
`,
			want: []string{
				"my-namespace",
				"my-namespace-2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractNamespaces(tt.m); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractNamespaces() = %v, want %v", got, tt.want)
			}
		})
	}
}
