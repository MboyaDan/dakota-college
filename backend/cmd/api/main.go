// Command api runs the Dakota Training College HTTP API.
//
// Usage:
//
//	api              start the server
//	api healthcheck  exit 0 if the local server answers /api/v1/health
//	                 (used by the container HEALTHCHECK; the image has no curl)
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/MboyaDan/dakota-college/backend/internal/academics"
	"github.com/MboyaDan/dakota-college/backend/internal/access"
	"github.com/MboyaDan/dakota-college/backend/internal/admissions"
	"github.com/MboyaDan/dakota-college/backend/internal/applicant"
	"github.com/MboyaDan/dakota-college/backend/internal/application"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/config"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db"
	"github.com/MboyaDan/dakota-college/backend/internal/enquiry"
	"github.com/MboyaDan/dakota-college/backend/internal/finance"
	"github.com/MboyaDan/dakota-college/backend/internal/httpapi"
	"github.com/MboyaDan/dakota-college/backend/internal/kcb"
	"github.com/MboyaDan/dakota-college/backend/internal/mail"
	"github.com/MboyaDan/dakota-college/backend/internal/media"
	"github.com/MboyaDan/dakota-college/backend/internal/mpesa"
	"github.com/MboyaDan/dakota-college/backend/internal/notify"
	"github.com/MboyaDan/dakota-college/backend/internal/outbox"
	"github.com/MboyaDan/dakota-college/backend/internal/portal"
	"github.com/MboyaDan/dakota-college/backend/internal/privacy"
	"github.com/MboyaDan/dakota-college/backend/internal/reports"
	"github.com/MboyaDan/dakota-college/backend/internal/results"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "api:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)
	if cfg.Env == config.EnvDevelopment {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	mailer := mail.SMTP{Host: cfg.SMTP.Host, Port: cfg.SMTP.Port, Username: cfg.SMTP.Username, Password: cfg.SMTP.Password, From: cfg.SMTP.From}
	if cfg.MessageKeyIsDevelopment {
		logger.Warn("MESSAGE_KEY not set: queued messages use the development key")
	}
	if cfg.TOTPKeyIsDevelopment {
		logger.Warn("TOTP_KEY not set: two-step sign-in secrets use the development key")
	}
	// Every email goes through the queue; text messages are recorded as
	// skipped until an SMS provider is chosen (DG-03).
	queue, err := outbox.New(outbox.Options{Pool: pool, Mailer: mailer, Key: cfg.MessageKey, Logger: logger})
	if err != nil {
		return fmt.Errorf("message queue: %w", err)
	}
	queueDone := make(chan struct{})
	go func() {
		defer close(queueDone)
		queue.Run(ctx)
	}()

	authSvc, err := auth.NewService(auth.Options{
		Pool:       pool,
		Outbox:     queue,
		Logger:     logger,
		Session:    cfg.Session,
		AppBaseURL: cfg.AppBaseURL,
		TOTPKey:    cfg.TOTPKey,
	})
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	var images *media.Images
	var store media.Storage
	if cfg.S3.Enabled() {
		s3, err := media.NewS3(media.S3Config{Endpoint: cfg.S3.Endpoint, Region: cfg.S3.Region, AccessKey: cfg.S3.AccessKey, SecretKey: cfg.S3.SecretKey, UseSSL: cfg.S3.UseSSL})
		if err != nil {
			return err
		}
		// Local buckets are created automatically; hosted ones by hand.
		if cfg.Env == config.EnvDevelopment || cfg.Env == config.EnvTest {
			if err := s3.EnsureBuckets(ctx, cfg.S3.PublicBucket, cfg.S3.PrivateBucket); err != nil {
				logger.Warn("could not prepare storage buckets", slog.String("error", err.Error()))
			}
		}
		store = s3
		images = &media.Images{Pool: pool, Storage: s3, Bucket: cfg.S3.PublicBucket, PublicBaseURL: cfg.S3.PublicBaseURL}
	} else {
		logger.Warn("object storage not configured: image uploads are disabled")
	}

	var scanner media.Scanner = media.NoScanner{}
	if cfg.ClamAVAddr != "" {
		scanner = media.ClamAV{Addr: cfg.ClamAVAddr}
	} else {
		logger.Warn("no virus scanner configured: applicant documents are stored as not scanned")
	}

	notifier := notify.NewService(pool, queue, logger)
	// M-Pesa C2B is on only when MPESA_ENABLED (DG-02); every value comes from the environment.
	mpesaSvc := mpesa.NewService(mpesa.Options{Pool: pool, Logger: logger, Config: cfg.Mpesa})
	paybill := ""
	if mpesaSvc != nil {
		paybill = mpesaSvc.ShortCode()
		logger.Info("M-Pesa C2B enabled", slog.String("environment", cfg.Mpesa.Environment), slog.String("shortcode", cfg.Mpesa.ShortCode))
	}
	academicsSvc := academics.NewService(pool, nil)
	admissionsSvc := admissions.NewService(admissions.Options{
		Pool: pool, Notify: notifier, Logger: logger, AppBaseURL: cfg.AppBaseURL,
		Storage: store, PrivateBucket: cfg.S3.PrivateBucket,
	})
	// The retention schedule runs daily in RETENTION_MODE (Stage 17c).
	privacySvc := privacy.NewService(privacy.Options{Pool: pool, Logger: logger, Storage: store, Mode: cfg.RetentionMode})
	if cfg.RetentionMode != privacy.ModeApply {
		logger.Info("retention schedule in " + cfg.RetentionMode + " mode: personal data past its period is counted, not erased")
	}
	go privacySvc.RunDaily(ctx)

	router, err := httpapi.NewRouter(httpapi.Deps{
		Logger:         logger,
		DB:             pool,
		TrustedProxies: cfg.TrustedProxies,
		ClientIPSecret: cfg.ClientIPSecret,
		AllowedOrigins: cfg.AllowedOrigins,
		CookieSecure:   cfg.Session.CookieSecure,
		Auth:           authSvc,
		Access:         access.NewService(pool),
		Content:        content.NewService(pool),
		Images:         images,
		Enquiries:      enquiry.NewService(pool, queue, logger, cfg.AppBaseURL),
		Notify:         notifier,
		Applicants: applicant.NewService(applicant.Options{
			Pool: pool, Auth: authSvc, Notify: notifier, Logger: logger, AppBaseURL: cfg.AppBaseURL,
		}),
		Applications: application.NewService(application.Options{
			Pool: pool, Notify: notifier, Logger: logger, AppBaseURL: cfg.AppBaseURL,
			Storage: store, PrivateBucket: cfg.S3.PrivateBucket, Scanner: scanner,
		}),
		Academics: academicsSvc,
		Results: results.NewService(results.Options{
			Pool: pool, Logger: logger, Storage: store, PrivateBucket: cfg.S3.PrivateBucket,
		}),
		Finance: finance.NewService(finance.Options{Pool: pool, Logger: logger, MpesaPaybill: paybill}),
		Mpesa:   mpesaSvc,
		Kcb:     kcb.NewService(kcb.Options{Pool: pool, Logger: logger, Config: cfg.KcbIPN}),
		Portal: portal.NewService(portal.Options{
			Pool: pool, Academics: academicsSvc, Notify: notifier, Logger: logger, AppBaseURL: cfg.AppBaseURL,
		}),
		Admissions: admissionsSvc,
		Reports:    reports.NewService(reports.Options{Pool: pool, Admissions: admissionsSvc}),
		Privacy:    privacySvc,
	})
	if err != nil {
		return fmt.Errorf("router: %w", err)
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	errc := make(chan error, 1)
	go func() {
		logger.Info("api listening", slog.String("addr", cfg.HTTPAddr), slog.String("env", string(cfg.Env)))
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("listen: %w", err)
		}
	case <-ctx.Done():
		logger.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	// The worker stops with ctx; a message it was sending is recorded first.
	stop()
	select {
	case <-queueDone:
	case <-shutdownCtx.Done():
	}
	logger.Info("stopped")
	return nil
}

func healthcheck() int {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 1
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + net.JoinHostPort(host, port) + "/api/v1/health")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
