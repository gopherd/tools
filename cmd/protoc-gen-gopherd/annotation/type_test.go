package annotation

import (
	"regexp"
	"testing"
)

func TestParseTypesProto(t *testing.T) {
	const content = `syntax = "proto3";

package test;

enum MessageType {
}

// other comment
`

	//result := typesProtoRegexp.FindAllSubmatch([]byte(content), -1)
	result := regexp.MustCompile(`(?ms)(.*)enum[[:space:]]?([a-zA-Z_]?[a-zA-Z0-9_]*)[[:space:]]?{(.*)}(.*)`).FindAllStringSubmatch(content, -1)
	for i := range result {
		for j := range result[i] {
			t.Logf("%q", result[i][j])
		}
	}
}
