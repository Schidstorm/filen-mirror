package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/FilenCloudDienste/filen-sdk-go/filen"
	filenextra "github.com/Schidstorm/edge_config/apps/filen-mirror/pkg/filen_extra"
	"github.com/Schidstorm/edge_config/apps/filen-mirror/pkg/mirror"
	"github.com/Schidstorm/edge_config/apps/filen-mirror/pkg/totp"
	"github.com/rs/zerolog"
)

var version = "dev"
var userAgent = fmt.Sprintf("filen-mirror/%s", version)

var log = zerolog.New(os.Stderr).With().Timestamp().Logger().Level(zerolog.DebugLevel)

const timeout = time.Second * 30

var config *configStruct

type configStruct struct {
	filenEmail    string
	filenPassword string
	totpSecret    string
	totpDigits    int
	totpPeriod    int64
	syncDir       string
	socketURL     string
}

func getConfig() *configStruct {
	if config != nil {
		return config
	}

	totpDigits, err := strconv.Atoi(os.Getenv("TOTP_DIGITS"))
	if err != nil {
		log.Fatal().Err(err).Msg("Invalid TOTP_DIGITS")
	}
	totpPeriod, err := strconv.ParseInt(os.Getenv("TOTP_PERIOD"), 10, 64)
	if err != nil {
		log.Fatal().Err(err).Msg("Invalid TOTP_PERIOD")
	}

	config = &configStruct{
		filenEmail:    os.Getenv("FILEN_EMAIL"),
		filenPassword: os.Getenv("FILEN_PASSWORD"),
		totpSecret:    os.Getenv("TOTP_SECRET"),
		totpDigits:    totpDigits,
		totpPeriod:    totpPeriod,
		syncDir:       getenvDefault("FILEN_SYNC_DIR", "./data"),
		socketURL:     getenvDefault("FILEN_SOCKET_URL", "wss://socket.filen.io:443"),
	}
	return config
}

func main() {
	zerolog.DefaultContextLogger = &log

	dotenv, err := os.ReadFile(".env")
	if err == nil {
		lines := strings.SplitSeq(string(dotenv), "\n")
		for line := range lines {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				os.Setenv(parts[0], parts[1])
			}
		}
	}

	totp := setupTotp()
	client, err := setupFilenClient(totp)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to set up Filen client")
	}

	events, err := filenextra.NewFilenEvents(getConfig().socketURL, client, http.Header{
		"User-Agent": []string{userAgent},
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Filen events")
	}

	mirror := mirror.NewFilenMirror(client, events, mirror.FilenMirrorConfig{
		SyncDir: getConfig().syncDir,
	})
	mirror.Start()

	select {}
}

func setupFilenClient(totp *totp.TOTPGenerator) (*filen.Filen, error) {
	otp, err := totp.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate TOTP: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client, err := filen.New(ctx, getConfig().filenEmail, getConfig().filenPassword, otp)
	if err != nil {
		return nil, fmt.Errorf("failed to create Filen client: %w", err)
	}

	return client, nil
}

func setupTotp() *totp.TOTPGenerator {
	return &totp.TOTPGenerator{
		Secret: getConfig().totpSecret,
		Digits: getConfig().totpDigits,
		Period: getConfig().totpPeriod,
	}
}

func getenvDefault(name, def string) string {
	val := os.Getenv(name)
	if val == "" {
		return def
	}

	return val
}
