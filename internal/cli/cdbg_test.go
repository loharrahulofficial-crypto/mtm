package cli

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/spf13/cobra"
)

func TestCompletionDebug3(t *testing.T) {
	root := &cobra.Command{Use: "mtm"}
	comp := newCompletionCmd(root)
	root.AddCommand(comp)
	fmt.Printf("comp.Use=%s parent=%v children=%d\n", comp.Use, comp.Parent() == root, len(comp.Commands()))
	for _, c := range comp.Commands() {
		fmt.Printf("child: %s\n", c.Use)
	}
	comp.SetArgs([]string{"bash"})
	var buf bytes.Buffer
	comp.SetOut(&buf)
	comp.SetErr(&buf)
	err := comp.Execute()
	fmt.Printf("err=%v len=%d\n", err, buf.Len())
}
