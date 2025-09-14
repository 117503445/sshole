package utils

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/117503445/goutils"
	"github.com/rs/zerolog/log"
)

func Command(cmds ...string) (cmd *exec.Cmd) {
	if len(cmds) == 0 {
		panic("no command")
	}

	if len(cmds) == 1 {
		parts := strings.Split(cmds[0], " ")
		if len(parts) > 1 {
			return exec.Command(parts[0], parts[1:]...)
		} else if len(parts) == 1 {
			return exec.Command(cmds[0])
		} else {
			panic("invalid command")
		}
	}

	return exec.Command(cmds[0], cmds[1:]...)
}

type ExecuteParams struct {
	Cmd                      *exec.Cmd
	Dir                      string
	CallerWithSkipFrameCount int
	DisableStdout            bool
	AllowFailed              bool
	DisableLog               bool
	Envs                     map[string]string
}

type ExecuteResult struct {
	Output    string
	ExitCode  int
	ExecuteID string
}

func Execute(ctx context.Context, params ExecuteParams) ExecuteResult {
	callerWithSkipFrameCount := params.CallerWithSkipFrameCount
	if callerWithSkipFrameCount == 0 {
		callerWithSkipFrameCount = 3
	}

	if params.Cmd == nil {
		panic("invalid command")
	}

	cmd := params.Cmd

	if params.Dir != "" {
		cmd.Dir = params.Dir
	}
	if cmd.Dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			panic(err)
		}

		cmd.Dir = wd
	}

	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	for k, v := range params.Envs {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Env = append(cmd.Env, "TZ=Asia/Shanghai")

	executeID := goutils.TimeStrMilliSec()
	executeLogID := executeID[len(executeID)-3:]

	logger := log.Ctx(ctx).With().CallerWithSkipFrameCount(callerWithSkipFrameCount).
		Str("execID", executeID).
		Str("command", cmd.String()).
		Str("dir", cmd.Dir).
		Logger()

	if !params.DisableLog {
		logger.Info().
			Msg("executing")
	}

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	startAt := time.Now()

	var outputBuf strings.Builder // 用于收集输出内容
	var exitCode int

	if err := cmd.Start(); err != nil {
		logger.Panic().Err(err).Msg("Failed to start command")
	}

	// 在 goroutine 中读取输出
	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(pr)

		var f *os.File

		// topic := GetMainContext(ctx).Topic
		for scanner.Scan() {
			line := scanner.Text()
			if !params.DisableStdout {
				fmt.Println(time.Now().Format("15:04:05.000"), executeLogID, "|", line)
			}

			outputBuf.WriteString(line)
			outputBuf.WriteString("\n") // 注意：scanner 已经去掉了换行，这里补上

			if f != nil {
				if _, err := f.WriteString(line + "\n"); err != nil {
					logger.Error().Err(err).Msg("Error writing to log file")
				} else {
					if err := f.Sync(); err != nil {
						logger.Error().Err(err).Msg("Error syncing log file")
					}
				}
			}
		}
		if err := scanner.Err(); err != nil {
			logger.Error().Err(err).Msg("Error scanning output")
		}
	}()

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		logger.Error().
			Err(err).
			Str("duration", goutils.DurationToStr(time.Since(startAt))).
			Msg("Command failed")
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			logger.Warn().
				Err(err).
				Msg("err is not ExitError")
			exitCode = -1
		}
	} else {
		if !params.DisableLog {
			logger.Info().
				Str("duration", goutils.DurationToStr(time.Since(startAt))).
				Msg("executed")
		}
	}

	pw.Close() // 关闭写入端，触发 pr EOF
	// 等待 goroutine 完成读取
	<-done

	output := outputBuf.String()

	if exitCode != 0 {
		if !params.AllowFailed {
			logger.Panic().
				Int("exitCode", exitCode).
				Str("output", output).
				Msg("MustSuccess")
		} else {
			logger.Info().
				Int("exitCode", exitCode).
				Msg("AllowFailed")
		}
	}

	return ExecuteResult{
		ExitCode:  exitCode,
		Output:    output,
		ExecuteID: executeID,
	}
}
