package oauth

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenBrowser attempts to open the URL in the default browser
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	
	return cmd.Start()
}
