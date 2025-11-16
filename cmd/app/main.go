package main

import (
	"context"
	"database/sql"
	"errors"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"go.uber.org/zap"

	"prservice/internal/app/config"
	httpapi "prservice/internal/app/http"
	"prservice/internal/app/http/handler"
	"prservice/internal/domain/pr"
	"prservice/internal/domain/stats"
	"prservice/internal/domain/team"
	"prservice/internal/domain/user"
	"prservice/internal/infrastructure/async"
	"prservice/internal/infrastructure/db/pg"
	"prservice/internal/infrastructure/logging"
)

type randSource struct {
	mu sync.Mutex
	r  *rand.Rand
}

func (rs *randSource) Shuffle(n int, swap func(i, j int)) {
	rs.mu.Lock()
	rs.r.Shuffle(n, swap)
	rs.mu.Unlock()
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	log, err := logging.NewLogger()
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatal("db open error", zap.Error(err))
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		log.Fatal("db ping error", zap.Error(err))
	}

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatal("goose dialect error", zap.Error(err))
	}
	if err := goose.Up(db, "migrations"); err != nil {
		log.Fatal("goose up error", zap.Error(err))
	}

	uow := pg.NewTxManager(db)

	eventBus := async.NewAsyncEventBus(ctx, 4, log)
	defer eventBus.Close()

	rnd := &randSource{r: rand.New(rand.NewSource(time.Now().UnixNano()))}

	teamRepo := pg.NewTeamRepository(db)
	userRepo := pg.NewUserRepository(db)
	prRepo := pg.NewPRRepository(db)
	statsRepo := pg.NewStatsRepository(db)

	teamSvc := team.NewService(uow, teamRepo, userRepo, eventBus)
	userSvc := user.NewService(uow, userRepo, eventBus)
	prSvc := pr.NewService(uow, prRepo, userRepo, eventBus, rnd)
	statsSvc := stats.NewService(statsRepo)

	h := handler.New(teamSvc, userSvc, prSvc, statsSvc, log)
	router := httpapi.NewRouter(h, log)

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("server starting", zap.String("addr", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Info("shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", zap.Error(err))
	}
}
