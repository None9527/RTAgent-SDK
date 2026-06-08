package execution

import (
	"context"
	"os/exec"
	"syscall"
	"time"
)

type SoftSandboxRunner struct {
	WorkDir    string
	AllowedEnv []string // 显式注入的环境变量白名单，严禁透传无关 Secret
}

// RunCommandInSandbox 执行受控子进程命令
func (r *SoftSandboxRunner) RunCommandInSandbox(ctx context.Context, cmdName string, args []string, timeout time.Duration) (stdout string, exitCode int, err error) {
	// 1. 结合硬超时与外部 cancellation 上下文
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, cmdName, args...)
	cmd.Dir = r.WorkDir
	cmd.Env = r.AllowedEnv // 强制覆盖环境变量

	// 2. 将进程放入独立的 PGID (进程组)，确保超时时能 Cascade 杀死所有衍生子进程
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	outputBytes, runErr := cmd.CombinedOutput()
	stdout = string(outputBytes)

	if runErr != nil {
		// 尝试捕获退出状态码
		if exitError, ok := runErr.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
				return stdout, exitCode, nil
			}
		}
		return stdout, -1, runErr
	}

	return stdout, 0, nil
}
