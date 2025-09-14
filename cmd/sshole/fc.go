package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"sshole/pkg/clients"
	"strings"

	fc20230330 "github.com/alibabacloud-go/fc-20230330/v4/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/rs/zerolog/log"
)

func cmdFc(ctx context.Context) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Starting Fc")

	fc3Client, err := clients.GetFc3Client(ctx, clients.GetFc3ClientParams{
		Region:          cli.Fc.Region,
		AccessKeyId:     cli.Fc.AccessKeyId,
		AccessKeySecret: cli.Fc.AccessKeySecret,
		AccountID:       cli.Fc.AccountID,
	})
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to get fc3 client")
	}

	var codeBase64 string
	{
		// 1. 获取当前可执行文件路径
		exePath, err := os.Executable()
		if err != nil {
			log.Panic().Err(err).Msg("Failed to get executable path")
		}

		// 2. 读取可执行文件内容到内存
		exeData, err := os.ReadFile(exePath)
		if err != nil {
			log.Panic().Err(err).Msg("Failed to read executable file")
		}

		// 3. 创建一个内存缓冲区来保存 ZIP 数据
		var buf bytes.Buffer

		// 4. 创建 ZIP writer，写入内存缓冲区
		zipWriter := zip.NewWriter(&buf)
		defer zipWriter.Close()

		// 5. 向 ZIP 中添加文件（文件名设为 "binary" 或原文件名）
		fileWriter, err := zipWriter.Create("sshole")
		if err != nil {
			log.Panic().Err(err).Msg("Failed to create zip entry")
		}

		// 6. 将二进制内容写入 ZIP 条目
		_, err = fileWriter.Write(exeData)
		if err != nil {
			log.Panic().Err(err).Msg("Failed to write file to zip")
		}

		// 7. 关闭 ZIP 写入器（关键！确保数据被刷新）
		err = zipWriter.Close()
		if err != nil {
			log.Panic().Err(err).Msg("Failed to close zip writer")
		}

		// 8. 获取 ZIP 的完整字节数据
		zipData := buf.Bytes()

		// 9. 编码为 base64
		codeBase64 = base64.StdEncoding.EncodeToString(zipData)
	}

	const hubFunctionName = "_sshole_hub"

	{
		// 创建函数
		logger.Info().Msg("create hub function")

		resp, err := fc3Client.GetFunction(tea.String(hubFunctionName), &fc20230330.GetFunctionRequest{})
		if err != nil {
			if strings.Contains(err.Error(), "FunctionNotFound") {
				logger.Info().
					Msg("function not found, create it")

				input := &fc20230330.CreateFunctionInput{
					FunctionName: tea.String(hubFunctionName),
					Runtime:      tea.String("custom.debian11"),
					Handler:      tea.String("./sshole"),
					Code: &fc20230330.InputCodeLocation{
						ZipFile: tea.String(codeBase64),
					},
					CustomRuntimeConfig: &fc20230330.CustomRuntimeConfig{
						Command: []*string{
							tea.String("./sshole"),
							tea.String("hub"),
						},
						Port: tea.Int32(9000),
					},
				}

				resp, err := fc3Client.CreateFunction(&fc20230330.CreateFunctionRequest{
					Body: input,
				})
				if err != nil {
					logger.Panic().
						Err(err).
						Msg("create function failed")
				}
				logger.Info().
					Interface("resp", resp).
					Msg("create function")
			} else {
				logger.Info().
					Msg("function not exists")
			}
		} else {
			logger.Info().
				Interface("resp", resp).
				Msg("function exists")
		}
	}

}
