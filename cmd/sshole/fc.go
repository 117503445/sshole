package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"sshole/pkg/clients"
	"strings"
	"time"

	fc "github.com/aliyun/fc-go-sdk"
	chclient "github.com/jpillora/chisel/client"

	"net/http"
	rpcv1 "sshole/pkg/rpc/v1"
	"sshole/pkg/rpc/v1/rpcv1connect"

	"connectrpc.com/connect"
	"github.com/117503445/goutils"
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

	// exeData 的 sha256
	var codeHash string
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

		sha256hash := sha256.Sum256(exeData)
		codeHash = fmt.Sprintf("%x", sha256hash)

		// 3. 创建一个内存缓冲区来保存 ZIP 数据
		var buf bytes.Buffer

		// 4. 创建 ZIP writer，写入内存缓冲区
		zipWriter := zip.NewWriter(&buf)
		defer zipWriter.Close()

		// 5. 创建自定义 FileHeader，设置权限为 0755（-rwxr-xr-x）
		header := &zip.FileHeader{
			Name:   "sshole",    // ZIP 内部文件名
			Method: zip.Deflate, // 压缩方式（可选）
		}

		// 👇 关键：设置 Unix 权限为 0755（可执行！）
		// 注意：必须使用 *nix 风格权限位，且高位要设为目录标志（040000）或普通文件（0100000）
		header.SetMode(0755) // 这会自动转换为正确的外部文件属性格式

		// 6. 使用 CreateHeader 创建带有权限的 ZIP 条目
		fileWriter, err := zipWriter.CreateHeader(header)
		if err != nil {
			log.Panic().Err(err).Msg("Failed to create zip entry with header")
		}

		// 7. 将二进制内容写入 ZIP 条目
		_, err = fileWriter.Write(exeData)
		if err != nil {
			log.Panic().Err(err).Msg("Failed to write file to zip")
		}

		// 8. 关闭 ZIP 写入器（关键！确保数据被刷新）
		err = zipWriter.Close()
		if err != nil {
			log.Panic().Err(err).Msg("Failed to close zip writer")
		}

		// 9. 获取 ZIP 的完整字节数据
		zipData := buf.Bytes()

		// 10. 编码为 base64
		codeBase64 = base64.StdEncoding.EncodeToString(zipData)
	}

	const hubFunctionName = "_sshole_hub"

	// 创建函数
	{
		logger.Info().Msg("create hub function")

		getResp, err := fc3Client.GetFunction(tea.String(hubFunctionName), &fc20230330.GetFunctionRequest{})
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
					Timeout:             tea.Int32(86400),
					InstanceConcurrency: tea.Int32(200),
					EnvironmentVariables: map[string]*string{
						"CODE_HASH": tea.String(codeHash),
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

				_, err = fc3Client.PutConcurrencyConfig(tea.String(hubFunctionName), &fc20230330.PutConcurrencyConfigRequest{
					Body: &fc20230330.PutConcurrencyInput{
						ReservedConcurrency: tea.Int64(1),
					},
				})
				if err != nil {
					logger.Panic().Err(err).Msg("put concurrency config failed")
				}
			} else {
				logger.Panic().Err(err).Msg("get function failed")
			}
		} else {
			logger.Info().
				Interface("getResp", getResp).
				Msg("function exists")

			// 检查环境变量中的 CODE_HASH 是否一致
			needUpdate := true
			if getResp.Body.EnvironmentVariables != nil {
				if existingCodeHash, ok := getResp.Body.EnvironmentVariables["CODE_HASH"]; ok {
					if *existingCodeHash == codeHash {
						needUpdate = false
					}
				}
			}

			if needUpdate {
				updateResp, err := fc3Client.UpdateFunction(tea.String(hubFunctionName), &fc20230330.UpdateFunctionRequest{
					Body: &fc20230330.UpdateFunctionInput{
						Code: &fc20230330.InputCodeLocation{ZipFile: tea.String(codeBase64)},
						EnvironmentVariables: map[string]*string{
							"CODE_HASH": tea.String(codeHash),
						},
					},
				})
				if err != nil {
					logger.Panic().Err(err).Msg("update function failed")
				}
				logger.Info().
					Interface("updateResp", updateResp).
					Msg("update function")
			} else {
				logger.Info().Msg("code hash matches, skipping update")
			}
		}
	}

	// 生成连接ID
	connId := goutils.UUID4()

	// 调用hub的AcquireConnection RPC
	hubUrl := "https://sshole-hub-eflksbzknn.cn-hangzhou.fcapp.run"
	hubClient := rpcv1connect.NewHoleServiceClient(http.DefaultClient, hubUrl)

	acquireReq := connect.NewRequest(&rpcv1.AcquireConnectionRequest{
		Id: connId,
	})

	acquireResp, err := hubClient.AcquireConnection(context.Background(), acquireReq)
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to acquire connection")
	}
	logger.Info().Interface("Msg", acquireResp.Msg).Send()

	// 将SSH私钥写入临时文件
	tmpFile, err := os.CreateTemp("", "sshole_private_key_*.pem")
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to create temporary file")
	}
	defer tmpFile.Close()

	_, err = tmpFile.WriteString(acquireResp.Msg.SshPrivateKey)
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to write private key to temporary file")
	}

	// 设置权限
	err = os.Chmod(tmpFile.Name(), 0600)
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to chmod temporary file")
	}

	fmt.Printf("SSH Private Key Path: %s\n", tmpFile.Name())

	go func() {
		fcClient, err := clients.GetFcClient(ctx, clients.GetFcClientParams{
			Region:          cli.Fc.Region,
			AccessKeyId:     cli.Fc.AccessKeyId,
			AccessKeySecret: cli.Fc.AccessKeySecret,
			AccountID:       cli.Fc.AccountID,
		})
		if err != nil {
			logger.Panic().Err(err).Msg("Failed to get fc client")
		}

		input := &fc.InstanceExecInput{
			ServiceName:  tea.String(cli.Fc.ServiceName),
			FunctionName: tea.String(cli.Fc.FunctionName),
			InstanceID:   tea.String(cli.Fc.InstanceID),
			// Command:      []string{"curl", "-o", "/sshole", "https://webdav.cloud.117503445.top/public-writable/sshole"},
			Command:     []string{"bash", "-c", fmt.Sprintf("[ -f /sshole ] || curl -o /sshole https://sshole-hub-eflksbzknn.cn-hangzhou.fcapp.run/bin && chmod +x /sshole && HUB_SERVER=https://sshole-hub-eflksbzknn.cn-hangzhou.fcapp.run CONN_ID=%v /sshole agent", connId)},
			Stdin:       false,
			Stdout:      true,
			Stderr:      true,
			TTY:         false,
			IdleTimeout: tea.Int(86400),
		}
		input.OnStdout(func(data []byte) {
			fmt.Printf("STDOUT: %s\n", data)
		})
		input.OnStderr(func(data []byte) {
			fmt.Printf("STDERR: %s\n", data)
		})

		logger.Info().Msg("exec")
		_, err = fcClient.InstanceExec(input)
		if err != nil {
			logger.Panic().Err(err).Msg("Failed to exec")
		}
	}()

	time.Sleep(time.Second * 10)

	fmt.Println("ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null root@localhost -p 24")

	c, err := chclient.NewClient(&chclient.Config{
		Server:  "https://sshole-hub-eflksbzknn.cn-hangzhou.fcapp.run",
		Remotes: []string{"24:localhost:23"}, // 把服务器的 23 端口映射到本地的 24 端口
	})
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to create chisel client")
	}
	if err := c.Start(ctx); err != nil {
		logger.Panic().Err(err).Msg("Failed to start chisel client")
	}
	if err := c.Wait(); err != nil {
		logger.Panic().Err(err).Msg("Failed to wait chisel client")
	}
}
