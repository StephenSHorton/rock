package childproc

import (
	"os/exec"
	"runtime"
	"testing"
)

func TestIsolate(t *testing.T) {
	Isolate(nil) // must not panic
	cmd := exec.Command("true")
	Isolate(cmd)
	got := Flags(cmd)
	if runtime.GOOS == "windows" {
		if got&0x08000000 == 0 {
			t.Fatalf("CREATE_NO_WINDOW not set: %#x", got)
		}
		return
	}
	if got != 0 || cmd.SysProcAttr != nil {
		t.Fatalf("non-windows should leave SysProcAttr alone: %#x %+v", got, cmd.SysProcAttr)
	}
}
