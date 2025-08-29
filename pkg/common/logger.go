package common

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type InitLoggerOption struct {
	Component string
}

func InitLogger(ctx context.Context, option InitLoggerOption) context.Context {
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		return filepath.Base(file) + ":" + strconv.Itoa(line)
	}
	zerolog.TimeFieldFormat = "2006-01-02 15:04:05.000"

	writers := []io.Writer{os.Stdout}
	writer := io.MultiWriter(writers...)

	logger := log.Output(zerolog.ConsoleWriter{Out: writer, TimeFormat: "2006-01-02 15:04:05.000", NoColor: false, FormatCaller: func(i any) string {
		var c string
		if cc, ok := i.(string); ok {
			c = cc
		}
		if len(c) > 0 {
			c = fmt.Sprintf("[%v] %v >", option.Component, c)
		} else {
			c = fmt.Sprintf("[%v] >", option.Component)
		}
		return c
	},
	}).Level(zerolog.DebugLevel).With().Caller().Logger()
	ctx = logger.WithContext(ctx)
	return ctx
}
