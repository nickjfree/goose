package route


import (
	"context"
	"time"
	"os/exec"
	"github.com/pkg/errors"
)


func RunCmd(name string, cmdStr ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10000 * time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, cmdStr...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, errors.Wrapf(err, "cmd %+v failed output %s", cmd, string(out))
	} else {
		return out, nil
	}
}






