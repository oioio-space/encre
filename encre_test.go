package encre_test

import (
	"fmt"
	"testing"

	encre "github.com/oioio-space/encre"
)

func TestGreet(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "named", in: "Go", want: "Hello, Go!"},
		{name: "empty defaults to world", in: "", want: "Hello, world!"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := encre.Greet(tt.in); got != tt.want {
				t.Errorf("Greet(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func ExampleGreet() {
	fmt.Println(encre.Greet("Go"))
	// Output: Hello, Go!
}
