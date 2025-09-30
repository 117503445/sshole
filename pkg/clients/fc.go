package clients

import (
	"context"
	"fmt"
	"net/http"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	fc20230330 "github.com/alibabacloud-go/fc-20230330/v4/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/aliyun/credentials-go/credentials"
	fc "github.com/117503445/fc-go-sdk"
	"github.com/rs/zerolog/log"
)

type GetFc3ClientParams struct {
	Region    string
	AccountID string

	AccessKeyId     string
	AccessKeySecret string
	SecurityToken   string
}

func GetFc3Client(ctx context.Context, params GetFc3ClientParams) (*fc20230330.Client, error) {
	logger := log.Ctx(ctx)

	logger.Info().Interface("params", params).Msg("GetFc3Client")

	var credential credentials.Credential

	if params.SecurityToken == "" {
		var err error
		credential, err = credentials.NewCredential(&credentials.Config{
			AccessKeyId:     tea.String(params.AccessKeyId),
			AccessKeySecret: tea.String(params.AccessKeySecret),
			Type:            tea.String("access_key"),
		})
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		credential, err = credentials.NewCredential(&credentials.Config{
			AccessKeyId:     tea.String(params.AccessKeyId),
			AccessKeySecret: tea.String(params.AccessKeySecret),
			SecurityToken:   tea.String(params.SecurityToken),
			Type:            tea.String("sts"),
		})
		if err != nil {
			return nil, err
		}
	}

	config := &openapi.Config{
		Credential: credential,
	}
	endpoint := fmt.Sprintf("%s.%s.fc.aliyuncs.com", params.AccountID, params.Region)
	logger.Info().
		Interface("endpoint", endpoint).
		Send()

	config.Endpoint = tea.String(endpoint)
	fcClient, err := fc20230330.NewClient(config)
	if err != nil {
		return nil, err
	}

	return fcClient, nil
}

type GetFcClientParams struct {
	Region    string
	AccountID string

	AccessKeyId     string
	AccessKeySecret string
	SecurityToken   string
}

func GetFcClient(ctx context.Context, params GetFcClientParams) (*fc.Client, error) {
	logger := log.Ctx(ctx)

	logger.Info().Interface("params", params).Msg("GetFcClient")

	endpoint := fmt.Sprintf("%s.%s.fc.aliyuncs.com", params.AccountID, params.Region)

	client, err := fc.NewClient(endpoint, "2016-08-15", params.AccessKeyId, params.AccessKeySecret, fc.WithTransport(&http.Transport{MaxIdleConnsPerHost: 100}))
	// client, err := fc.NewClient(endpoint, "2021-04-06", params.AccessKeyId, params.AccessKeySecret, fc.WithTransport(&http.Transport{MaxIdleConnsPerHost: 100}))
	if err != nil {
		// logger.Panic().Err(err).Msg("Failed to create fc client")
		return nil, err
	}
	return client, nil
}
