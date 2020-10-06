package proxy //nolint

import (
	"testing"
)

func Test_getPathWithoutPrefix(t *testing.T) {
	type args struct {
		path   string
		prefix string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "#1", args: args{
			path:   "/api/some/11/22",
			prefix: "/api/some",
		}, want: "11/22"},
		{name: "#2", args: args{
			path:   "/api/service/group/method",
			prefix: "/api",
		}, want: "service/group/method"},
	}
	for _, test := range tests {
		tt := test
		t.Run(tt.name, func(t *testing.T) {
			if got := getPathWithoutPrefix(tt.args.path, tt.args.prefix); got != tt.want {
				t.Errorf("getPathWithoutPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}
