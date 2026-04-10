package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "temp/gen/pb/v1"
	grpcadapter "temp/internal/adapter/grpc"
	"temp/internal/adapter/postgres"
	"temp/internal/infrastructure"
	"temp/internal/usecase"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg := infrastructure.Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     5432,
		User:     getEnv("DB_USER", "iam"),
		Password: getEnv("DB_PASSWORD", "iam"),
		DBName:   getEnv("DB_NAME", "iam"),
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := infrastructure.NewPostgresPool(ctx, cfg)
	if err != nil {
		log.Error("connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	log.Info("connected to postgres")

	// ─── repositories ───
	aoRepo := postgres.NewAccessObjectRepo(pool)
	resRepo := postgres.NewResourceRepo(pool)
	roleRepo := postgres.NewRoleRepo(pool)

	// ─── use cases ───
	aoUC := usecase.NewAccessObjectUseCase(aoRepo)
	resUC := usecase.NewResourceUseCase(resRepo, aoRepo)
	roleUC := usecase.NewRoleUseCase(roleRepo, resRepo, aoRepo)

	// ─── gRPC server ───
	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(loggingInterceptor(log)),
	)

	pb.RegisterAccessObjectServiceServer(grpcSrv, grpcadapter.NewAccessObjectHandler(aoUC))
	pb.RegisterResourceServiceServer(grpcSrv, grpcadapter.NewResourceHandler(resUC))
	pb.RegisterRoleServiceServer(grpcSrv, grpcadapter.NewRoleHandler(roleUC))
	reflection.Register(grpcSrv)

	addr := getEnv("GRPC_ADDR", ":50051")
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error("listen", "error", err)
		os.Exit(1)
	}

	log.Info("starting gRPC server", "addr", addr)

	go func() {
		<-ctx.Done()
		log.Info("shutting down gRPC server")
		grpcSrv.GracefulStop()
	}()

	if err := grpcSrv.Serve(lis); err != nil {
		log.Error("serve", "error", err)
		os.Exit(1)
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func loggingInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			log.Error("rpc error", "method", info.FullMethod, "error", err)
		} else {
			log.Info("rpc", "method", info.FullMethod)
		}
		return resp, err
	}
}
