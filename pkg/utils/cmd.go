package utils

import (
	"context"
	"log"
	"time"
	"os"
	"os/exec"
	"github.com/pkg/errors"
)

var (
	logger = log.New(os.Stdout, "route: ", log.Lshortfile)
)


func RunCmd(name string, cmdStr ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 300 * time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, cmdStr...)
	if out, err := cmd.CombinedOutput(); err != nil {
		logger.Printf("run cmd %s failed(%+v) %s", cmd, err, string(out))
		return nil, errors.Wrapf(err, "cmd %+v failed output %s", cmd, string(out))
	} else {
		return out, nil
	}
}
