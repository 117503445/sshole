package main

import (
	"archive/zip"
	"context"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
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

	// 添加文件到 zip 文件中
	addFileToZip := func(zipWriter *zip.Writer, filename string) error {
		fileToZip, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer fileToZip.Close()

		// 获取文件信息
		info, err := fileToZip.Stat()
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// 可选：设置压缩方式
		// header.Method = zip.Store
		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		_, err = io.Copy(writer, fileToZip)
		return err
	}

	var codeBase64 string
	{
		// 1. 创建临时目录
		tempDir, err := os.MkdirTemp("", "zip-example-*")
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(tempDir) // 清理临时目录

		// 2. 定义 ZIP 文件路径（在临时目录中）
		zipPath := filepath.Join(tempDir, "output.zip")
		// 3. 创建 ZIP 文件
		zipFile, err := os.Create(zipPath)
		if err != nil {
			log.Panic().Err(err).Send()
		}
		defer zipFile.Close()
		// 4. 初始化 ZIP writer
		zipWriter := zip.NewWriter(zipFile)

		exePath, err := os.Executable()
		if err != nil {
			logger.Panic().Err(err).Msg("Failed to get executable path")
		}

		// 5. 要打包的文件列表
		filesToZip := []string{
			exePath,
		}
		for _, filename := range filesToZip {
			if err := addFileToZip(zipWriter, filename); err != nil {
				log.Panic().Err(err).Send()
			}
		}
		zipWriter.Close()

		codeBase64 = base64.StdEncoding.EncodeToString(body)
	}

	const hubFunctionName = "_sshole_hub"

	{
		// 创建函数
		logger.Info().Msg("create layer build function")

		resp, err := fc3Client.GetFunction(tea.String(hubFunctionName), &fc20230330.GetFunctionRequest{})
		if err != nil {
			if strings.Contains(err.Error(), "FunctionNotFound") {
				logger.Info().
					Msg("function not found, create it")

				input := &fc20230330.CreateFunctionInput{
					FunctionName: tea.String(hubFunctionName),
					Runtime:      tea.String("go1"),
					Handler:      tea.String("/sshole"),
					Code: &fc20230330.InputCodeLocation{
						ZipFile: tea.String(codeBase64),
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
