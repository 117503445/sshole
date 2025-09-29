package common

// func GetEnvAuth(ctx context.Context) string {
// 	logger := log.Ctx(ctx)
// 	auth := os.Getenv("AUTH")
// 	logger.Info().Str("AUTH", auth).Send()
// 	return auth
// }

// func GetEnvHubUrl(ctx context.Context) string {
// 	logger := log.Ctx(ctx)

// 	hubUrl := os.Getenv("HUB_SERVER")
// 	if hubUrl == "" {
// 		logger.Panic().Msg("HUB_SERVER not found in environment variables")
// 	}
// 	return hubUrl
// }
